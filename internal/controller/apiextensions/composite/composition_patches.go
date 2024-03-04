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

package composite

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

const (
	errPatchSetType             = "a patch in a PatchSet cannot be of type PatchSet"
	errCombineRequiresVariables = "combine patch types require at least one variable"

	errFmtUndefinedPatchSet           = "cannot find PatchSet by name %s"
	errFmtInvalidPatchType            = "patch type %s is unsupported"
	errFmtCombineStrategyNotSupported = "combine strategy %s is not supported"
	errFmtCombineConfigMissing        = "given combine strategy %s requires configuration"
	errFmtCombineStrategyFailed       = "%s strategy could not combine"
	errFmtExpandingArrayFieldPaths    = "cannot expand ToFieldPath %s"
)

// ApplyEnvironmentPatch executes a patching operation between the cp and env objects.
func ApplyEnvironmentPatch(p v1.EnvironmentPatch, cp, env runtime.Object) error {
	// TODO(negz): Should this take composite.Resource and *env.Environment as
	// arguments rather than generic runtime.Objects?

	regularPatch := p.ToPatch()
	if regularPatch == nil {
		// Should never happen, but just in case, nothing to do.
		return nil
	}
	// To make thing easy, we are going to reuse the logic of a regular
	// composition patch.
	return ApplyToObjects(
		*regularPatch,
		cp,
		env,
		v1.PatchTypeFromCompositeFieldPath,
		v1.PatchTypeCombineFromComposite,
		v1.PatchTypeToCompositeFieldPath,
		v1.PatchTypeCombineToComposite,
	)
}

// Apply executes a patching operation between the from and to resources.
// Applies all patch types unless an 'only' filter is supplied.
func Apply(p v1.Patch, cp resource.Composite, cd resource.Composed, only ...v1.PatchType) error {
	return ApplyToObjects(p, cp, cd, only...)
}

// ApplyToObjects works like c.Apply but accepts any kind of runtime.Object
// (such as EnvironmentConfigs).
// It might be vulnerable to conversion panics
// (see https://github.com/crossplane/crossplane/pull/3394 for details).
func ApplyToObjects(p v1.Patch, cp, cd runtime.Object, only ...v1.PatchType) error {
	if filterPatch(p, only...) {
		return nil
	}

	switch p.GetType() {
	case v1.PatchTypeFromCompositeFieldPath, v1.PatchTypeFromEnvironmentFieldPath:
		return ApplyFromFieldPathPatch(p, cp, cd)
	case v1.PatchTypeToCompositeFieldPath, v1.PatchTypeToEnvironmentFieldPath:
		return ApplyFromFieldPathPatch(p, cd, cp)
	case v1.PatchTypeCombineFromComposite, v1.PatchTypeCombineFromEnvironment:
		return ApplyCombineFromVariablesPatch(p, cp, cd)
	case v1.PatchTypeCombineToComposite, v1.PatchTypeCombineToEnvironment:
		return ApplyCombineFromVariablesPatch(p, cd, cp)
	case v1.PatchTypePatchSet:
		// Already resolved - nothing to do.
	}
	return errors.Errorf(errFmtInvalidPatchType, p.Type)
}

// filterPatch returns true if patch should be filtered (not applied).
func filterPatch(p v1.Patch, only ...v1.PatchType) bool {
	// filter does not apply if not set
	if len(only) == 0 {
		return false
	}

	for _, patchType := range only {
		if patchType == p.Type {
			return false
		}
	}
	return true
}

// ResolveTransforms applies a list of transforms to a patch value.
func ResolveTransforms(c v1.Patch, input any) (any, error) {
	var err error
	for i, t := range c.Transforms {
		if input, err = Resolve(t, input); err != nil {
			// TODO(negz): Including the type might help find the offending transform faster.
			return nil, errors.Wrapf(err, errFmtTransformAtIndex, i)
		}
	}
	return input, nil
}

// patchFieldValueToMultiple, given a path with wildcards in an array index,
// expands the arrays paths in the "to" object and patches the value into each
// of the resulting fields, returning any errors as they occur.
func patchFieldValueToMultiple(fieldPath string, value any, to runtime.Object, mo *xpv1.MergeOptions) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}

	arrayFieldPaths, err := paved.ExpandWildcards(fieldPath)
	if err != nil {
		return err
	}

	if len(arrayFieldPaths) == 0 {
		return errors.Errorf(errFmtExpandingArrayFieldPaths, fieldPath)
	}

	for _, field := range arrayFieldPaths {
		if err := paved.MergeValue(field, value, mo); err != nil {
			return err
		}
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}

