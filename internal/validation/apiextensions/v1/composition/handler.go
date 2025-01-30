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

// Package composition contains internal logic linked to the validation of the v1.Composition type.
package composition

import (
	"context"
	"fmt"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/pkg/validation/apiextensions/v1/composition"
)

const (
	// Key used to index CRDs by "Kind" and "group", to be used when
	// indexing and retrieving needed CRDs.
	crdsIndexKey = "crd.kind.group"
)

// Error strings.
const (
	errNotComposition = "supplied object was not a Composition"
	errValidationMode = "cannot get validation mode"

	errFmtTooManyCRDs = "more than one CRD found for %s.%s: %v"
	errFmtGetCRDs     = "cannot get the needed CRDs: %v"
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, options controller.Options) error {
	if options.Features.Enabled(features.EnableBetaCompositionWebhookSchemaValidation) {
		// Setup an index on CRDs so we can retrieve them by group and kind.
		// The index is used by the getCRD function below.
		indexer := mgr.GetFieldIndexer()
		if err := indexer.IndexField(context.Background(), &extv1.CustomResourceDefinition{}, crdsIndexKey, func(obj client.Object) []string {
			return []string{getIndexValueForCRD(obj.(*extv1.CustomResourceDefinition))} //nolint:forcetypeassert // Will always be a CRD.
		}); err != nil {
			return err
		}
	}

	v := &validator{reader: mgr.GetClient(), options: options}
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(v).
		For(&v1.Composition{}).
		Complete()
}

type validator struct {
	reader  client.Reader
	options controller.Options
}

// ValidateCreate validates a Composition.
func (v *validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return nil, errors.New(errNotComposition)
	}

	// Validate the composition itself, we'll disable it on the Validator below.
	warns, validationErrs := comp.Validate()
	if len(validationErrs) != 0 {
		return warns, kerrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), validationErrs)
	}

	if !v.options.Features.Enabled(features.EnableBetaCompositionWebhookSchemaValidation) {
		return warns, nil
	}

	// Get the composition validation mode from annotation
	validationMode, err := comp.GetSchemaAwareValidationMode()
	if err != nil {
		return warns, errors.Wrap(err, errValidationMode)
	}

	// Get all the needed CRDs, Composite Resource, Managed resources ... ?
	// Error out if missing in strict mode
	gkToCRD, errs := v.getNeededCRDs(ctx, comp)
	// If we have errors, and we are in strict mode or any of the errors is not
	// a NotFound, return them.
	if len(errs) != 0 {
		if validationMode == v1.SchemaAwareCompositionValidationModeStrict || containsOtherThanNotFound(errs) {
			return warns, errors.Errorf(errFmtGetCRDs, errs)
		}
		// If we have errors, but we are not in strict mode, and all of the
		// errors are not found errors, just move them to warnings and skip any
		// further validation.

		// TODO(phisco): we are playing it safe and skipping validation
		// altogether, in the future we might want to also support partially
		// available inputs.
		for _, err := range errs {
			warns = append(warns, err.Error())
		}
		return warns, nil
	}

	cv, err := composition.NewValidator(
		composition.WithCRDGetterFromMap(gkToCRD),
		// We disable logical Validation as this has already been done above.
		composition.WithoutLogicalValidation(),
	)
	if err != nil {
		return warns, kerrors.NewInternalError(err)
	}
	schemaWarns, errList := cv.Validate(ctx, comp)
	warns = append(warns, schemaWarns...)
	if len(errList) != 0 {
		if validationMode != v1.SchemaAwareCompositionValidationModeWarn {
			return warns, kerrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), errList)
		}
		for _, err := range errList {
			warns = append(warns, fmt.Sprintf("Composition %q invalid for schema-aware validation: %s", comp.GetName(), err))
		}
	}
	return warns, nil
}

// ValidateUpdate implements the same logic as ValidateCreate.
func (v *validator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

// ValidateDelete always allows delete requests.
func (v *validator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// containsOtherThanNotFound returns true if the given slice of errors contains
// any error other than a not found error.
func containsOtherThanNotFound(errs []error) bool {
	for _, err := range errs {
		if !kerrors.IsNotFound(err) {
			return true
		}
	}
	return false
}

func (v *validator) getNeededCRDs(ctx context.Context, comp *v1.Composition) (map[schema.GroupKind]apiextensions.CustomResourceDefinition, []error) {
	// TODO(negz): Use https://pkg.go.dev/errors#Join to return a single error?
	var resultErrs []error
	neededCrds := make(map[schema.GroupKind]apiextensions.CustomResourceDefinition)

	// Get schema for the Composite Resource Definition defined by
	// comp.Spec.CompositeTypeRef.
	compositeResGK := schema.FromAPIVersionAndKind(comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind).GroupKind()

	compositeCRD, err := v.getCRD(ctx, &compositeResGK)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return nil, []error{err}
		}
		resultErrs = append(resultErrs, err)
	}
	if compositeCRD != nil {
		neededCrds[compositeResGK] = *compositeCRD
	}

	return neededCrds, resultErrs
}

// getCRD returns the validation schema for the given GVK, by looking up the CRD
// by group and kind using the provided client.
func (v *validator) getCRD(ctx context.Context, gk *schema.GroupKind) (*apiextensions.CustomResourceDefinition, error) {
	crds := extv1.CustomResourceDefinitionList{}
	if err := v.reader.List(ctx, &crds, client.MatchingFields{crdsIndexKey: getIndexValueForGroupKind(gk)}); err != nil {
		return nil, err
	}
	switch {
	case len(crds.Items) == 0:
		return nil, kerrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "CustomResourceDefinition"}, fmt.Sprintf("%s.%s", gk.Kind, gk.Group))
	case len(crds.Items) > 1:
		names := []string{}
		for _, crd := range crds.Items {
			names = append(names, crd.Name)
		}
		return nil, kerrors.NewInternalError(errors.Errorf(errFmtTooManyCRDs, gk.Kind, gk.Group, names))
	}
	crd := crds.Items[0]
	internal := &apiextensions.CustomResourceDefinition{}
	return internal, extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&crd, internal, nil)
}

// getIndexValueForCRD returns the index value for the given CRD, according to
// the resource defined in the spec.
func getIndexValueForCRD(crd *extv1.CustomResourceDefinition) string {
	return getIndexValueForGroupKind(&schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind})
}

// getIndexValueForGroupKind returns the index value for the given GroupKind.
func getIndexValueForGroupKind(gk *schema.GroupKind) string {
	return gk.String()
}
