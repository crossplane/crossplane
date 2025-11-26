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
	"slices"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

// DefaultComparators returns all standard comparators.
func DefaultComparators() []Comparator {
	return []Comparator{
		CompareFieldRemoval,
		CompareTypeChange,
		CompareEnumValues,
		CompareRequiredFields,
		ComparePatternChange,
		CompareNumericConstraints,
		CompareStringConstraints,
		CompareArrayConstraints,
		CompareObjectConstraints,
		CompareCELValidations,
		CompareKubernetesExtensions,
	}
}

// CompareFieldRemoval detects removed properties.
func CompareFieldRemoval(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	for propName := range existing.Properties {
		if _, exists := proposed.Properties[propName]; !exists {
			changes = append(changes, SchemaChange{
				Path:    append(path, fieldpath.Field(propName)),
				Type:    ChangeTypeFieldRemoved,
				Message: "field removed (existing resources with this field cannot be read or updated)",
			})
		}
	}

	return changes
}

// CompareTypeChange detects type changes.
func CompareTypeChange(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	if existing.Type != proposed.Type && existing.Type != "" && proposed.Type != "" {
		return []SchemaChange{{
			Path:     path,
			Type:     ChangeTypeTypeChanged,
			Message:  fmt.Sprintf("type changed from %s to %s (existing resources will fail to deserialize)", existing.Type, proposed.Type),
			OldValue: existing.Type,
			NewValue: proposed.Type,
		}}
	}
	return nil
}

// CompareEnumValues detects enum value changes.
func CompareEnumValues(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	if len(existing.Enum) == 0 || len(proposed.Enum) == 0 {
		return nil
	}

	oldSet := jsonValueSet(existing.Enum)
	newSet := jsonValueSet(proposed.Enum)

	var changes []SchemaChange

	// Check for removed enum values (breaking)
	for val := range oldSet {
		if !newSet.Has(val) {
			changes = append(changes, SchemaChange{
				Path:     path,
				Type:     ChangeTypeEnumRestricted,
				Message:  fmt.Sprintf("enum value removed: %s (resources using this value will fail validation)", val),
				OldValue: val,
			})
		}
	}

	// Check for added enum values (non-breaking)
	for val := range newSet {
		if !oldSet.Has(val) {
			changes = append(changes, SchemaChange{
				Path:     path,
				Type:     ChangeTypeEnumExpanded,
				Message:  fmt.Sprintf("enum value added: %s", val),
				NewValue: val,
			})
		}
	}

	return changes
}

// CompareRequiredFields detects changes to required fields.
func CompareRequiredFields(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	if len(existing.Required) == 0 && len(proposed.Required) == 0 {
		return nil
	}

	oldSet := sets.New(existing.Required...)
	newSet := sets.New(proposed.Required...)

	// Only consider existing properties (not newly added ones)
	existingProps := sets.New[string]()
	for propName := range existing.Properties {
		existingProps.Insert(propName)
	}

	var changes []SchemaChange

	// Check for newly required fields on existing properties (breaking)
	for req := range newSet {
		if existingProps.Has(req) && !oldSet.Has(req) {
			// Check if the field has a default value
			hasDefault := false
			if newProp, exists := proposed.Properties[req]; exists {
				hasDefault = newProp.Default != nil && len(newProp.Default.Raw) > 0
			}

			if !hasDefault {
				changes = append(changes, SchemaChange{
					Path:     path,
					Type:     ChangeTypeRequiredAdded,
					Message:  fmt.Sprintf("field %s is now required without default (existing resources without this field will fail validation)", req),
					NewValue: req,
				})
			}
		}
	}

	// Check for removed required fields (non-breaking)
	for req := range oldSet {
		if !newSet.Has(req) {
			changes = append(changes, SchemaChange{
				Path:     path,
				Type:     ChangeTypeRequiredRemoved,
				Message:  fmt.Sprintf("field %s is no longer required", req),
				OldValue: req,
			})
		}
	}

	return changes
}

// ComparePatternChange detects regex pattern changes.
func ComparePatternChange(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	if existing.Pattern != proposed.Pattern && existing.Pattern != "" && proposed.Pattern != "" {
		return []SchemaChange{{
			Path:     path,
			Type:     ChangeTypePatternChanged,
			Message:  "regex pattern changed (values matching old pattern may no longer be valid)",
			OldValue: existing.Pattern,
			NewValue: proposed.Pattern,
		}}
	}
	return nil
}

