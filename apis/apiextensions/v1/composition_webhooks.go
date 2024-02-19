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
func (c *Composition) GetSchemaAwareValidationMode() (CompositionValidationMode, error) {
	if c.Annotations == nil {
		return DefaultSchemaAwareCompositionValidationMode, nil
	}

	mode, ok := c.Annotations[SchemaAwareCompositionValidationModeAnnotation]
	if !ok {
		return DefaultSchemaAwareCompositionValidationMode, nil
	}

	switch mode := CompositionValidationMode(mode); mode {
	case SchemaAwareCompositionValidationModeStrict, SchemaAwareCompositionValidationModeLoose, SchemaAwareCompositionValidationModeWarn:
		return mode, nil
	}
	return "", errors.Errorf(errFmtInvalidCompositionValidationMode, mode)
}
