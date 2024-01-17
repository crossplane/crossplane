// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:verbs=update;create,path=/validate-apiextensions-crossplane-io-v1-composition,mutating=false,failurePolicy=fail,groups=apiextensions.crossplane.io,resources=compositions,versions=v1,name=compositions.apiextensions.crossplane.io,sideEffects=None,admissionReviewVersions=v1

package v1

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	// CompositionValidatingWebhookPath is the path for the Composition's validating webhook, should be kept in sync with the annotation above.
	CompositionValidatingWebhookPath = "/validate-apiextensions-crossplane-io-v1-composition"
	// SchemaAwareCompositionValidationModeAnnotation is the annotation that can be used to specify the schema-aware validation mode for a Composition.
	SchemaAwareCompositionValidationModeAnnotation = "crossplane.io/composition-schema-aware-validation-mode"

	errFmtInvalidCompositionValidationMode = "invalid schema-aware composition validation mode: %s"
)

// CompositionValidationMode is the validation mode for a Composition.
type CompositionValidationMode string

var (
	// DefaultSchemaAwareCompositionValidationMode is the default validation mode for Compositions.
	DefaultSchemaAwareCompositionValidationMode = SchemaAwareCompositionValidationModeWarn
	// SchemaAwareCompositionValidationModeWarn means only warnings will be
	// returned in case of errors during schema-aware validation, both for missing CRDs or any schema related error.
	SchemaAwareCompositionValidationModeWarn CompositionValidationMode = "warn"
	// SchemaAwareCompositionValidationModeLoose means that Compositions will be validated loosely, so no errors will be returned
	// in case of missing referenced resources, e.g. Managed Resources or Composite Resources.
	SchemaAwareCompositionValidationModeLoose CompositionValidationMode = "loose"
	// SchemaAwareCompositionValidationModeStrict means that Compositions will
	// be validated strictly, so errors will be returned in case of errors
	// during schema-aware validation or for missing resources' CRDs.
	SchemaAwareCompositionValidationModeStrict CompositionValidationMode = "strict"
)

// GetSchemaAwareValidationMode returns the schema-aware validation mode set for the Composition.
func (in *Composition) GetSchemaAwareValidationMode() (CompositionValidationMode, error) {
	if in.Annotations == nil {
		return DefaultSchemaAwareCompositionValidationMode, nil
	}

	mode, ok := in.Annotations[SchemaAwareCompositionValidationModeAnnotation]
	if !ok {
		return DefaultSchemaAwareCompositionValidationMode, nil
	}

	switch mode := CompositionValidationMode(mode); mode {
	case SchemaAwareCompositionValidationModeStrict, SchemaAwareCompositionValidationModeLoose, SchemaAwareCompositionValidationModeWarn:
		return mode, nil
	}
	return "", errors.Errorf(errFmtInvalidCompositionValidationMode, mode)
}