// CompareNumericConstraints detects changes to numeric constraints.
func CompareNumericConstraints(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// Maximum tightened
	if existing.Maximum != nil && proposed.Maximum != nil && *proposed.Maximum < *existing.Maximum {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("maximum reduced from %v to %v (values above new maximum will fail validation)", *existing.Maximum, *proposed.Maximum),
			OldValue: fmt.Sprintf("%v", *existing.Maximum),
			NewValue: fmt.Sprintf("%v", *proposed.Maximum),
		})
	}

	// Minimum tightened
	if existing.Minimum != nil && proposed.Minimum != nil && *proposed.Minimum > *existing.Minimum {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("minimum increased from %v to %v (values below new minimum will fail validation)", *existing.Minimum, *proposed.Minimum),
			OldValue: fmt.Sprintf("%v", *existing.Minimum),
			NewValue: fmt.Sprintf("%v", *proposed.Minimum),
		})
	}

	// ExclusiveMaximum added or changed
	if !existing.ExclusiveMaximum && proposed.ExclusiveMaximum {
		changes = append(changes, SchemaChange{
			Path:    path,
			Type:    ChangeTypeConstraintTightened,
			Message: "maximum changed to exclusive (values equal to maximum will fail validation)",
		})
	}

	// ExclusiveMinimum added or changed
	if !existing.ExclusiveMinimum && proposed.ExclusiveMinimum {
		changes = append(changes, SchemaChange{
			Path:    path,
			Type:    ChangeTypeConstraintTightened,
			Message: "minimum changed to exclusive (values equal to minimum will fail validation)",
		})
	}

	// MultipleOf added or changed
	if existing.MultipleOf == nil && proposed.MultipleOf != nil {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("multipleOf constraint added: %v (values not divisible by %v will fail validation)", *proposed.MultipleOf, *proposed.MultipleOf),
			NewValue: fmt.Sprintf("%v", *proposed.MultipleOf),
		})
	} else if existing.MultipleOf != nil && proposed.MultipleOf != nil && *proposed.MultipleOf != *existing.MultipleOf {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("multipleOf changed from %v to %v (values valid under old constraint may fail)", *existing.MultipleOf, *proposed.MultipleOf),
			OldValue: fmt.Sprintf("%v", *existing.MultipleOf),
			NewValue: fmt.Sprintf("%v", *proposed.MultipleOf),
		})
	}

	return changes
}

// CompareStringConstraints detects changes to string constraints.
func CompareStringConstraints(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// MaxLength tightened
	if existing.MaxLength != nil && proposed.MaxLength != nil && *proposed.MaxLength < *existing.MaxLength {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("maxLength reduced from %d to %d (strings longer than %d will fail validation)", *existing.MaxLength, *proposed.MaxLength, *proposed.MaxLength),
			OldValue: fmt.Sprintf("%d", *existing.MaxLength),
			NewValue: fmt.Sprintf("%d", *proposed.MaxLength),
		})
	}

	// MinLength tightened
	if existing.MinLength != nil && proposed.MinLength != nil && *proposed.MinLength > *existing.MinLength {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("minLength increased from %d to %d (strings shorter than %d will fail validation)", *existing.MinLength, *proposed.MinLength, *proposed.MinLength),
			OldValue: fmt.Sprintf("%d", *existing.MinLength),
			NewValue: fmt.Sprintf("%d", *proposed.MinLength),
		})
	}

	return changes
}

// CompareArrayConstraints detects changes to array constraints.
func CompareArrayConstraints(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// MaxItems tightened
	if existing.MaxItems != nil && proposed.MaxItems != nil && *proposed.MaxItems < *existing.MaxItems {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("maxItems reduced from %d to %d (arrays with more than %d items will fail validation)", *existing.MaxItems, *proposed.MaxItems, *proposed.MaxItems),
			OldValue: fmt.Sprintf("%d", *existing.MaxItems),
			NewValue: fmt.Sprintf("%d", *proposed.MaxItems),
		})
	}

	// MinItems tightened
	if existing.MinItems != nil && proposed.MinItems != nil && *proposed.MinItems > *existing.MinItems {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("minItems increased from %d to %d (arrays with fewer than %d items will fail validation)", *existing.MinItems, *proposed.MinItems, *proposed.MinItems),
			OldValue: fmt.Sprintf("%d", *existing.MinItems),
			NewValue: fmt.Sprintf("%d", *proposed.MinItems),
		})
	}

	// UniqueItems added
	if !existing.UniqueItems && proposed.UniqueItems {
		changes = append(changes, SchemaChange{
			Path:    path,
			Type:    ChangeTypeConstraintTightened,
			Message: "uniqueItems constraint added (arrays with duplicate items will fail validation)",
		})
	}

	return changes
}

