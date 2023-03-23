package composition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8sValidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xprComposite "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/validation"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	compositeResourceValidationName      = "validationName"
	compositeResourceValidationNamespace = "validationNamespace"
)

// WithReconciler returns a ValidatorOption that configures the Validator to use the given Reconciler to render
// the Composition.
func WithReconciler(r *composite.Reconciler) ValidatorOption {
	return func(v *Validator) {
		v.reconciler = r
		v.client = r.GetClient()
	}
}

// RenderAndValidateResources renders and validates all resources of the given Composition.
func (v *Validator) renderAndValidateResources(ctx context.Context, comp *xpv1.Composition) (errs field.ErrorList) {
	// Return if using unsupported/non-deterministic features, e.g. Transforms...
	// TODO(phisco): what about unsupported patch types? should we consider them non-deterministic as we do for Functions?
	// TODO(phisco): we could loose the constraint here, to be discussed
	if !v.shouldRenderResources(comp) {
		return nil
	}

	if v.reconciler == nil {
		// no Composite resource Reconciler was provided, means we are done
		return nil
	}
	// Render and Validate all rendered resources, according to the provided CRDs

	// Mock any required input given their CRDs
	compositeRes, compositeResGVK := genCompositeResource(comp)
	compositeResCRD, err := v.crdGetter.Get(ctx, compositeResGVK)
	if err != nil {
		return append(errs, field.InternalError(
			field.NewPath("spec", "compositeTypeRef"),
			err,
		))
	}
	if err := validation.MockRequiredFields(compositeRes, compositeResCRD.Spec.Validation.OpenAPIV3Schema); err != nil {
		return append(errs, field.InternalError(field.NewPath("spec", "compositeTypeRef"), err))
	}

	// create all required resources
	mockedObjects := []client.Object{compositeRes, sanitizedComposition(comp)}
	for _, obj := range mockedObjects {
		err := v.reconciler.GetClient().Create(ctx, obj)
		if err != nil {
			return append(errs, field.InternalError(field.NewPath("spec"), xperrors.Wrap(err, "cannot create required mock resources")))
		}
	}

	// Render resources
	if _, err := v.reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: compositeResourceValidationName, Namespace: compositeResourceValidationNamespace}}); err != nil {
		return append(errs, field.InternalError(field.NewPath("spec"), xperrors.Wrap(err, "cannot render resources")))
	}

	// Validate resources given their CRDs
	return v.validateRenderedResources(ctx, compositeResGVK, comp)
}

//nolint:gocyclo // TODO(phisco): refactor this function
func (v *Validator) validateRenderedResources(ctx context.Context, compositeResGVK schema.GroupVersionKind, comp *xpv1.Composition) (errs field.ErrorList) {
	var validationWarns []error
	// TODO (lsviben): we are currently validating only things we have schema for, instead of everything created by the reconciler
	allAvailableCRDs, err := v.crdGetter.GetAll(ctx)
	if err != nil {
		return append(errs, field.InternalError(field.NewPath("spec"), xperrors.Wrap(err, "cannot get all available CRDs")))
	}
	for gvk, crd := range allAvailableCRDs {
		if gvk == compositeResGVK {
			continue
		}
		composedRes := &unstructured.UnstructuredList{}
		composedRes.SetGroupVersionKind(gvk)
		err := v.reconciler.GetClient().List(ctx, composedRes, client.MatchingLabels{xcrd.LabelKeyNamePrefixForComposed: compositeResourceValidationName})
		if err != nil {
			return append(errs, field.InternalError(field.NewPath("spec"), xperrors.Wrap(err, "cannot list composed resources")))
		}
		vs, _, err := k8sValidation.NewSchemaValidator(crd.Spec.Validation)
		if err != nil {
			return append(errs, field.InternalError(field.NewPath("spec"), xperrors.Wrap(err, "cannot create schema validator")))
		}
		for _, cd := range composedRes.Items {
			r := vs.Validate(cd.Object)
			if r.HasErrors() {
				sourceResourceIndex := findSourceResourceIndex(comp.Spec.Resources, cd)
				for _, err := range r.Errors {
					cdString, marshalErr := json.Marshal(cd)
					if marshalErr != nil {
						cdString = []byte(fmt.Sprintf("%+v", cd))
					}

					// if we can get the sourceResourceIndex, we can send out an error with more context.
					if sourceResourceIndex >= 0 {
						errs = append(errs, field.Invalid(
							field.NewPath("spec", "resources").Index(sourceResourceIndex).Child("base"),
							string(comp.Spec.Resources[sourceResourceIndex].Base.Raw),
							err.Error(),
						))
					} else {
						errs = append(errs, field.Invalid(field.NewPath("composedResource"), string(cdString), err.Error()))
					}
				}
			}
			if r.HasWarnings() {
				validationWarns = append(validationWarns, r.Warnings...)
			}
		}
	}
	if len(validationWarns) != 0 {
		// TODO (lsviben) send the warnings back
		fmt.Printf("there were some warnings while validating the rendered resources:\n%s\n", errors.Join(validationWarns...))
	}
	return errs
}

func genCompositeResource(comp *xpv1.Composition) (*xprComposite.Unstructured, schema.GroupVersionKind) {
	compositeResGVK := schema.FromAPIVersionAndKind(comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind)
	compositeRes := xprComposite.New(xprComposite.WithGroupVersionKind(compositeResGVK))
	compositeRes.SetName(compositeResourceValidationName)
	compositeRes.SetNamespace(compositeResourceValidationNamespace)
	compositeRes.SetCompositionReference(&corev1.ObjectReference{Name: comp.GetName()})
	return compositeRes, compositeResGVK
}

func findSourceResourceIndex(resources []xpv1.ComposedTemplate, composed unstructured.Unstructured) int {
	for i, r := range resources {
		if obj, err := r.GetBaseObject(); err == nil {
			if obj.GetObjectKind().GroupVersionKind() == composed.GetObjectKind().GroupVersionKind() && obj.GetName() == composite.GetCompositionResourceName(&composed) {
				return i
			}
		}
	}
	return -1
}

func sanitizedComposition(comp *xpv1.Composition) *xpv1.Composition {
	out := comp.DeepCopyObject().(*xpv1.Composition)
	out.CreationTimestamp = metav1.Time{}
	out.SetResourceVersion("")
	out.SetUID("")
	out.SetSelfLink("")
	out.SetGeneration(0)
	out.SetManagedFields(nil)
	return out
}
