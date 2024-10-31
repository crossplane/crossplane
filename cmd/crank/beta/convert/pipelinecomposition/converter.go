/*
Copyright 2024 The Crossplane Authors.

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

package pipelinecomposition

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func convertPnTToPipeline(in *unstructured.Unstructured, functionRefName string) (*unstructured.Unstructured, error) {
	if in == nil {
		return nil, errors.New("input is nil")
	}

	gvk := in.GetObjectKind().GroupVersionKind()

	if gvk.Empty() {
		return nil, errors.New("GroupVersionKind is empty")
	}

	if gvk.Group != v1.Group {
		return nil, errors.Errorf("GroupVersionKind Group is not %s", v1.Group)
	}

	if gvk.Kind != v1.CompositionKind {
		return nil, errors.Errorf("GroupVersionKind Kind is not %s", v1.CompositionKind)
	}

	out, err := fieldpath.PaveObject(in)
	if err != nil {
		return nil, err
	}

	var mode v1.CompositionMode
	err = out.GetValueInto("spec.mode", &mode)
	switch {
	case fieldpath.IsNotFound(err):
		mode = v1.CompositionModeResources
	case err != nil:
		return nil, errors.Wrap(err, "failed to get composition mode")
	case mode == v1.CompositionModePipeline:
		// nothing to do
		return nil, nil
	}

	// Set up the pipeline step
	if err := out.SetValue("spec.mode", v1.CompositionModePipeline); err != nil {
		return nil, errors.Wrap(err, "failed to set Composition mode to Pipeline")
	}

	// Composition Environment settings are now handled by both function-patch-and-transform and function-environment-configs
	// Prepare function-patch-and-transform input
	fptInputPaved := fieldpath.Pave(map[string]any{
		"apiVersion": "pt.fn.crossplane.io/v1beta1",
		"kind":       "Resources",
	})

	// Copy spec.environment.patches to function-patch-and-transform, if any
	if err := migrateEnvironmentPatches(out, fptInputPaved); err != nil && !fieldpath.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to migrate environment")
	}

	// Copy spec.patchSets, if any, and migrate all patches
	if err := migratePatchSets(out, fptInputPaved); err != nil && !fieldpath.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to migrate patchSets")
	}

	// Copy spec.resources, if any, and migrate all patches
	if err := migrateResources(out, fptInputPaved); err != nil {
		return nil, errors.Wrap(err, "failed to migrate resources")
	}

	// Override function name if provided
	if functionRefName == "" {
		functionRefName = "function-patch-and-transform"
	}

	if err := out.SetValue("spec.pipeline[0].step", "patch-and-transform"); err != nil {
		return nil, errors.Wrap(err, "failed to set pipeline step")
	}

	if err := out.SetValue("spec.pipeline[0].functionRef.name", functionRefName); err != nil {
		return nil, errors.Wrap(err, "failed to set pipeline functionRef name")
	}

	if err := out.SetValue("spec.pipeline[0].input", fptInputPaved.UnstructuredContent()); err != nil {
		return nil, errors.Wrap(err, "failed to set pipeline input")
	}

	return &unstructured.Unstructured{Object: out.UnstructuredContent()}, nil
}

func migrateResources(out *fieldpath.Paved, fptInputPaved *fieldpath.Paved) error {
	var resources []map[string]any
	if err := out.GetValueInto("spec.resources", &resources); err == nil {
		for idx, resource := range resources {
			r, err := migrateResource(resource, idx)
			if err != nil {
				return errors.Wrap(err, "failed to migrate resource")
			}
			resources[idx] = r.UnstructuredContent()
		}
		if err := fptInputPaved.SetValue("resources", resources); err != nil {
			return errors.Wrap(err, "failed to copy resources")
		}
	}
	if err := out.DeleteField("spec.resources"); err != nil {
		return errors.Wrap(err, "failed to delete resources")
	}
	return nil
}

func migratePatchSets(out *fieldpath.Paved, fptInputPaved *fieldpath.Paved) error {
	var patchSets []map[string]any
	if err := out.GetValueInto("spec.patchSets", &patchSets); err != nil {
		return errors.Wrap(err, "failed to get patchSets")
	}
	if err := fptInputPaved.SetValue("patchSets", patchSets); err != nil {
		return errors.Wrap(err, "failed to copy patchSets")
	}
	if err := out.DeleteField("spec.patchSets"); err != nil {
		return errors.Wrap(err, "failed to delete patchSets")
	}
	paths, err := fptInputPaved.ExpandWildcards("patchSets[*].patches[*]")
	if err != nil {
		return errors.Wrap(err, "failed to expand patchSets")
	}
	for _, path := range paths {
		p := map[string]any{}
		if err := fptInputPaved.GetValueInto(path, &p); err != nil {
			return errors.Wrap(err, "failed to get patch")
		}
		paved, err := migratePatch(p)
		if err != nil {
			return errors.Wrap(err, "failed to migrate patch")
		}
		if err := fptInputPaved.SetValue(path, paved.UnstructuredContent()); err != nil {
			return errors.Wrap(err, "failed to set patch")
		}
	}
	return nil
}

func migrateEnvironmentPatches(out *fieldpath.Paved, fptInputPaved *fieldpath.Paved) error {
	var envPatches []map[string]any
	if err := out.GetValueInto("spec.environment.patches", &envPatches); err == nil {
		for idx, patch := range envPatches {
			p, err := migratePatch(patch)
			if err != nil {
				return errors.Wrap(err, "failed to set environment patch type")
			}
			envPatches[idx] = p.UnstructuredContent()
		}
		if err := fptInputPaved.SetValue("environment.patches", envPatches); err != nil {
			return errors.Wrap(err, "failed to set environment patches")
		}
		if err := out.DeleteField("spec.environment.patches"); err != nil {
			return errors.Wrap(err, "failed to delete environment patches")
		}
	}

	if err := out.DeleteField("spec.environment.patches"); err != nil && !fieldpath.IsNotFound(err) {
		return errors.Wrap(err, "failed to delete environment")
	}

	env := map[string]any{}
	if err := out.GetValueInto("spec.environment", &env); err != nil {
		return errors.Wrap(err, "failed to get environment")
	}

	if len(env) != 0 {
		// other fields left in environment, nothing to do
		return nil
	}

	if err := out.DeleteField("spec.environment"); err != nil {
		return errors.Wrap(err, "failed to delete empty environment")
	}

	return nil
}

// migrateMergeOptions implements the conversion of mergeOptions to the new
// toFieldPath policy. The conversion logic is described in
// https://github.com/crossplane-contrib/function-patch-and-transform/?tab=readme-ov-file#mergeoptions-replaced-by-tofieldpath.
func migrateMergeOptions(mo *commonv1.MergeOptions) *ToFieldPathPolicy {
	if mo == nil {
		// No merge options at all, default to nil which will mean Replace
		return nil
	}

	if ptr.Deref(mo.KeepMapValues, false) {
		if !ptr.Deref(mo.AppendSlice, false) {
			// { appendSlice: nil/false, keepMapValues: true}
			return ptr.To(ToFieldPathPolicyMergeObjects)
		}

		// { appendSlice: true, keepMapValues: true }
		return ptr.To(ToFieldPathPolicyMergeObjectsAppendArrays)
	}

	if ptr.Deref(mo.AppendSlice, false) {
		// { appendSlice: true, keepMapValues: nil/false }
		return ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays)
	}

	// { appendSlice: nil/false, keepMapValues: nil/false }
	return ptr.To(ToFieldPathPolicyForceMergeObjects)
}

func getMathTransformType(tt v1.Transform) v1.MathTransformType {
	switch {
	case tt.Math.Type != "":
		return tt.Math.Type
	case tt.Math.ClampMin != nil:
		return v1.MathTransformTypeClampMin
	case tt.Math.ClampMax != nil:
		return v1.MathTransformTypeClampMax
	case tt.Math.Multiply != nil:
		return v1.MathTransformTypeMultiply
	}
	return ""
}

func getStringTransformType(tt v1.Transform) v1.StringTransformType {
	switch {
	case tt.String.Type != "":
		return tt.String.Type
	case tt.String.Format != nil:
		return v1.StringTransformTypeFormat
	case tt.String.Convert != nil:
		return v1.StringTransformTypeConvert
	case tt.String.Regexp != nil:
		return v1.StringTransformTypeRegexp
	}
	return ""
}

// migratePatch will perform all migration steps required to ensure that the
// given patch is in the correct format for function-patch-and-transform:
// - default the type to FromCompositeFieldPath if it is not set
// - migrate the patch policy to the new format
// - enforce that all transforms have a type set.
func migratePatch(patch map[string]any) (*fieldpath.Paved, error) {
	p := fieldpath.Pave(patch)
	_, err := p.GetString("type")
	if fieldpath.IsNotFound(err) {
		if err := p.SetValue("type", v1.PatchTypeFromCompositeFieldPath); err != nil {
			return nil, errors.Wrap(err, "failed to set patch type")
		}
	}

	var mo *commonv1.MergeOptions
	if err := p.GetValueInto("policy.mergeOptions", &mo); err != nil && !fieldpath.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get mergeOptions")
	}

	if newPolicy := migrateMergeOptions(mo); newPolicy != nil {
		if err := p.SetValue("policy.toFieldPath", newPolicy); err != nil {
			return nil, errors.Wrap(err, "failed to set policy.toFieldPath")
		}
	}

	if err := p.DeleteField("policy.mergeOptions"); err != nil && !fieldpath.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to delete policy.mergeOptions")
	}

	var transforms []v1.Transform
	if err := p.GetValueInto("transforms", &transforms); err == nil {
		for idx, transform := range transforms {
			transforms[idx] = setTransformTypeRequiredFields(transform)
		}
		if err := p.SetValue("transforms", transforms); err != nil {
			return nil, errors.Wrap(err, "failed to set transforms")
		}
	}

	return p, nil
}

// migrateResources will perform all migration steps required to ensure that the
// given resource is in the correct format for function-patch-and-transform:
// - set a resource name, if not set
// - defaulting connection details, if needed
// - migrate all patches, if needed.
func migrateResource(resource map[string]any, idx int) (*fieldpath.Paved, error) {
	r := fieldpath.Pave(resource)

	_, err := r.GetString("name")
	if fieldpath.IsNotFound(err) {
		if err := r.SetValue("name", fmt.Sprintf("resource-%d", idx)); err != nil {
			return nil, errors.Wrap(err, "failed to set resource name")
		}
	}

	var connectionDetails []v1.ConnectionDetail
	if err := r.GetValueInto("connectionDetails", &connectionDetails); err == nil {
		for idx, cd := range connectionDetails {
			connectionDetails[idx] = setMissingConnectionDetailFields(cd)
		}
		if err := r.SetValue("connectionDetails", connectionDetails); err != nil {
			return nil, errors.Wrap(err, "failed to set connectionDetails")
		}
	}

	var patches []map[string]any
	if err := r.GetValueInto("patches", &patches); err == nil {
		for idx, patch := range patches {
			p, err := migratePatch(patch)
			if err != nil {
				return nil, errors.Wrap(err, "failed to migrate patch")
			}
			patches[idx] = p.UnstructuredContent()
		}
		if err := r.SetValue("patches", patches); err != nil {
			return nil, errors.Wrap(err, "failed to set patches")
		}
	}

	return r, nil
}

func setMissingConnectionDetailFields(sk v1.ConnectionDetail) v1.ConnectionDetail {
	// Only one of the values should be set, but we are not validating it here
	nsk := v1.ConnectionDetail{
		Name:                    sk.Name,
		Value:                   sk.Value,
		FromConnectionSecretKey: sk.FromConnectionSecretKey,
		FromFieldPath:           sk.FromFieldPath,
	}
	// Type is now required
	if nsk.Type == nil {
		switch {
		case sk.Value != nil:
			nsk.Type = ptr.To(v1.ConnectionDetailTypeFromValue)
		case sk.FromFieldPath != nil:
			nsk.Type = ptr.To(v1.ConnectionDetailTypeFromFieldPath)
		case sk.FromConnectionSecretKey != nil:
			nsk.Type = ptr.To(v1.ConnectionDetailTypeFromConnectionSecretKey)
		}
	}
	// Name is also required
	if nsk.Name == nil {
		switch { //nolint:gocritic // we could add more here in the future
		case ptr.Equal(nsk.Type, ptr.To(v1.ConnectionDetailTypeFromConnectionSecretKey)):
			nsk.Name = sk.FromConnectionSecretKey
		}
		// FromValue and FromFieldPath should have a name, skip implementation for now
	}
	return nsk
}

// setTransformTypeRequiredFields sets fields that are required with
// function-patch-and-transform but were optional with the built-in engine.
func setTransformTypeRequiredFields(tt v1.Transform) v1.Transform {
	if tt.Type == "" {
		if tt.Math != nil {
			tt.Type = v1.TransformTypeMath
		}
		if tt.String != nil {
			tt.Type = v1.TransformTypeString
		}
	}
	if tt.Type == v1.TransformTypeMath && tt.Math.Type == "" {
		tt.Math.Type = getMathTransformType(tt)
	}

	if tt.Type == v1.TransformTypeString && tt.String.Type == "" {
		tt.String.Type = getStringTransformType(tt)
	}
	return tt
}