// CompareObjectConstraints detects changes to object constraints.
func CompareObjectConstraints(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// MaxProperties tightened
	if existing.MaxProperties != nil && proposed.MaxProperties != nil && *proposed.MaxProperties < *existing.MaxProperties {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("maxProperties reduced from %d to %d (objects with more than %d properties will fail validation)", *existing.MaxProperties, *proposed.MaxProperties, *proposed.MaxProperties),
			OldValue: fmt.Sprintf("%d", *existing.MaxProperties),
			NewValue: fmt.Sprintf("%d", *proposed.MaxProperties),
		})
	}

	// MinProperties tightened
	if existing.MinProperties != nil && proposed.MinProperties != nil && *proposed.MinProperties > *existing.MinProperties {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeConstraintTightened,
			Message:  fmt.Sprintf("minProperties increased from %d to %d (objects with fewer than %d properties will fail validation)", *existing.MinProperties, *proposed.MinProperties, *proposed.MinProperties),
			OldValue: fmt.Sprintf("%d", *existing.MinProperties),
			NewValue: fmt.Sprintf("%d", *proposed.MinProperties),
		})
	}

	return changes
}

// CompareCELValidations detects changes to CEL validation rules (conservative).
func CompareCELValidations(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	// Build sets of rules for comparison
	oldRules := sets.New[string]()
	for _, rule := range existing.XValidations {
		oldRules.Insert(rule.Rule)
	}

	newRules := sets.New[string]()
	for _, rule := range proposed.XValidations {
		newRules.Insert(rule.Rule)
	}

	var changes []SchemaChange

	// Check for new or modified rules (breaking)
	for _, rule := range proposed.XValidations {
		if !oldRules.Has(rule.Rule) {
			changes = append(changes, SchemaChange{
				Path:     path,
				Type:     ChangeTypeCELValidationAdded,
				Message:  fmt.Sprintf("CEL validation rule added or modified: %s (existing resources may fail new validation)", rule.Rule),
				NewValue: rule.Rule,
			})
		}
	}

	// Check for removed rules (non-breaking - less restrictive)
	for _, rule := range existing.XValidations {
		if !newRules.Has(rule.Rule) {
			changes = append(changes, SchemaChange{
				Path:     path,
				Type:     ChangeTypeCELValidationRemoved,
				Message:  fmt.Sprintf("CEL validation rule removed: %s", rule.Rule),
				OldValue: rule.Rule,
			})
		}
	}

	return changes
}

// CompareKubernetesExtensions detects changes to x-kubernetes extensions (conservative).
func CompareKubernetesExtensions(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
	var changes []SchemaChange

	// XPreserveUnknownFields: true → false is breaking (data loss)
	if existing.XPreserveUnknownFields != nil && *existing.XPreserveUnknownFields &&
		(proposed.XPreserveUnknownFields == nil || !*proposed.XPreserveUnknownFields) {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypePreserveUnknownChanged,
			Message:  "x-kubernetes-preserve-unknown-fields changed from true to false (unknown fields will be pruned)",
			OldValue: "true",
			NewValue: "false",
		})
	}

	// XListType changes are breaking
	if existing.XListType != nil && proposed.XListType != nil && *existing.XListType != *proposed.XListType {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeListTypeChanged,
			Message:  fmt.Sprintf("x-kubernetes-list-type changed from %s to %s (merge behavior will change)", *existing.XListType, *proposed.XListType),
			OldValue: *existing.XListType,
			NewValue: *proposed.XListType,
		})
	}

	// XMapType changes are breaking
	if existing.XMapType != nil && proposed.XMapType != nil && *existing.XMapType != *proposed.XMapType {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeMapTypeChanged,
			Message:  fmt.Sprintf("x-kubernetes-map-type changed from %s to %s (merge behavior will change)", *existing.XMapType, *proposed.XMapType),
			OldValue: *existing.XMapType,
			NewValue: *proposed.XMapType,
		})
	}

	// XListMapKeys changes are breaking
	if !slices.Equal(existing.XListMapKeys, proposed.XListMapKeys) && len(existing.XListMapKeys) > 0 {
		changes = append(changes, SchemaChange{
			Path:     path,
			Type:     ChangeTypeListTypeChanged,
			Message:  "x-kubernetes-list-map-keys changed (merge behavior will change)",
			OldValue: fmt.Sprintf("%v", existing.XListMapKeys),
			NewValue: fmt.Sprintf("%v", proposed.XListMapKeys),
		})
	}

	return changes
}

// Helper functions

func jsonValueSet(values []extv1.JSON) sets.Set[string] {
	set := sets.New[string]()
	for _, val := range values {
		set.Insert(string(val.Raw))
	}
	return set
}
