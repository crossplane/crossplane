/*
Copyright 2020 The Crossplane Authors.

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

// Generated from apiextensions/v1/composition_patches.go by ../hack/duplicate_api_type.sh. DO NOT EDIT.

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	verrors "github.com/crossplane/crossplane/internal/validation/errors"
)

// A PatchType is a type of patch.
type PatchType string

// Patch types.
const (
	PatchTypeFromCompositeFieldPath   PatchType = "FromCompositeFieldPath" // Default
	PatchTypeFromEnvironmentFieldPath PatchType = "FromEnvironmentFieldPath"
	PatchTypePatchSet                 PatchType = "PatchSet"
	PatchTypeToCompositeFieldPath     PatchType = "ToCompositeFieldPath"
	PatchTypeToEnvironmentFieldPath   PatchType = "ToEnvironmentFieldPath"
	PatchTypeCombineFromEnvironment   PatchType = "CombineFromEnvironment"
	PatchTypeCombineFromComposite     PatchType = "CombineFromComposite"
	PatchTypeCombineToComposite       PatchType = "CombineToComposite"
	PatchTypeCombineToEnvironment     PatchType = "CombineToEnvironment"
)

// A FromFieldPathPolicy determines how to patch from a field path.
type FromFieldPathPolicy string

// FromFieldPath patch policies.
const (
	FromFieldPathPolicyOptional FromFieldPathPolicy = "Optional"
	FromFieldPathPolicyRequired FromFieldPathPolicy = "Required"
)

// A PatchPolicy configures the specifics of patching behaviour.
type PatchPolicy struct {
	// FromFieldPath specifies how to patch from a field path. The default is
	// 'Optional', which means the patch will be a no-op if the specified
	// fromFieldPath does not exist. Use 'Required' if the patch should fail if
	// the specified path does not exist.
	// +kubebuilder:validation:Enum=Optional;Required
	// +optional
	FromFieldPath *FromFieldPathPolicy `json:"fromFieldPath,omitempty"`
	MergeOptions  *xpv1.MergeOptions   `json:"mergeOptions,omitempty"`
}

// GetFromFieldPathPolicy returns the FromFieldPathPolicy for this PatchPolicy, defaulting to FromFieldPathPolicyOptional if not specified.
func (pp *PatchPolicy) GetFromFieldPathPolicy() FromFieldPathPolicy {
	if pp == nil || pp.FromFieldPath == nil {
		return FromFieldPathPolicyOptional
	}
	return *pp.FromFieldPath
}

// Patch objects are applied between composite and composed resources. Their
// behaviour depends on the Type selected. The default Type,
// FromCompositeFieldPath, copies a value from the composite resource to
// the composed resource, applying any defined transformers.
type Patch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;FromEnvironmentFieldPath;PatchSet;ToCompositeFieldPath;ToEnvironmentFieldPath;CombineFromEnvironment;CombineFromComposite;CombineToComposite;CombineToEnvironment
	// +kubebuilder:default=FromCompositeFieldPath
	Type PatchType `json:"type,omitempty"`

	// FromFieldPath is the path of the field on the resource whose value is
	// to be used as input. Required when type is FromCompositeFieldPath,
	// FromEnvironmentFieldPath, ToCompositeFieldPath, ToEnvironmentFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Combine is the patch configuration for a CombineFromComposite,
	// CombineFromEnvironment, CombineToComposite or CombineToEnvironment patch.
	// +optional
	Combine *Combine `json:"combine,omitempty"`

	// ToFieldPath is the path of the field on the resource whose value will
	// be changed with the result of transforms. Leave empty if you'd like to
	// propagate to the same path as fromFieldPath.
	// +optional
	ToFieldPath *string `json:"toFieldPath,omitempty"`

	// PatchSetName to include patches from. Required when type is PatchSet.
	// +optional
	PatchSetName *string `json:"patchSetName,omitempty"`

	// Transforms are the list of functions that are used as a FIFO pipe for the
	// input to be transformed.
	// +optional
	Transforms []Transform `json:"transforms,omitempty"`

	// Policy configures the specifics of patching behaviour.
	// +optional
	Policy *PatchPolicy `json:"policy,omitempty"`
}

// GetFromFieldPath returns the FromFieldPath for this Patch, or an empty string if it is nil.
func (p *Patch) GetFromFieldPath() string {
	if p.FromFieldPath == nil {
		return ""
	}
	return *p.FromFieldPath
}

// GetToFieldPath returns the ToFieldPath for this Patch, or an empty string if it is nil.
func (p *Patch) GetToFieldPath() string {
	if p.ToFieldPath == nil {
		return ""
	}
	return *p.ToFieldPath
}

// GetType returns the patch type. If the type is not set, it returns the default type.
func (p *Patch) GetType() PatchType {
	if p.Type == "" {
		return PatchTypeFromCompositeFieldPath
	}
	return p.Type
}

// Validate the Patch object.
func (p *Patch) Validate() *field.Error {
	switch p.GetType() {
	case PatchTypeFromCompositeFieldPath, PatchTypeFromEnvironmentFieldPath, PatchTypeToCompositeFieldPath, PatchTypeToEnvironmentFieldPath:
		if p.FromFieldPath == nil {
			return field.Required(field.NewPath("fromFieldPath"), fmt.Sprintf("fromFieldPath must be set for patch type %s", p.Type))
		}
	case PatchTypePatchSet:
		if p.PatchSetName == nil {
			return field.Required(field.NewPath("patchSetName"), fmt.Sprintf("patchSetName must be set for patch type %s", p.Type))
		}
	case PatchTypeCombineFromEnvironment, PatchTypeCombineFromComposite, PatchTypeCombineToComposite, PatchTypeCombineToEnvironment:
		if p.Combine == nil {
			return field.Required(field.NewPath("combine"), fmt.Sprintf("combine must be set for patch type %s", p.Type))
		}
		if p.ToFieldPath == nil {
			return field.Required(field.NewPath("toFieldPath"), fmt.Sprintf("toFieldPath must be set for patch type %s", p.Type))
		}
	default:
		// Should never happen
		return field.Invalid(field.NewPath("type"), p.Type, "unknown patch type")
	}
	for i, transform := range p.Transforms {
		if err := transform.Validate(); err != nil {
			return verrors.WrapFieldError(err, field.NewPath("transforms").Index(i))
		}
	}

	return nil
}

// A CombineVariable defines the source of a value that is combined with
// others to form and patch an output value. Currently, this only supports
// retrieving values from a field path.
type CombineVariable struct {
	// FromFieldPath is the path of the field on the source whose value is
	// to be used as input.
	FromFieldPath string `json:"fromFieldPath"`
}

// A CombineStrategy determines what strategy will be applied to combine
// variables.
type CombineStrategy string

// CombineStrategy strategy definitions.
const (
	CombineStrategyString CombineStrategy = "string"
)

// A Combine configures a patch that combines more than
// one input field into a single output field.
type Combine struct {
	// Variables are the list of variables whose values will be retrieved and
	// combined.
	// +kubebuilder:validation:MinItems=1
	Variables []CombineVariable `json:"variables"`

	// Strategy defines the strategy to use to combine the input variable values.
	// Currently only string is supported.
	// +kubebuilder:validation:Enum=string
	Strategy CombineStrategy `json:"strategy"`

	// String declares that input variables should be combined into a single
	// string, using the relevant settings for formatting purposes.
	// +optional
	String *StringCombine `json:"string,omitempty"`
}

// A StringCombine combines multiple input values into a single string.
type StringCombine struct {
	// Format the input using a Go format string. See
	// https://golang.org/pkg/fmt/ for details.
	Format string `json:"fmt"`
}
