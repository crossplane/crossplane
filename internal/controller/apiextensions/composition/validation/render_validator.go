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
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	composite2 "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
)

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
	if err := p.PureAPINamingConfigurator.Configure(ctx, compositeRes, nil); err != nil {
		return err
	}

	composedResources := make([]runtime.Object, len(resources))
	var patchingErr error
	// For each composed resource, validate its patches and then render it
	for i, resource := range resources {
		// validate patches using it and the compositeCrd resource
		cd := composed.New()
		if err := json.Unmarshal(resource.Base.Raw, cd); err != nil {
			patchingErr = errors.Join(patchingErr, fmt.Errorf("resource %s (%d): %w", *resource.Name, i, err))
			continue
		}
		composedGVK := cd.GetObjectKind().GroupVersionKind()
		patchCtx := PatchValidationRequest{
			GVKCRDValidation:          req.ManagedResourcesCRDs,
			CompositionValidationMode: req.ValidationMode,
			ComposedGVK:               composedGVK,
			CompositeGVK:              req.CompositeResGVK,
		}
		for j, patch := range resource.Patches {
			if err := ValidatePatch(patch, &patchCtx); err != nil {
				patchingErr = errors.Join(patchingErr, fmt.Errorf("resource %s (%d), patch %d: %w", *resource.Name, i, j, err))
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
		crdV, ok := req.ManagedResourcesCRDs[renderedComposed.GetObjectKind().GroupVersionKind()]
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
