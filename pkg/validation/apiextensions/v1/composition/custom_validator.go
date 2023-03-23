/*
Copyright 2023 the Crossplane Authors.

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

package composition

import (
	"context"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// CustomValidator gathers required information using the provided client.Reader and then use them to render and
// validated a Composition.
type CustomValidator struct {
	reader ReaderWithScheme
}

// ReaderWithScheme is a client.Reader that also returns the scheme it uses.
// Unfortunately the client.Reader interface does not have a Scheme() method, only the client.Client interface does.
type ReaderWithScheme interface {
	client.Reader
	Scheme() *runtime.Scheme
}

// SetupWithManager sets up the CustomValidator with the provided manager, setting up all the required indexes it requires.
func (c *CustomValidator) SetupWithManager(mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()
	if err := indexer.IndexField(context.Background(), &extv1.CustomResourceDefinition{}, "spec.group", func(obj client.Object) []string {
		return []string{obj.(*extv1.CustomResourceDefinition).Spec.Group}
	}); err != nil {
		return err
	}
	if err := indexer.IndexField(context.Background(), &extv1.CustomResourceDefinition{}, "spec.names.kind", func(obj client.Object) []string {
		return []string{obj.(*extv1.CustomResourceDefinition).Spec.Names.Kind}
	}); err != nil {
		return err
	}

	c.reader = unstructured.NewClient(mgr.GetClient())

	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(c).
		For(&v1.Composition{}).
		Complete()
}

// ValidateUpdate is a no-op for now.
func (c *CustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	// TODO(phisco): check if there is anything to validate in the diff
	return c.ValidateCreate(ctx, newObj)
}

// ValidateDelete is a no-op for now.
func (c *CustomValidator) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate validates the Composition by rendering it and then validating the rendered resources.
//
//nolint:gocyclo // TODO (phisco): refactor this function
func (c *CustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return xperrors.New("not a v1 Composition")
	}

	// Validate the composition itself, we'll disable it on the Validator below
	if errs := comp.Validate(); len(errs) != 0 {
		return apierrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), errs)
	}

	// Get the composition validation mode from annotation
	validationMode, err := comp.GetValidationMode()
	if err != nil {
		return xperrors.Wrap(err, "cannot get validation mode")
	}

	// Get all the needed CRDs, Composite Resource, Managed resources ... ? Error out if missing in strict mode
	gvkToCRDs, errs := c.getNeededCRDs(ctx, comp)
	var shouldSkip bool
	for _, err := range errs {
		if err == nil {
			continue
		}
		// If any of the errors is not a NotFound error, error out
		if !apierrors.IsNotFound(err) {
			return xperrors.Errorf("there were some errors while getting the needed CRDs: %v", errs)
		}
		// If any of the needed CRDs is not found, error out if strict mode is enabled, otherwise continue
		if validationMode == v1.CompositionValidationModeStrict {
			return xperrors.Wrap(err, "cannot get needed CRDs and strict mode is enabled")
		}
		if validationMode == v1.CompositionValidationModeLoose {
			shouldSkip = true
		}
	}

	// Given that some requirement is missing, and we are in loose mode, skip validation
	if shouldSkip {
		// TODO(phisco): emit a warning here
		return nil
	}

	// from here on we should refactor the code to allow using it from linters/Lsp
	// TODO (lsviben) figure out how to emit warnings instead of errors in case of WARN state (strict, but just warnings)
	v, err := NewValidator(
		WithCRDGetterFromMap(gvkToCRDs),
		// We disable logical Validation as this has already been done above
		WithoutLogicalValidation(),
	)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	if errs := v.Validate(ctx, comp); len(errs) != 0 {
		return apierrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), errs)
	}
	return nil
}

func (c *CustomValidator) getNeededCRDs(ctx context.Context, comp *v1.Composition) (map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition, []error) {
	var resultErrs []error
	neededCrds := make(map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition)

	// Get schema for the Composite Resource Definition defined by comp.Spec.CompositeTypeRef
	compositeResGVK := schema.FromAPIVersionAndKind(comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind)

	compositeCRD, err := c.getCRDForGVK(ctx, &compositeResGVK)
	switch {
	case apierrors.IsNotFound(err):
		resultErrs = append(resultErrs, err)
	case err != nil:
		return nil, []error{err}
	case compositeCRD != nil:
		neededCrds[compositeResGVK] = *compositeCRD
	}

	// Get schema for all Managed Resource Definitions defined by comp.Spec.Resources
	for _, res := range comp.Spec.Resources {
		cd, err := res.GetBaseObject()
		if err != nil {
			return nil, []error{err}
		}
		gvk := cd.GetObjectKind().GroupVersionKind()
		if _, ok := neededCrds[gvk]; ok {
			continue
		}
		crd, err := c.getCRDForGVK(ctx, &gvk)
		switch {
		case apierrors.IsNotFound(err):
			resultErrs = append(resultErrs, err)
		case err != nil:
			return nil, []error{err}
		case compositeCRD != nil:
			neededCrds[gvk] = *crd
		}
	}

	return neededCrds, resultErrs
}

// getCRDForGVK returns the validation schema for the given GVK, by looking up the CRD by group and kind using
// the provided client.
func (c *CustomValidator) getCRDForGVK(ctx context.Context, gvk *schema.GroupVersionKind) (*apiextensions.CustomResourceDefinition, error) {
	crds := extv1.CustomResourceDefinitionList{}
	if err := c.reader.List(ctx, &crds, client.MatchingFields{"spec.group": gvk.Group},
		client.MatchingFields{"spec.names.kind": gvk.Kind}); err != nil {
		return nil, err
	}
	if len(crds.Items) != 1 {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "CustomResourceDefinition"}, fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group))
	}
	crd := crds.Items[0]
	internal := &apiextensions.CustomResourceDefinition{}
	return internal, extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&crd, internal, nil)
}
