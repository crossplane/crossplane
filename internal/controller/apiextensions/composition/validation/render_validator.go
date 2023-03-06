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

package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	xprerrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	composite2 "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

const TrueVal = true

// RenderValidator is responsible for validating a composition after having rendered it.
type RenderValidator interface {
	RenderAndValidate(ctx context.Context, comp *v1.Composition, req *CompositionRenderValidationRequest) error
}

// PureValidator is a RenderValidator that does not use a client to validate the composition.
type PureValidator struct {
	PureRenderer              *composite.PureRenderer
	LogicalValidationChain    ValidationChain
	PureAPINamingConfigurator *composite.PureAPINamingConfigurator
}

// NewPureValidator returns a new PureValidator.
func NewPureValidator() *PureValidator {
	return &PureValidator{
		PureRenderer:              composite.NewPureRenderer(),
		LogicalValidationChain:    GetDefaultCompositionValidationChain(),
		PureAPINamingConfigurator: composite.NewPureAPINamingConfigurator(),
	}
}

// RenderAndValidate validates a composition after having rendered it.
func (p *PureValidator) RenderAndValidate(
	ctx context.Context,
	comp *v1.Composition,
	req *CompositionRenderValidationRequest,
) error {
	// dereference all patches first
	resources, err := composite.ComposedTemplates(comp.Spec)
	if err != nil {
		return err
	}

	// RenderAndValidate general assertions
	if err := p.LogicalValidationChain.Validate(comp); err != nil {
		return err
	}

	// Create a composite resource to validate patches against, setting all required fields
	compositeRes := composite2.New(composite2.WithGroupVersionKind(req.CompositeResGVK))
	compositeRes.SetUID("validation-uid")
	compositeRes.SetName("validation-name")
	if err, changed := p.PureAPINamingConfigurator.Configure(compositeRes); err != nil {
		return err
	} else if !changed {
		return nil
	}

	// Set all required fields on the composite resource
	if err := mockRequiredFields(compositeRes, req.CompositeResGVK, req.AvailableCRDs); err != nil {
		return err
	}

	_, compositeAvailable := req.AvailableCRDs[req.CompositeResGVK]

	composedResources := make([]runtime.Object, len(resources))
	var patchingErr error
	// For each composed resource, validate its patches and then render it
	for i, resource := range resources {
		var name string
		if resource.Name != nil {
			name = *resource.Name
		}
		// validate patches using it and the compositeCrd resource
		cd := composed.New()
		if err := json.Unmarshal(resource.Base.Raw, cd); err != nil {
			patchingErr = errors.Join(patchingErr, fmt.Errorf("resource %s (%d): %w", *resource.Name, i, err))
			continue
		}
		composedGVK := cd.GetObjectKind().GroupVersionKind()
		patchCtx := PatchValidationRequest{
			GVKCRDValidation:          req.AvailableCRDs,
			CompositionValidationMode: req.ValidationMode,
			ComposedGVK:               composedGVK,
			CompositeGVK:              req.CompositeResGVK,
		}

		// in loose mode we need to mock all fields of the composite resource used by a FromCompositeFieldPath patch to
		// be able to apply patches successfully
		if req.ValidationMode == "loose" && !compositeAvailable {
			o, err := fieldpath.PaveObject(compositeRes)
			if err != nil {
				return err
			}
			var changed bool
			for _, patch := range resource.Patches {
				if patch.Type == "" || patch.Type == v1.PatchTypeFromCompositeFieldPath {
					// get the toFieldPath type from the composed resource CRD if available
					if composedCRD, ok := req.AvailableCRDs[composedGVK]; ok {
						toFieldPathType, err := getFieldPathType(composedCRD.OpenAPIV3Schema, safeDeref(patch.ToFieldPath))
						if err != nil {
							return err
						}
						if err := setTypeDefaultValue(o, safeDeref(patch.FromFieldPath), toFieldPathType); err != nil {
							return err
						}
						changed = true
					}
				}
			}
			if changed {
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), compositeRes); err != nil {
					return err
				}
			}
		}
		for j, patch := range resource.Patches {
			if err := ValidatePatch(patch, &patchCtx); err != nil {
				patchingErr = errors.Join(patchingErr, fmt.Errorf("resource %s (%d), patch %d: %w", name, i, j, err))
				continue
			}
		}

		// TODO: handle env too
		if err := p.PureRenderer.Render(ctx, compositeRes, cd, resource, nil); err != nil {
			patchingErr = errors.Join(patchingErr, err)
			continue
		}
		composedResources[i] = cd
	}

	if patchingErr != nil {
		return patchingErr
	}

	var renderError error
	// RenderAndValidate Rendered Composed Resources from Composition
	for _, renderedComposed := range composedResources {
		crdV, ok := req.AvailableCRDs[renderedComposed.GetObjectKind().GroupVersionKind()]
		if !ok {
			if req.ValidationMode == v1.CompositionValidationModeStrict {
				renderError = errors.Join(renderError, xprerrors.Errorf("No CRD validation found for rendered resource: %v", renderedComposed.GetObjectKind().GroupVersionKind()))
				continue
			}
			continue
		}
		vs, _, err := validation.NewSchemaValidator(&crdV)
		if err != nil {
			return err
		}
		r := vs.Validate(renderedComposed)
		if r.HasErrors() {
			renderError = errors.Join(renderError, errors.Join(r.Errors...))
		}
		// TODO: handle warnings
	}

	if renderError != nil {
		return renderError
	}
	return nil
}

