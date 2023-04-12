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

// +kubebuilder:webhook:verbs=update;create,path=/validate-apiextensions-crossplane-io-v1-composition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositions,versions=v1,name=compositions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

package v1

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	// CompositionValidatingWebhookPath is the path for the Composition's validating webhook, should be kept in sync with the annotation above.
	CompositionValidatingWebhookPath = "/validate-apiextensions-crossplane-io-v1-composition"
	// CompositionValidationModeAnnotation is the annotation that can be used to specify the validation mode for a Composition.
	CompositionValidationModeAnnotation = "crossplane.io/composition-validation-mode"

	errFmtInvalidCompositionValidationMode = "invalid composition validation mode: %s"
)

// CompositionValidationMode is the validation mode for a Composition.
type CompositionValidationMode string

var (
	// DefaultCompositionValidationMode is the default validation mode for Compositions.
	DefaultCompositionValidationMode = CompositionValidationModeLoose
	// CompositionValidationModeLoose means that Compositions will be validated loosely, so no errors will be returned
	// in case of missing referenced resources, e.g. Managed Resources or Composite Resources.
	CompositionValidationModeLoose CompositionValidationMode = "loose"
	// CompositionValidationModeStrict means that Compositions will be validated strictly, so errors will be returned
	// in case of missing referenced resources, e.g. Managed Resources or Composite Resources.
	CompositionValidationModeStrict CompositionValidationMode = "strict"
)

// SetupWebhookWithManager sets up the Composition webhook with the provided manager and CustomValidator.
func (in *Composition) SetupWebhookWithManager(mgr ctrl.Manager, validator admission.CustomValidator) error {
	// Needed to inject validator in order to avoid dependency cycles.
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(validator).
		For(in).
		Complete()
}

// GetValidationMode returns the validation mode set for the composition.
func (in *Composition) GetValidationMode() (CompositionValidationMode, error) {
	if in.Annotations == nil {
		return DefaultCompositionValidationMode, nil
	}

	mode, ok := in.Annotations[CompositionValidationModeAnnotation]
	if !ok {
		return DefaultCompositionValidationMode, nil
	}

	switch mode := CompositionValidationMode(mode); mode {
	case CompositionValidationModeStrict, CompositionValidationModeLoose:
		return mode, nil
	}
	return "", errors.Errorf(errFmtInvalidCompositionValidationMode, mode)
}
