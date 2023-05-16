/*
Copyright 2022 The Crossplane Authors.

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

package v1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane/internal/validation/errors"
)

// An EnvironmentConfiguration specifies the environment for rendering composed
// resources.
type EnvironmentConfiguration struct {
	// EnvironmentConfigs selects a list of `EnvironmentConfig`s. The resolved
	// resources are stored in the composite resource at
	// `spec.environmentConfigRefs` and is only updated if it is null.
	//
	// The list of references is used to compute an in-memory environment at
	// compose time. The data of all object is merged in the order they are
	// listed, meaning the values of EnvironmentConfigs with a larger index take
	// priority over ones with smaller indices.
	//
	// The computed environment can be accessed in a composition using
	// `FromEnvironmentFieldPath` and `CombineFromEnvironment` patches.
	// +optional
	EnvironmentConfigs []EnvironmentSource `json:"environmentConfigs,omitempty"`

	// Patches is a list of environment patches that are executed before a
	// composition's resources are composed.
	Patches []EnvironmentPatch `json:"patches,omitempty"`
}

// Validate the EnvironmentConfiguration.
func (e *EnvironmentConfiguration) Validate() field.ErrorList {
	errs := field.ErrorList{}

	for i, p := range e.Patches {
		if err := errors.WrapFieldError(p.Validate(), field.NewPath("patches").Index(i)); err != nil {
			errs = append(errs, err)
		}
	}

	for i, ec := range e.EnvironmentConfigs {
		if err := errors.WrapFieldError(ec.Validate(), field.NewPath("environmentConfigs").Index(i)); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// EnvironmentSourceType specifies the way the EnvironmentConfig is selected.
type EnvironmentSourceType string

const (
	// EnvironmentSourceTypeReference by name.
	EnvironmentSourceTypeReference EnvironmentSourceType = "Reference"
	// EnvironmentSourceTypeSelector by labels.
	EnvironmentSourceTypeSelector EnvironmentSourceType = "Selector"
)

// EnvironmentSource selects a EnvironmentConfig resource.
type EnvironmentSource struct {
	// Type specifies the way the EnvironmentConfig is selected.
	// Default is `Reference`
	// +optional
	// +kubebuilder:validation:Enum=Reference;Selector
	// +kubebuilder:default=Reference
	Type EnvironmentSourceType `json:"type,omitempty"`

	// Ref is a named reference to a single EnvironmentConfig.
	// Either Ref or Selector is required.
	// +optional
	Ref *EnvironmentSourceReference `json:"ref,omitempty"`

	// Selector selects one EnvironmentConfig via labels.
	// +optional
	Selector *EnvironmentSourceSelector `json:"selector,omitempty"`
}

// Validate the EnvironmentSource.
func (e *EnvironmentSource) Validate() *field.Error {
	switch e.Type {
	case EnvironmentSourceTypeReference:
		if e.Ref == nil {
			return field.Required(field.NewPath("ref"), "ref is required")
		}
		if err := e.Ref.Validate(); err != nil {
			return errors.WrapFieldError(err, field.NewPath("ref"))
		}

	case EnvironmentSourceTypeSelector:
		if e.Selector == nil {
			return field.Required(field.NewPath("selector"), "selector is required")
		}
		if len(e.Selector.MatchLabels) == 0 {
			return field.Required(field.NewPath("selector", "matchLabels"), "selector must have at least one match label")
		}
		for i, m := range e.Selector.MatchLabels {
			if err := m.Validate(); err != nil {
				return errors.WrapFieldError(err, field.NewPath("selector", "matchLabels").Index(i))
			}
		}
	default:
		return field.Invalid(field.NewPath("type"), e.Type, "invalid type")
	}
	return nil
}

// An EnvironmentSourceReference references an EnvironmentConfig by it's name.
type EnvironmentSourceReference struct {
	// The name of the object.
	Name string `json:"name"`
}

// Validate the EnvironmentSourceReference.
func (e *EnvironmentSourceReference) Validate() *field.Error {
	if e.Name == "" {
		return field.Required(field.NewPath("name"), "name is required")
	}
	return nil
}

// An EnvironmentSourceSelector selects an EnvironmentConfig via labels.
type EnvironmentSourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels []EnvironmentSourceSelectorLabelMatcher `json:"matchLabels,omitempty"`
}

// EnvironmentSourceSelectorLabelMatcherType specifies where the value for a
// label comes from.
type EnvironmentSourceSelectorLabelMatcherType string

const (
	// EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath extracts
	// the label value from a composite fieldpath.
	EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath EnvironmentSourceSelectorLabelMatcherType = "FromCompositeFieldPath"
	// EnvironmentSourceSelectorLabelMatcherTypeValue uses a literal as label
	// value.
	EnvironmentSourceSelectorLabelMatcherTypeValue EnvironmentSourceSelectorLabelMatcherType = "Value"
)

// An EnvironmentSourceSelectorLabelMatcher acts like a k8s label selector but
// can draw the label value from a different path.
type EnvironmentSourceSelectorLabelMatcher struct {
	// Type specifies where the value for a label comes from.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;Value
	// +kubebuilder:default=FromCompositeFieldPath
	Type EnvironmentSourceSelectorLabelMatcherType `json:"type"`

	// Key of the label to match.
	Key string `json:"key"`

	// ValueFromFieldPath specifies the field path to look for the label value.
	ValueFromFieldPath *string `json:"valueFromFieldPath,omitempty"`

	// Value specifies a literal label value.
	Value *string `json:"value,omitempty"`
}

// GetType returns the type of the label matcher, returning the default if not set.
func (e *EnvironmentSourceSelectorLabelMatcher) GetType() EnvironmentSourceSelectorLabelMatcherType {
	if e == nil {
		return EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath
	}
	return e.Type
}

// Validate logically validate the EnvironmentSourceSelectorLabelMatcher.
func (e *EnvironmentSourceSelectorLabelMatcher) Validate() *field.Error {
	if e.Key == "" {
		return field.Required(field.NewPath("key"), "key is required")
	}
	switch e.GetType() {
	case EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath:
		if e.ValueFromFieldPath == nil {
			return field.Required(field.NewPath("valueFromFieldPath"), "valueFromFieldPath is required")
		}
		if *e.ValueFromFieldPath == "" {
			return field.Required(field.NewPath("valueFromFieldPath"), "valueFromFieldPath must not be empty")
		}
	case EnvironmentSourceSelectorLabelMatcherTypeValue:
		if e.Value == nil {
			return field.Required(field.NewPath("value"), "value is required")
		}
		if *e.Value == "" {
			return field.Required(field.NewPath("value"), "value must not be empty")
		}
	default:
		return field.Invalid(field.NewPath("type"), e.Type, "invalid type")
	}
	return nil
}

// EnvironmentPatch is a patch for a Composition environment.
type EnvironmentPatch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;ToCompositeFieldPath;CombineFromComposite;CombineToComposite
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the resource whose value is
	// to be used as input. Required when type is FromCompositeFieldPath or
	// ToCompositeFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Combine is the patch configuration for a CombineFromComposite or
	// CombineToComposite patch.
	// +optional
	Combine *Combine `json:"combine,omitempty"`

	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as fromFieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	Transforms []Transform `json:"transforms,omitempty"`

	// Policy configures the specifics of patching behaviour.
	// +optional
	Policy *PatchPolicy `json:"policy,omitempty"`
}

// ToPatch converts the EnvironmentPatch to a Patch.
func (e *EnvironmentPatch) ToPatch() *Patch {
	if e == nil {
		return nil
	}
	return &Patch{
		Type:          e.Type,
		FromFieldPath: e.FromFieldPath,
		Combine:       e.Combine,
		ToFieldPath:   e.ToFieldPath,
		Transforms:    e.Transforms,
		Policy:        e.Policy,
	}
}

// Validate validates the EnvironmentPatch.
func (e *EnvironmentPatch) Validate() *field.Error {
	p := e.ToPatch()
	if p == nil {
		return nil
	}
	return p.Validate()
}