// getFieldPathType returns the type of a field path in a given schema
func getFieldPathType(o *apiextensions.JSONSchemaProps, path string) (string, error) {
	segments, err := fieldpath.Parse(path)
	if err != nil {
		return "", err
	}
	current := o
	for _, segment := range segments {
		if segment.Type == fieldpath.SegmentIndex {
			if current.Items == nil {
				return "", nil
			}
			if len(current.Items.JSONSchemas) > 0 {
				current = &current.Items.JSONSchemas[segment.Index]
				continue
			}
			if current.Items.Schema != nil {
				current = current.Items.Schema
				continue
			}
			// means there is no schema for this index
			return "", nil
		}
		if current.Properties != nil {
			if c, ok := current.Properties[segment.Field]; ok {
				current = &c
				continue
			}
		}
		if current.AdditionalProperties != nil && current.AdditionalProperties.Allows && current.AdditionalProperties.Schema != nil && current.AdditionalProperties.Schema.Type != "" {
			current = current.AdditionalProperties.Schema
			continue
		}
	}
	if current != nil {
		return current.Type, nil
	}
	return "", nil
}

func mockRequiredFields(res *composite2.Unstructured, gvk schema.GroupVersionKind, ds GVKValidationMap) error {
	o, err := fieldpath.PaveObject(res)
	if err != nil {
		return err
	}
	v, ok := ds[gvk]
	if !ok {
		return nil
	}
	if v.OpenAPIV3Schema == nil {
		return nil
	}
	err = mockRequiredFieldsSchemaProps(v.OpenAPIV3Schema, o, "")
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(o.UnstructuredContent(), res)

}

// mockRequiredFieldsSchemaPropos mock required fields for a given schema property
func mockRequiredFieldsSchemaProps(prop *apiextensions.JSONSchemaProps, o *fieldpath.Paved, path string) error {
	if prop == nil {
		return nil
	}
	switch prop.Type {
	case "string":
		if prop.Default == nil {
			return setTypeDefaultValue(o, path, prop.Type)
		}
		v := *prop.Default
		vs, ok := v.(string)
		if !ok {
			return fmt.Errorf("default value for %s is not a string", path)
		}
		return o.SetString(path, vs)
	case "integer":
		if prop.Default == nil {
			return setTypeDefaultValue(o, path, prop.Type)
		}
		v := *prop.Default
		vs, ok := v.(float64)
		if !ok {
			return fmt.Errorf("default value for %s is not an integer", path)
		}
		return o.SetNumber(path, vs)
	case "object":
		for _, s := range prop.Required {
			p := prop.Properties[s]
			err := mockRequiredFieldsSchemaProps(&p, o, strings.TrimLeft(strings.Join([]string{path, s}, "."), "."))
			if err != nil {
				return err
			}
		}
		return nil
	case "array":
		return nil
	}
	return nil
}

// setTypeDefaultValue sets the default value for a given type at a given path
func setTypeDefaultValue(o *fieldpath.Paved, path string, t string) error {
	switch t {
	case "string":
		return o.SetString(path, "default")
	case "integer":
		return o.SetNumber(path, 1)
	}
	return nil
}

func safeDeref[T any](ptr *T) T {
	var zero T
	if ptr == nil {
		return zero
	}
	return *ptr
}
