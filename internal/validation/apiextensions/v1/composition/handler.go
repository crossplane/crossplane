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

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Error strings.
const (
	errNotComposition = "supplied object was not a Composition"
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, _ controller.Options) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(&validator{}).
		For(&v1.Composition{}).
		Complete()
}

type validator struct{}

// ValidateCreate validates a Composition.
func (v *validator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	comp, ok := obj.(*v1.Composition)
	if !ok {
		return nil, errors.New(errNotComposition)
	}

	// TODO(negz): Could this be done with CEL validation instead?

	// Validate the composition itself, we'll disable it on the Validator below.
	warns, validationErrs := comp.Validate()
	if len(validationErrs) != 0 {
		return warns, kerrors.NewInvalid(comp.GroupVersionKind().GroupKind(), comp.GetName(), validationErrs)
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
