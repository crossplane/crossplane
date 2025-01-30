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

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Validator validates the provided Composition.
type Validator struct {
	logicalValidation func(*v1.Composition) ([]string, field.ErrorList)
	crdGetter         CRDGetter
}

// CRDGetter is used to get all CRDs the Validator needs, either one by one or all at once.
type CRDGetter interface {
	Get(ctx context.Context, gvk schema.GroupKind) (*apiextensions.CustomResourceDefinition, error)
	GetAll(ctx context.Context) (map[schema.GroupKind]apiextensions.CustomResourceDefinition, error)
}

// ValidatorOption is used to configure the Validator.
type ValidatorOption func(*Validator)

// NewValidator returns a new Validator starting with the default configuration and applying
// the given options.
func NewValidator(opts ...ValidatorOption) (*Validator, error) {
	v := &Validator{}
	// Configure all default options and then any option passed in.
	for _, f := range append([]ValidatorOption{
		WithLogicalValidation(),
	},
		opts...) {
		f(v)
	}

	return v, v.isValid()
}

func (v *Validator) isValid() error {
	if v.crdGetter == nil {
		return xperrors.New("CRDGetterFn is required")
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
// Will return an error if the CRD is not found on calls to Get.
func WithCRDGetterFromMap(m map[schema.GroupKind]apiextensions.CustomResourceDefinition) ValidatorOption {
	return WithCRDGetter(crdGetterMap(m))
}

type crdGetterMap map[schema.GroupKind]apiextensions.CustomResourceDefinition

func (c crdGetterMap) Get(_ context.Context, gk schema.GroupKind) (*apiextensions.CustomResourceDefinition, error) {
	if crd, ok := c[gk]; ok {
		return &crd, nil
	}
	return nil, kerrors.NewNotFound(schema.GroupResource{Group: gk.Group, Resource: "CustomResourceDefinition"}, gk.String())
}

func (c crdGetterMap) GetAll(_ context.Context) (map[schema.GroupKind]apiextensions.CustomResourceDefinition, error) {
	return c, nil
}

// WithLogicalValidation returns a ValidatorOption that configures the Validator to use the given function to logically
// validate the Composition.
func WithLogicalValidation() ValidatorOption {
	return func(v *Validator) {
		v.logicalValidation = func(in *v1.Composition) ([]string, field.ErrorList) {
			return in.Validate()
		}
	}
}

// WithoutLogicalValidation returns a ValidatorOption that configures the Validator to not perform any logical check on
// the Composition.
func WithoutLogicalValidation() ValidatorOption {
	return func(v *Validator) {
		v.logicalValidation = func(*v1.Composition) ([]string, field.ErrorList) {
			return nil, nil
		}
	}
}

// Validate validates the provided Composition.
func (v *Validator) Validate(_ context.Context, obj runtime.Object) (warns []string, errs field.ErrorList) {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return nil, append(errs, field.NotSupported(field.NewPath("kind"), obj.GetObjectKind().GroupVersionKind().Kind, []string{v1.CompositionGroupVersionKind.Kind}))
	}

	// Validate the Composition itself
	if v.logicalValidation != nil {
		if warns, errs := v.logicalValidation(comp); len(errs) != 0 {
			return warns, errs
		}
	}

	// TODO(phisco): add more  phase 3 validation here
	return nil, errs
}
