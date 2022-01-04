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

package v1

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
)

const (
	errPatchSetType             = "a patch in a PatchSet cannot be of type PatchSet"
	errCombineRequiresVariables = "combine patch types require at least one variable"

	errFmtRequiredField               = "%s is required by type %s"
	errFmtUndefinedPatchSet           = "cannot find PatchSet by name %s"
	errFmtInvalidPatchType            = "patch type %s is unsupported"
	errFmtCombineStrategyNotSupported = "combine strategy %s is not supported"
	errFmtCombineConfigMissing        = "given combine strategy %s requires configuration"
	errFmtCombineStrategyFailed       = "%s strategy could not combine"
)

// A PatchType is a type of patch.
type PatchType string

// Patch types.
const (
	PatchTypeFromCompositeFieldPath PatchType = "FromCompositeFieldPath" // Default
	PatchTypePatchSet               PatchType = "PatchSet"
	PatchTypeToCompositeFieldPath   PatchType = "ToCompositeFieldPath"
	PatchTypeCombineFromComposite   PatchType = "CombineFromComposite"
	PatchTypeCombineToComposite     PatchType = "CombineToComposite"
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

// Patch objects are applied between composite and composed resources. Their
// behaviour depends on the Type selected. The default Type,
// FromCompositeFieldPath, copies a value from the composite resource to
// the composed resource, applying any defined transformers.
type Patch struct {
	// Type sets the patching behaviour to be used. Each patch type may require
	// its' own fields to be set on the Patch object.
	// +optional
	// +kubebuilder:validation:Enum=FromCompositeFieldPath;PatchSet;ToCompositeFieldPath;CombineFromComposite;CombineToComposite
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

// Apply executes a patching operation between the from and to resources.
// Applies all patch types unless an 'only' filter is supplied.
func (c *Patch) Apply(cp, cd runtime.Object, only ...PatchType) error {
	if c.filterPatch(only...) {
		return nil
	}

	switch c.Type {
	case PatchTypeFromCompositeFieldPath:
		return c.applyFromFieldPathPatch(cp, cd)
	case PatchTypeToCompositeFieldPath:
		return c.applyFromFieldPathPatch(cd, cp)
	case PatchTypeCombineFromComposite:
		return c.applyCombineFromVariablesPatch(cp, cd)
	case PatchTypeCombineToComposite:
		return c.applyCombineFromVariablesPatch(cd, cp)
	case PatchTypePatchSet:
		// Already resolved - nothing to do.
	}
	return errors.Errorf(errFmtInvalidPatchType, c.Type)
}

// filterPatch returns true if patch should be filtered (not applied)
func (c *Patch) filterPatch(only ...PatchType) bool {
	// filter does not apply if not set
	if len(only) == 0 {
		return false
	}

	for _, patchType := range only {
		if patchType == c.Type {
			return false
		}
	}
	return true
}

// applyTransforms applies a list of transforms to a patch value.
func (c *Patch) applyTransforms(input interface{}) (interface{}, error) {
	var err error
	for i, t := range c.Transforms {
		if input, err = t.Transform(input); err != nil {
			return nil, errors.Wrapf(err, errFmtTransformAtIndex, i)
		}
	}
	return input, nil
}

// patchFieldValueToObject, given a path, value and "to" object, will
// apply the value to the "to" object at the given path, returning
// any errors as they occur.
func patchFieldValueToObject(fieldPath string, value interface{}, to runtime.Object, mo *xpv1.MergeOptions) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}

	if err := paved.MergeValue(fieldPath, value, mo); err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}

// applyFromFieldPathPatch patches the "to" resource, using a source field
// on the "from" resource. Values may be transformed if any are defined on
// the patch.
func (c *Patch) applyFromFieldPathPatch(from, to runtime.Object) error {
	if c.FromFieldPath == nil {
		return errors.Errorf(errFmtRequiredField, "FromFieldPath", c.Type)
	}

	// Default to patching the same field on the composed resource.
	if c.ToFieldPath == nil {
		c.ToFieldPath = c.FromFieldPath
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*c.FromFieldPath)
	if IsOptionalFieldPathNotFound(err, c.Policy) {
		return nil
	}
	if err != nil {
		return err
	}

	var mo *xpv1.MergeOptions
	if c.Policy != nil {
		mo = c.Policy.MergeOptions
	}

	// Apply transform pipeline
	out, err := c.applyTransforms(in)
	if err != nil {
		return err
	}

	return patchFieldValueToObject(*c.ToFieldPath, out, to, mo)
}