// ApplyFromFieldPathPatch patches the "to" resource, using a source field
// on the "from" resource. Values may be transformed if any are defined on
// the patch.
func ApplyFromFieldPathPatch(p v1.Patch, from, to runtime.Object) error {
	if p.FromFieldPath == nil {
		return errors.Errorf(errFmtRequiredField, "FromFieldPath", p.Type)
	}

	// Default to patching the same field on the composed resource.
	if p.ToFieldPath == nil {
		p.ToFieldPath = p.FromFieldPath
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	in, err := fieldpath.Pave(fromMap).GetValue(*p.FromFieldPath)
	if IsOptionalFieldPathNotFound(err, p.Policy) {
		return nil
	}
	if err != nil {
		return err
	}

	var mo *xpv1.MergeOptions
	if p.Policy != nil {
		mo = p.Policy.MergeOptions
	}

	// Apply transform pipeline
	out, err := ResolveTransforms(p, in)
	if err != nil {
		return err
	}

	// Patch all expanded fields if the ToFieldPath contains wildcards
	if strings.Contains(*p.ToFieldPath, "[*]") {
		return patchFieldValueToMultiple(*p.ToFieldPath, out, to, mo)
	}

	return patchFieldValueToObject(*p.ToFieldPath, out, to, mo)
}

// ApplyCombineFromVariablesPatch patches the "to" resource, taking a list of
// input variables and combining them into a single output value.
// The single output value may then be further transformed if they are defined
// on the patch.
func ApplyCombineFromVariablesPatch(p v1.Patch, from, to runtime.Object) error {
	// Combine patch requires configuration
	if p.Combine == nil {
		return errors.Errorf(errFmtRequiredField, "Combine", p.Type)
	}
	// Destination field path is required since we can't default to multiple
	// fields.
	if p.ToFieldPath == nil {
		return errors.Errorf(errFmtRequiredField, "ToFieldPath", p.Type)
	}

	vl := len(p.Combine.Variables)

	if vl < 1 {
		return errors.New(errCombineRequiresVariables)
	}

	fromMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return err
	}

	in := make([]any, vl)

	// Get value of each variable
	// NOTE: This currently assumes all variables define a 'fromFieldPath'
	// value. If we add new variable types, this may not be the case and
	// this code may be better served split out into a dedicated function.
	for i, sp := range p.Combine.Variables {
		iv, err := fieldpath.Pave(fromMap).GetValue(sp.FromFieldPath)

		// If any source field is not found, we will not
		// apply the patch. This is to avoid situations
		// where a combine patch is expecting a fixed
		// number of inputs (e.g. a string format
		// expecting 3 fields '%s-%s-%s' but only
		// receiving 2 values).
		if IsOptionalFieldPathNotFound(err, p.Policy) {
			return nil
		}
		if err != nil {
			return err
		}
		in[i] = iv
	}

	// Combine input values
	cb, err := Combine(*p.Combine, in)
	if err != nil {
		return err
	}

	// Apply transform pipeline
	out, err := ResolveTransforms(p, cb)
	if err != nil {
		return err
	}

	return patchFieldValueToObject(*p.ToFieldPath, out, to, nil)
}

// IsOptionalFieldPathNotFound returns true if the supplied error indicates a
// field path was not found, and the supplied policy indicates a patch from that
// field path was optional.
func IsOptionalFieldPathNotFound(err error, p *v1.PatchPolicy) bool {
	switch {
	case p == nil:
		fallthrough
	case p.FromFieldPath == nil:
		fallthrough
	case *p.FromFieldPath == v1.FromFieldPathPolicyOptional:
		return fieldpath.IsNotFound(err)
	default:
		return false
	}
}

// Combine calls the appropriate combiner.
func Combine(c v1.Combine, vars []any) (any, error) {
	var out any
	var err error

	switch c.Strategy {
	case v1.CombineStrategyString:
		if c.String == nil {
			return nil, errors.Errorf(errFmtCombineConfigMissing, c.Strategy)
		}
		out, err = CombineString(c.String.Format, vars)
	default:
		return nil, errors.Errorf(errFmtCombineStrategyNotSupported, c.Strategy)
	}

	// Note: There are currently no tests or triggers to exercise this error as
	// our only strategy ("String") uses fmt.Sprintf, which cannot return an error.
	return out, errors.Wrapf(err, errFmtCombineStrategyFailed, string(c.Strategy))
}

// CombineString returns a single output by running a string format with all of
// its input variables.
func CombineString(format string, vars []any) (any, error) {
	return fmt.Sprintf(format, vars...), nil
}

// ComposedTemplates returns the supplied composed resource templates with any
// supplied patchsets dereferenced.
func ComposedTemplates(pss []v1.PatchSet, cts []v1.ComposedTemplate) ([]v1.ComposedTemplate, error) {
	pn := make(map[string][]v1.Patch)
	for _, s := range pss {
		for _, p := range s.Patches {
			if p.Type == v1.PatchTypePatchSet {
				return nil, errors.New(errPatchSetType)
			}
		}
		pn[s.Name] = s.Patches
	}

	ct := make([]v1.ComposedTemplate, len(cts))
	for i, r := range cts {
		var po []v1.Patch
		for _, p := range r.Patches {
			if p.Type != v1.PatchTypePatchSet {
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
