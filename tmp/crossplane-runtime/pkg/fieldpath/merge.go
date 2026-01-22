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

package fieldpath

import (
	"reflect"

	"dario.cat/mergo"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

const (
	errInvalidMerge = "failed to merge values"
)

// MergeValue of the receiver p at the specified field path with the supplied
// value according to supplied merge options.
func (p *Paved) MergeValue(path string, value any, mo *MergeOptions) error {
	dst, err := p.GetValue(path)
	if IsNotFound(err) || mo == nil {
		dst = nil
	} else if err != nil {
		return err
	}

	dst, err = merge(dst, value, mo)
	if err != nil {
		return err
	}

	return p.SetValue(path, dst)
}

// merges the given src onto the given dst.
// dst and src must have the same type.
// If a nil merge options is supplied, the default behavior is MergeOptions'
// default behavior. If dst or src is nil, src is returned
// (i.e., dst replaced by src).
func merge(dst, src any, mergeOptions *MergeOptions) (any, error) {
	// because we are merging values of a field, which can be a slice, and
	// because mergo currently supports merging only maps or structs,
	// we wrap the argument to be passed to mergo.Merge in a map.
	const keyArg = "arg"

	argWrap := func(arg any) map[string]any {
		return map[string]any{
			keyArg: arg,
		}
	}

	if dst == nil || src == nil {
		return src, nil // no merge, replace
	}
	// TODO(aru): we may provide an extra MergeOption to also append duplicates of slice elements
	// but, by default, do not append duplicate slice items if MergeOptions.AppendSlice is set
	if mergeOptions.isAppendSlice() {
		src = removeSourceDuplicates(dst, src)
	}

	mDst := argWrap(dst)
	// use merge semantics with the configured merge options to obtain the target dst value
	if err := mergo.Merge(&mDst, argWrap(src), mergeOptions.mergoConfiguration()...); err != nil {
		return nil, errors.Wrap(err, errInvalidMerge)
	}

	return mDst[keyArg], nil
}

func removeSourceDuplicates(dst, src any) any {
	sliceDst, sliceSrc := reflect.ValueOf(dst), reflect.ValueOf(src)
	if sliceDst.Kind() == reflect.Ptr {
		sliceDst = sliceDst.Elem()
	}

	if sliceSrc.Kind() == reflect.Ptr {
		sliceSrc = sliceSrc.Elem()
	}

	if sliceDst.Kind() != reflect.Slice || sliceSrc.Kind() != reflect.Slice {
		return src
	}

	result := reflect.New(sliceSrc.Type()).Elem() // we will not modify src
	for i := range sliceSrc.Len() {
		itemSrc := sliceSrc.Index(i)

		found := false
		for j := 0; j < sliceDst.Len() && !found; j++ {
			// if src item is found in the dst array
			if reflect.DeepEqual(itemSrc.Interface(), sliceDst.Index(j).Interface()) {
				found = true
			}
		}

		if !found {
			// then put src item into result
			result = reflect.Append(result, itemSrc)
		}
	}

	return result.Interface()
}

// MergeOptions Specifies merge options on a field path.
type MergeOptions struct { // TODO(aru): add more options that control merging behavior
	// Specifies that already existing values in a merged map should be preserved
	// +optional
	KeepMapValues *bool `json:"keepMapValues,omitempty"`
	// Specifies that already existing elements in a merged slice should be preserved
	// +optional
	AppendSlice *bool `json:"appendSlice,omitempty"`
}

// mergoConfiguration the default behavior is to replace maps and slices.
func (mo *MergeOptions) mergoConfiguration() []func(*mergo.Config) {
	config := []func(*mergo.Config){mergo.WithOverride}
	if mo == nil {
		return config
	}

	if mo.KeepMapValues != nil && *mo.KeepMapValues {
		config = config[:0]
	}

	if mo.AppendSlice != nil && *mo.AppendSlice {
		config = append(config, mergo.WithAppendSlice)
	}

	return config
}

// isAppendSlice returns true if mo.AppendSlice is set to true.
func (mo *MergeOptions) isAppendSlice() bool {
	return mo != nil && mo.AppendSlice != nil && *mo.AppendSlice
}
