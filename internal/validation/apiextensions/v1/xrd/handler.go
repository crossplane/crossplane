/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package xrd contains internal logic linked to the validation of the v1.CompositeResourceDefinition type.
package xrd

import (
	"context"
	"errors"
	"fmt"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errNotCompositeResourceDefinition = "supplied object was not a CompositeResourceDefinition"

	errUnexpectedType = "unexpected type"
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, _ controller.Options) error {
	v := &validator{client: mgr.GetClient()}
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(v).
		For(&v1.CompositeResourceDefinition{}).
		Complete()
}

type validator struct {
	client client.Client
}

func getAllCRDsForXRD(in *v1.CompositeResourceDefinition) (out []*apiextv1.CustomResourceDefinition, err error) {
	crd, err := xcrd.ForCompositeResource(in)
	if err != nil {
		return out, xperrors.Wrap(err, "cannot get CRD for Composite Resource")
	}
	out = append(out, crd)
	// if claim enabled, validate claim CRD
	if in.Spec.ClaimNames == nil {
		return out, nil
	}
	crdClaim, err := xcrd.ForCompositeResourceClaim(in)
	if err != nil {
		return out, xperrors.Wrap(err, "cannot get Claim CRD for Composite Claim")
	}
	out = append(out, crdClaim)
	return out, nil
}

// ValidateCreate validates a Composition.
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) (warns admission.Warnings, err error) {
	in, ok := obj.(*v1.CompositeResourceDefinition)
	if !ok {
		return nil, errors.New(errNotCompositeResourceDefinition)
	}
	validationWarns, validationErr := in.Validate()
	warns = append(warns, validationWarns...)
	if validationErr != nil {
		return validationWarns, validationErr.ToAggregate()
	}
	crds, err := getAllCRDsForXRD(in)
	if err != nil {
		return warns, xperrors.Wrap(err, "cannot get CRDs for CompositeResourceDefinition")
	}
	for _, crd := range crds {
		// Can't use validation.ValidateCustomResourceDefinition because it leads to dependency errors,
		// see https://github.com/kubernetes/apiextensions-apiserver/issues/59
		// if errs := validation.ValidateCustomResourceDefinition(ctx, crd); len(errs) != 0 {
		//	return warns, errors.Wrap(errs.ToAggregate(), "invalid CRD generated for CompositeResourceDefinition")
		//}
		if err := v.client.Create(ctx, crd, client.DryRunAll); err != nil {
			return warns, v.rewriteError(err, in, crd)
		}
	}

	return warns, nil
}

// ValidateUpdate implements the same logic as ValidateCreate.
func (v *validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warns admission.Warnings, err error) {
	// Validate the update
	oldXRD, ok := oldObj.(*v1.CompositeResourceDefinition)
	if !ok {
		return nil, errors.New(errUnexpectedType)
	}
	newXRD, ok := newObj.(*v1.CompositeResourceDefinition)
	if !ok {
		return nil, errors.New(errUnexpectedType)
	}
	// Validate the update
	validationWarns, validationErr := newXRD.ValidateUpdate(oldXRD)
	warns = append(warns, validationWarns...)
	if validationErr != nil {
		return validationWarns, validationErr.ToAggregate()
	}
	crds, err := getAllCRDsForXRD(newXRD)
	if err != nil {
		return warns, xperrors.Wrap(err, "cannot get CRDs for CompositeResourceDefinition")
	}
	for _, crd := range crds {
		// Can't use validation.ValidateCustomResourceDefinition because it leads to dependency errors,
		// see https://github.com/kubernetes/apiextensions-apiserver/issues/59
		// if errs := validation.ValidateCustomResourceDefinition(ctx, crd); len(errs) != 0 {
		//	return warns, errors.Wrap(errs.ToAggregate(), "invalid CRD generated for CompositeResourceDefinition")
		//}
		//
		// We need to be able to handle both cases:
		// 1. both CRDs exists already, which should be most of the time
		// 2. Claim's CRD does not exist yet, e.g. the user updated the XRD spec
		// which previously did not specify a claim.
		err := v.dryRunUpdateOrCreateIfNotFound(ctx, crd)
		if err != nil {
			return warns, v.rewriteError(err, newXRD, crd)
		}
	}

	return warns, nil
}

func (v *validator) dryRunUpdateOrCreateIfNotFound(ctx context.Context, crd *apiextv1.CustomResourceDefinition) error {
	got := crd.DeepCopy()
	err := v.client.Get(ctx, client.ObjectKey{Name: crd.Name}, got)
	if err == nil {
		got.Spec = crd.Spec
		return v.client.Update(ctx, got, client.DryRunAll)
	}
	if kerrors.IsNotFound(err) {
		return v.client.Create(ctx, crd, client.DryRunAll)
	}
	return err
}

// ValidateDelete always allows delete requests.
func (v *validator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (v *validator) rewriteError(err error, in *v1.CompositeResourceDefinition, crd *apiextv1.CustomResourceDefinition) error {
	// the handler is just discarding wrapping errors unfortunately, so
	// we need to unwrap it here, modify its content and return that
	// instead
	if err == nil {
		return nil
	}
	var apiErr *kerrors.StatusError
	if errors.As(err, &apiErr) {
		apiErr.ErrStatus.Message = "invalid CRD generated for CompositeResourceDefinition: " + apiErr.ErrStatus.Message
		apiErr.ErrStatus.Details.Kind = v1.CompositeResourceDefinitionKind
		apiErr.ErrStatus.Details.Group = v1.Group
		apiErr.ErrStatus.Details.Name = in.GetName()
		for i, cause := range apiErr.ErrStatus.Details.Causes {
			cause.Field = fmt.Sprintf("<generated_CRD_%q>.%s", crd.GetName(), cause.Field)
			apiErr.ErrStatus.Details.Causes[i] = cause
		}
		return apiErr
	}
	return err
}
