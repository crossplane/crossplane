/*
Copyright 202333he Crossplane Authors.

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

package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	xprerrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	metadataSchema = apiextensions.JSONSchemaProps{
		Type: "object",
		AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
			Allows: true,
		},
		Properties: map[string]apiextensions.JSONSchemaProps{
			"name": {
				Type: "string",
			},
			"namespace": {
				Type: "string",
			},
			"labels": {
				Type: "object",
				AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
					Schema: &apiextensions.JSONSchemaProps{
						Type: "string",
					},
				},
			},
			"annotations": {
				Type: "object",
				AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
					Schema: &apiextensions.JSONSchemaProps{
						Type: "string",
					},
				},
			},
			"uid": {
				Type: "string",
			},
		},
	}
)

// ClientValidator gathers required information using the provided client.Reader and then use them to render and
// validated a Composition.
type ClientValidator struct {
	client          client.Reader
	renderValidator RenderValidator
}

// CompositionRenderValidationRequest should contain all the information needed to validate a Composition using a
// RenderValidator.
type CompositionRenderValidationRequest struct {
	CompositeResGVK      schema.GroupVersionKind
	ManagedResourcesCRDs GVKValidationMap
	ValidationMode       v1.CompositionValidationMode
}

// SetupWithManager sets up the ClientValidator with the provided manager, setting up all the required indexes it requires.
func (c *ClientValidator) SetupWithManager(mgr ctrl.Manager) error {
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
	c.client = unstructured.NewClient(mgr.GetClient())
	c.renderValidator = NewPureValidator()
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(c).
		For(&v1.Composition{}).
		Complete()
}

// ValidateCreate validates the Composition by rendering it and then validating the rendered resources.
func (c *ClientValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return xprerrors.New(errUnexpectedType)
	}

	if err := IsValidatable(comp); err != nil {
		fmt.Println("HERE: Composition is not validatable", err)
		return nil
	}

	// Get the validation mode set through annotations for the composition
	validationMode, err := GetCompositionValidationMode(comp)
	if err != nil {
		return err
	}

	// Get schema for Composite Resource Definition defined by comp.Spec.CompositeTypeRef
	compositeResGVK := schema.FromAPIVersionAndKind(comp.Spec.CompositeTypeRef.APIVersion,
		comp.Spec.CompositeTypeRef.Kind)

	// Get schema for
	compositeCrdValidation, err := GetCRDValidationForGVK(ctx, c.client, &compositeResGVK, validationMode)
	if err != nil {
		return err
	}
	// Get schema for all Managed Resources in comp.Spec.Resources[*].Base
	managedResourcesCRDs, err := GetBasesCRDs(ctx, c.client, comp.Spec.Resources, validationMode)
	if err != nil {
		return err
	}
	if compositeCrdValidation != nil {
		managedResourcesCRDs[compositeResGVK] = *compositeCrdValidation
	}

	if err := c.renderValidator.RenderAndValidate(
		ctx,
		comp,
		&CompositionRenderValidationRequest{
			CompositeResGVK:      compositeResGVK,
			ManagedResourcesCRDs: managedResourcesCRDs,
			ValidationMode:       validationMode,
		},
	); err != nil {
		return apierrors.NewBadRequest(errors.Join(errors.New("invalid composition"), err).Error())
	}

	return nil
}

// ValidateUpdate is a no-op for now.
func (c *ClientValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return c.ValidateCreate(ctx, newObj)
}

// ValidateDelete is a no-op for now.
func (c *ClientValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// GetCRDValidationForGVK returns the validation schema for the given GVK, by looking up the CRD by group and kind using
// the provided client.
func GetCRDValidationForGVK(ctx context.Context, c client.Reader, gvk *schema.GroupVersionKind, validationMode v1.CompositionValidationMode) (*apiextensions.CustomResourceValidation, error) {
	crds := extv1.CustomResourceDefinitionList{}
	if err := c.List(ctx, &crds, client.MatchingFields{"spec.group": gvk.Group},
		client.MatchingFields{"spec.names.kind": gvk.Kind}); err != nil {
		return nil, err
	}
	switch len(crds.Items) {
	case 0:
		if validationMode == v1.CompositionValidationModeStrict {
			return nil, fmt.Errorf("no CRDs found: %v", gvk)
		}
		return nil, nil
	case 1:
		crd := crds.Items[0]
		internal := &apiextensions.CustomResourceDefinition{}
		if err := extv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&crd, internal, nil); err != nil {
			return nil, err
		}
		if v := internal.Spec.Validation; v != nil {
			return v, nil
		}
		for _, version := range internal.Spec.Versions {
			if version.Name == gvk.Version {
				return version.Schema, nil
			}
		}
		return nil, fmt.Errorf("no CRD found for version: %v, %v", gvk, crd)
	}

	return nil, fmt.Errorf("too many CRDs found: %v, %v", gvk, crds)
}

// GetBasesCRDs returns a map of GVK to CRD validation schema for all the CRDs that are used in the given resources,
// using the provided client, which should be able to list CRDs by group and kind.
func GetBasesCRDs(ctx context.Context, c client.Reader, resources []v1.ComposedTemplate, validationMode v1.CompositionValidationMode) (GVKValidationMap, error) {
	gvkToCRDV := make(GVKValidationMap)
	for _, resource := range resources {
		cd := composed.New()
		if err := json.Unmarshal(resource.Base.Raw, cd); err != nil {
			return nil, err
		}
		gvk := cd.GetObjectKind().GroupVersionKind()
		if _, ok := gvkToCRDV[gvk]; ok {
			continue
		}
		crdv, err := GetCRDValidationForGVK(ctx, c, &gvk, validationMode)
		if err != nil {
			return nil, err
		}
		if crdv != nil {
			gvkToCRDV[gvk] = *crdv
		}
	}
	return gvkToCRDV, nil
}

// IsValidatable returns true if the composition is validatable.
func IsValidatable(comp *v1.Composition) error {
	if comp == nil {
		return fmt.Errorf("composition is nil")
	}
	// If the composition has any functions, it is not validatable.
	if len(comp.Spec.Functions) > 0 {
		return fmt.Errorf("composition has functions")
	}
	// If the composition uses any patch that we don't yet handle, it is not validatable.
	for _, set := range comp.Spec.PatchSets {
		for _, patch := range set.Patches {
			patch := patch
			if !IsValidatablePatchType(&patch) {
				return fmt.Errorf("composition uses patch type that is not yet validatable: %s", patch.Type)
			}
		}
	}
	for _, resource := range comp.Spec.Resources {
		for _, patch := range resource.Patches {
			patch := patch
			if !IsValidatablePatchType(&patch) {
				return fmt.Errorf("composition uses patch type that is not yet validatable: %s", patch.Type)
			}
		}
	}
	return nil
}

// GetCompositionValidationMode returns the validation mode set for the composition.
func GetCompositionValidationMode(comp *v1.Composition) (v1.CompositionValidationMode, error) {
	if comp.Annotations == nil {
		return v1.DefaultCompositionValidationMode, nil
	}

	mode, ok := comp.Annotations[v1.CompositionValidationModeAnnotation]
	if !ok {
		return v1.DefaultCompositionValidationMode, nil
	}

	switch mode := v1.CompositionValidationMode(mode); mode {
	case v1.CompositionValidationModeStrict, v1.CompositionValidationModeLoose:
		return mode, nil
	}
	return "", xprerrors.Errorf("invalid composition validation mode: %s", mode)
}
