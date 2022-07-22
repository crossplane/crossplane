/*
Copyright 2021 The Crossplane Authors.

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
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// mergePath merges the value at the given field path of the src object into
// the dst object.
func mergePath(path string, dst, src runtime.Object, mergeOptions *xpv1.MergeOptions) error {
	srcPaved, err := fieldpath.PaveObject(src)
	if err != nil {
		return err
	}

	val, err := srcPaved.GetValue(path)
	// if src has no value at the specified path, then nothing to merge
	if fieldpath.IsNotFound(err) || val == nil {
		return nil
	}
	if err != nil {
		return err
	}

	return patchFieldValueToObject(path, val, dst, mergeOptions)
}

// mergeReplace merges the value at path from dst into
// a copy of src and then replaces the value at path of
// dst with the merged value. src object is not modified.
func mergeReplace(path string, src, dst runtime.Object, mo *xpv1.MergeOptions) error {
	copySrc := src.DeepCopyObject()
	if err := mergePath(path, copySrc, dst, mo); err != nil {
		return err
	}
	// replace desired object's value at fieldPath with
	// the computed (merged) current value at the same path
	return mergePath(path, dst, copySrc, nil)
}

// withMergeOptions returns an ApplyOption for merging the value at the given
// fieldPath of desired object onto the current object with
// the given merge options.
func withMergeOptions(fieldPath string, mergeOptions *xpv1.MergeOptions) resource.ApplyOption {
	return func(_ context.Context, current, desired runtime.Object) error {
		return mergeReplace(fieldPath, current, desired, mergeOptions)
	}
}

// mergeOptions returns merge options for an unfiltered patch specification
// as an array of apply options.
func mergeOptions(pas []v1.Patch) []resource.ApplyOption {
	opts := make([]resource.ApplyOption, 0, len(pas))
	for _, p := range pas {
		if p.Policy == nil || p.ToFieldPath == nil {
			continue
		}
		opts = append(opts, withMergeOptions(*p.ToFieldPath, p.Policy.MergeOptions))
	}
	return opts
}

// patchFieldValueToObject applies the value to the "to" object at the given
// path with the given merge options, returning any errors as they occur.
// If no merge options is supplied, then destination field is replaced
// with the given value.
func patchFieldValueToObject(fieldPath string, value any, to runtime.Object, mo *xpv1.MergeOptions) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}

	if err := paved.MergeValue(fieldPath, value, mo); err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}
