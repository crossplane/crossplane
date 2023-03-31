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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Validator validates the provided Composition.
type Validator struct{}

// Validate validates the provided Composition.
func (c *Validator) Validate(_ context.Context, obj runtime.Object) (warns []string, err error) {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return warns, xperrors.New("not a v1 Composition")
	}

	// Validate the composition itself, we'll disable it on the Validator below
	if errs := comp.Validate(); len(errs) != 0 {
		return warns, apierrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), errs)
	}
	// TODO(phisco): get schemas and validate the Composition against it
	return nil, nil
}
