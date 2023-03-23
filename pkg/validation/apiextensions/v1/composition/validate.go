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
	"errors"

	"github.com/crossplane/crossplane/internal/controller/apiextensions/composite"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Validator validates a Composition.
type Validator struct {
	client                client.Client
	logicalValidation     func(*v1.Composition) field.ErrorList
	crdGetter             CRDGetter
	shouldRenderResources func(*v1.Composition) bool

	reconciler *composite.Reconciler
}

// CRDGetter is used to get all CRDs the Validator needs, either one by one or all at once.
type CRDGetter interface {
	Get(ctx context.Context, gvk schema.GroupVersionKind) (*apiextensions.CustomResourceDefinition, error)
	GetAll(ctx context.Context) (map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition, error)
}

// GetClient returns the client used by the Validator.
func (v *Validator) GetClient() client.Client {
	return v.client
}

// ValidatorOption is used to configure the Validator.
type ValidatorOption func(*Validator)

// NewValidator returns a new Validator starting with the default configuration and applying
// the given options.
func NewValidator(opts ...ValidatorOption) (*Validator, error) {
	v := &Validator{
		shouldRenderResources: func(in *v1.Composition) bool {
			return len(in.Spec.Functions) == 0
		},
	}
	for _, defaultOpt := range []ValidatorOption{
		WithLogicalValidation(func(in *v1.Composition) field.ErrorList {
			return in.Validate()
		}),
	} {
		defaultOpt(v)
	}
	for _, opt := range opts {
		opt(v)
	}
	return v, v.isValid()
}

func (v *Validator) isValid() error {
	if v.crdGetter == nil {
		return errors.New("CRDGetterFn is required")
	}
	return nil
}

// WithCRDGetter returns a ValidatorOption that configure the validator to use the given CRDGetter to retrieve the CRD
// it needs.
func WithCRDGetter(c CRDGetter) ValidatorOption {
	return func(v *Validator) {
		v.crdGetter = c
	}
}

// WithCRDGetterFromMap returns a ValidatorOption that configure the Validator to use the given map as a CRDGetter.
func WithCRDGetterFromMap(m map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition) ValidatorOption {
	return WithCRDGetter(crdGetterMap(m))
}

type crdGetterMap map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition

func (c crdGetterMap) Get(_ context.Context, gvk schema.GroupVersionKind) (*apiextensions.CustomResourceDefinition, error) {
	if crd, ok := c[gvk]; ok {
		return &crd, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: "CustomResourceDefinition"}, gvk.String())
}

func (c crdGetterMap) GetAll(_ context.Context) (map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition, error) {
	return c, nil
}

// WithLogicalValidation returns a ValidatorOption that configures the Validator to use the given function to logically
// validate the Composition.
func WithLogicalValidation(f func(composition *v1.Composition) field.ErrorList) ValidatorOption {
	return func(v *Validator) {
		v.logicalValidation = f
	}
}

// WithoutLogicalValidation returns a ValidatorOption that configures the Validator to not perform any logical check on
// the Composition.
func WithoutLogicalValidation() ValidatorOption {
	return WithLogicalValidation(func(*v1.Composition) field.ErrorList {
		return nil
	})
}

// Validate validates the Composition by rendering it and then validating the rendered resources using the
// provided CustomValidator.
func (v *Validator) Validate(
	ctx context.Context,
	comp *v1.Composition,
) (errs field.ErrorList) {
	if errs := comp.Validate(); len(errs) != 0 {
		return errs
	}

	// Validate patches given the above CRDs, skip if any of the required CRDs is not available
	for _, f := range []func(context.Context, *v1.Composition) field.ErrorList{
		v.validatePatchesWithSchemas,
		v.validateConnectionDetailsWithSchemas,
		v.validateReadinessCheckWithSchemas,
	} {
		errs = append(errs, f(ctx, comp)...)
	}

	// we don't need to render resources if there are already errors
	if len(errs) != 0 {
		return errs
	}

	return v.renderAndValidateResources(ctx, comp)
}
