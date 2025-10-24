/*
Copyright 2025 The Crossplane Authors.

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

package xcrd

import (
	"fmt"
	"strings"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

// VersionResult contains the comparison results for a specific CRD version.
type VersionResult struct {
	// Version is the name of the CRD version.
	Version string

	// Changes contains all schema changes found in this version.
	Changes []SchemaChange
}

// BreakingChangesError returns a formatted error if any breaking changes exist
// in the provided version results, or nil if none exist.
func BreakingChangesError(versions ...VersionResult) error {
	var breaking []SchemaChange
	var firstVersion string

	for _, v := range versions {
		for _, c := range v.Changes {
			if c.Type.Breaking() {
				if len(breaking) == 0 {
					firstVersion = v.Version
				}
				breaking = append(breaking, c)
			}
		}
	}

	if len(breaking) == 0 {
		return nil
	}

	msg := fmt.Sprintf("version %s: %s at %s", firstVersion, breaking[0].Message, breaking[0].Path)
	if len(breaking) > 1 {
		msg += fmt.Sprintf(" (and %d more)", len(breaking)-1)
	}

	return fmt.Errorf("%s", msg)
}

// SchemaChange represents a single schema change.
type SchemaChange struct {
	// Path is the field path to the changed field (e.g., "spec.forProvider.tags").
	Path fieldpath.Segments

	// Type describes what kind of change occurred.
	Type ChangeType

	// Message is a human-readable description of the change.
	Message string

	// OldValue is the old value (if applicable).
	OldValue string

	// NewValue is the new value (if applicable).
	NewValue string
}

// ChangeType describes the kind of schema change.
type ChangeType string

// Breaking changes.
const (
	ChangeTypeFieldRemoved           ChangeType = "FieldRemoved"
	ChangeTypeTypeChanged            ChangeType = "TypeChanged"
	ChangeTypeEnumRestricted         ChangeType = "EnumRestricted"
	ChangeTypeRequiredAdded          ChangeType = "RequiredAdded"
	ChangeTypePatternChanged         ChangeType = "PatternChanged"
	ChangeTypeConstraintTightened    ChangeType = "ConstraintTightened"
	ChangeTypeCELValidationAdded     ChangeType = "CELValidationAdded"
	ChangeTypeListTypeChanged        ChangeType = "ListTypeChanged"
	ChangeTypeMapTypeChanged         ChangeType = "MapTypeChanged"
	ChangeTypePreserveUnknownChanged ChangeType = "PreserveUnknownChanged"
)

// Non-breaking changes.
const (
	ChangeTypeFieldAdded           ChangeType = "FieldAdded"
	ChangeTypeEnumExpanded         ChangeType = "EnumExpanded"
	ChangeTypeRequiredRemoved      ChangeType = "RequiredRemoved"
	ChangeTypeConstraintLoosened   ChangeType = "ConstraintLoosened"
	ChangeTypeCELValidationRemoved ChangeType = "CELValidationRemoved"
)

// Breaking returns true if this change type is breaking.
func (ct ChangeType) Breaking() bool {
	switch ct {
	case ChangeTypeFieldRemoved,
		ChangeTypeTypeChanged,
		ChangeTypeEnumRestricted,
		ChangeTypeRequiredAdded,
		ChangeTypePatternChanged,
		ChangeTypeConstraintTightened,
		ChangeTypeCELValidationAdded,
		ChangeTypeListTypeChanged,
		ChangeTypeMapTypeChanged,
		ChangeTypePreserveUnknownChanged:
		return true
	case ChangeTypeFieldAdded,
		ChangeTypeEnumExpanded,
		ChangeTypeRequiredRemoved,
		ChangeTypeConstraintLoosened,
		ChangeTypeCELValidationRemoved:
		return false
	default:
		return false
	}
}

// CompareOption configures schema comparison behavior.
type CompareOption func(*CompareConfig)

// CompareConfig configures schema comparison behavior.
type CompareConfig struct {
	allowAlphaBreaking bool
	comparators        []Comparator
}

// WithAlphaExemption allows breaking changes in alpha API versions.
// Versions containing "alpha" in their name will skip validation.
func WithAlphaExemption() CompareOption {
	return func(c *CompareConfig) {
		c.allowAlphaBreaking = true
	}
}

// WithComparators specifies which comparators to use.
// If not specified, all default comparators are used.
func WithComparators(comparators ...Comparator) CompareOption {
	return func(c *CompareConfig) {
		c.comparators = comparators
	}
}

// CompareSchemas compares two CRD schemas for compatibility.
// By default, uses all available comparators and allows alpha breaking changes.
// Returns a slice of VersionResult, one for each version that was compared.
func CompareSchemas(existing, proposed *extv1.CustomResourceDefinition, opts ...CompareOption) []VersionResult {
	cfg := &CompareConfig{
		allowAlphaBreaking: true,
		comparators:        DefaultComparators(),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	results := make([]VersionResult, 0, len(proposed.Spec.Versions))

	comparators := Comparators(cfg.comparators)

	// Build map of existing versions by name
	existingVersions := make(map[string]*extv1.CustomResourceDefinitionVersion, len(existing.Spec.Versions))
	for i := range existing.Spec.Versions {
		existingVersions[existing.Spec.Versions[i].Name] = &existing.Spec.Versions[i]
	}

	// Compare each version
	for _, pv := range proposed.Spec.Versions {
		// Skip alpha versions if configured
		if cfg.allowAlphaBreaking && strings.Contains(strings.ToLower(pv.Name), "alpha") {
			continue
		}

		// Find corresponding existing version
		ev, exists := existingVersions[pv.Name]
		if !exists {
			// New version added - not breaking
			continue
		}

		// Verify schemas exist
		if ev.Schema == nil || ev.Schema.OpenAPIV3Schema == nil {
			continue
		}
		if pv.Schema == nil || pv.Schema.OpenAPIV3Schema == nil {
			continue
		}

		// Compare schemas recursively
		changes := comparators.compareSchemaProps(
			nil, // Start with empty path segments
			ev.Schema.OpenAPIV3Schema,
			pv.Schema.OpenAPIV3Schema,
		)

		// Add to results with version
		results = append(results, VersionResult{
			Version: pv.Name,
			Changes: changes,
		})
	}

	return results
}

// A Comparator checks for specific types of schema changes.
// The path parameter is a slice of field path segments.
type Comparator func(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange

// Comparators is a list of comparators that can be used to compare CRD schemas.
type Comparators []Comparator

// compareSchemaProps recursively compares schema properties and returns changes.
// The path parameter is a slice of field path segments.
func (c Comparators) compareSchemaProps(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// Run all comparators at this level
	for _, comparator := range c {
		changes = append(changes, comparator(path, existing, proposed)...)
	}

	// Recurse into properties
	for name, existingProp := range existing.Properties {
		proposedProp, exists := proposed.Properties[name]
		if !exists {
			// Property removed - already caught by FieldRemoval comparator
			continue
		}

		changes = append(changes, c.compareSchemaProps(append(path, fieldpath.Field(name)), &existingProp, &proposedProp)...)
	}

	// Recurse into array items
	if hasArrayItems(existing, proposed) {
		changes = append(changes, c.compareSchemaProps(append(path, fieldpath.Field("items")), existing.Items.Schema, proposed.Items.Schema)...)
	}

	// Recurse into AllOf
	for i := range min(len(existing.AllOf), len(proposed.AllOf)) {
		changes = append(changes, c.compareSchemaProps(
			append(path, fieldpath.Field("allOf"), fieldpath.FieldOrIndex(fmt.Sprintf("%d", i))),
			&existing.AllOf[i],
			&proposed.AllOf[i],
		)...)
	}

	// Recurse into OneOf
	for i := range min(len(existing.OneOf), len(proposed.OneOf)) {
		changes = append(changes, c.compareSchemaProps(
			append(path, fieldpath.Field("oneOf"), fieldpath.FieldOrIndex(fmt.Sprintf("%d", i))),
			&existing.OneOf[i],
			&proposed.OneOf[i],
		)...)
	}

	// Recurse into AnyOf
	for i := range min(len(existing.AnyOf), len(proposed.AnyOf)) {
		changes = append(changes, c.compareSchemaProps(
			append(path, fieldpath.Field("anyOf"), fieldpath.FieldOrIndex(fmt.Sprintf("%d", i))),
			&existing.AnyOf[i],
			&proposed.AnyOf[i],
		)...)
	}

	return changes
}

// hasArrayItems returns true if both schemas are arrays with item schemas.
func hasArrayItems(existing, proposed *extv1.JSONSchemaProps) bool {
	if existing.Type != "array" || proposed.Type != "array" {
		return false
	}
	if existing.Items == nil || existing.Items.Schema == nil {
		return false
	}
	return proposed.Items != nil && proposed.Items.Schema != nil
}
