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

	v12 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

// Error strings.
const (
	errNotCompositeResourceDefinition = "supplied object was not a CompositeResourceDefinition"

	errUnexpectedType = "unexpected type"

	errGroupImmutable                  = "spec.group is immutable"
	errPluralImmutable                 = "spec.names.plural is immutable"
	errKindImmutable                   = "spec.names.kind is immutable"
	errClaimPluralImmutable            = "spec.claimNames.plural is immutable"
	errClaimKindImmutable              = "spec.claimNames.kind is immutable"
	errConversionWebhookConfigRequired = "spec.conversion.webhook is required when spec.conversion.strategy is 'Webhook'"
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

func getAllCRDsForXRD(in *v1.CompositeResourceDefinition) (out []*v12.CustomResourceDefinition, err error) {
	crd, err := xcrd.ForCompositeResource(in)
	if err != nil {
		return out, errors.Wrap(err, "cannot get CRD for Composite Resource")
	}
	out = append(out, crd)
	// if claim enabled, validate claim CRD
	if in.Spec.ClaimNames == nil {
		return out, nil
	}
	crdClaim, err := xcrd.ForCompositeResourceClaim(in)
	if err != nil {
		return out, errors.Wrap(err, "cannot get Claim CRD for Composite Claim")
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
		return warns, errors.Wrap(err, "cannot get CRDs for CompositeResourceDefinition")
	}
	for _, crd := range crds {
		// Can't use validation.ValidateCustomResourceDefinition because it leads to dependency errors,
		// see https://github.com/kubernetes/apiextensions-apiserver/issues/59
		// if errs := validation.ValidateCustomResourceDefinition(ctx, crd); len(errs) != 0 {
		//	return warns, errors.Wrap(errs.ToAggregate(), "invalid CRD generated for CompositeResourceDefinition")
		//}
		got := crd.DeepCopy()
		err := v.client.Get(ctx, client.ObjectKey{Name: crd.Name}, got)
		switch {
		case err == nil:
			got.Spec = crd.Spec
			if err := v.client.Update(ctx, got, client.DryRunAll); err != nil {
				return warns, errors.Wrap(err, "cannot dry run update CRD for CompositeResourceDefinition")
			}
		case apierrors.IsNotFound(err):
			if err := v.client.Create(ctx, crd, client.DryRunAll); err != nil {
				return warns, errors.Wrap(err, "cannot dry run create CRD for CompositeResourceDefinition")
			}
		default:
			return warns, errors.Wrap(err, "cannot dry run get CRD for CompositeResourceDefinition")
		}
	}

	return warns, nil
}

// ValidateUpdate implements the same logic as ValidateCreate.
func (v *validator) ValidateUpdate(ctx context.Context, old, new runtime.Object) (admission.Warnings, error) {
	oldObj, ok := old.(*v1.CompositeResourceDefinition)
	if !ok {
		return nil, errors.New(errUnexpectedType)
	}
	newObj, ok := new.(*v1.CompositeResourceDefinition)
	if !ok {
		return nil, errors.New(errUnexpectedType)
	}
	switch {
	case newObj.Spec.Group != oldObj.Spec.Group:
		return nil, errors.New(errGroupImmutable)
	case newObj.Spec.Names.Plural != oldObj.Spec.Names.Plural:
		return nil, errors.New(errPluralImmutable)
	case newObj.Spec.Names.Kind != oldObj.Spec.Names.Kind:
		return nil, errors.New(errKindImmutable)
	}
	if newObj.Spec.ClaimNames != nil && oldObj.Spec.ClaimNames != nil {
		switch {
		case newObj.Spec.ClaimNames.Plural != oldObj.Spec.ClaimNames.Plural:
			return nil, errors.New(errClaimPluralImmutable)
		case newObj.Spec.ClaimNames.Kind != oldObj.Spec.ClaimNames.Kind:
			return nil, errors.New(errClaimKindImmutable)
		}
	}
	return v.ValidateCreate(ctx, newObj)
}

// ValidateDelete always allows delete requests.
func (v *validator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