// applyCombineFromVariablesPatch patches the "to" resource, taking a list of
// input variables and combining them into a single output value.
// The single output value may then be further transformed if they are defined
// on the patch.
func (c *Patch) applyCombineFromVariablesPatch(from, to runtime.Object) error {
	// Combine patch requires configuration
	if c.Combine == nil {
		return errors.Errorf(errFmtRequiredField, "Combine", c.Type)
	}
	// Destination field path is required since we can't default to multiple
	// fields.
	if c.ToFieldPath == nil {
		return errors.Errorf(errFmtRequiredField, "ToFieldPath", c.Type)
	}

	vl := len(c.Combine.Variables)

	if vl < 1 {
		return errors.New(errCombineRequiresVariables)
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	in := make([]interface{}, vl)

	// Get value of each variable
	// NOTE: This currently assumes all variables define a 'fromFieldPath'
	// value. If we add new variable types, this may not be the case and
	// this code may be better served split out into a dedicated function.
	for i, sp := range c.Combine.Variables {
		iv, err := fieldpath.Pave(fromMap).GetValue(sp.FromFieldPath)

		// If any source field is not found, we will not
		// apply the patch. This is to avoid situations
		// where a combine patch is expecting a fixed
		// number of inputs (e.g. a string format
		// expecting 3 fields '%s-%s-%s' but only
		// receiving 2 values).
		if IsOptionalFieldPathNotFound(err, c.Policy) {
			return nil
		}
		if err != nil {
			return err
		}
		in[i] = iv
	}

	// Combine input values
	cb, err := c.Combine.Combine(in)
	if err != nil {
		return err
	}

	// Apply transform pipeline
	out, err := c.applyTransforms(cb)
	if err != nil {
		return err
	}

	return patchFieldValueToObject(*c.ToFieldPath, out, to, nil)
}

// IsOptionalFieldPathNotFound returns true if the supplied error indicates a
// field path was not found, and the supplied policy indicates a patch from that
// field path was optional.
func IsOptionalFieldPathNotFound(err error, s *PatchPolicy) bool {
	switch {
	case s == nil:
		fallthrough
	case s.FromFieldPath == nil:
		fallthrough
	case *s.FromFieldPath == FromFieldPathPolicyOptional:
		return fieldpath.IsNotFound(err)
	default:
		return false
	}
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

// Combine returns a single output by running a string format
// with all of its' input variables.
func (s *StringCombine) Combine(vars []interface{}) (interface{}, error) {
	return fmt.Sprintf(s.Format, vars...), nil
}

// Combine calls the appropriate combiner.
func (c *Combine) Combine(vars []interface{}) (interface{}, error) {
	var combiner interface {
		Combine(vars []interface{}) (interface{}, error)
	}

	switch c.Strategy {
	case CombineStrategyString:
		combiner = c.String
	default:
		return nil, errors.Errorf(errFmtCombineStrategyNotSupported, string(c.Strategy))
	}

	// Check for nil interface requires reflection.
	if reflect.ValueOf(combiner).IsNil() {
		return nil, errors.Errorf(errFmtCombineConfigMissing, string(c.Strategy))
	}
	out, err := combiner.Combine(vars)
	// Note: There are currently no tests or triggers to exercise this error as
	// our only strategy ("String") uses fmt.Sprintf, which cannot return an error.
	return out, errors.Wrapf(err, errFmtCombineStrategyFailed, string(c.Strategy))
}

// ComposedTemplates returns a revision's composed resource templates with any
// patchsets dereferenced.
func (rs *CompositionSpec) ComposedTemplates() ([]ComposedTemplate, error) {
	pn := make(map[string][]Patch)
	for _, s := range rs.PatchSets {
		for _, p := range s.Patches {
			if p.Type == PatchTypePatchSet {
				return nil, errors.New(errPatchSetType)
			}
		}
		pn[s.Name] = s.Patches
	}

	ct := make([]ComposedTemplate, len(rs.Resources))
	for i, r := range rs.Resources {
		po := []Patch{}
		for _, p := range r.Patches {
			if p.Type != PatchTypePatchSet {
				po = append(po, p)
				continue
			}
			if p.PatchSetName == nil {
				return nil, errors.Errorf(errFmtRequiredField, "PatchSetName", p.Type)
			}
			ps, ok := pn[*p.PatchSetName]
			if !ok {
				return nil, errors.Errorf(errFmtUndefinedPatchSet, *p.PatchSetName)
			}
			po = append(po, ps...)
		}
		ct[i] = r
		ct[i].Patches = po
	}
	return ct, nil
}
