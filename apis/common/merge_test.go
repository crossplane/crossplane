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

package common

import (
	"reflect"
	"runtime"
	"sort"
	"testing"

	"dario.cat/mergo"
	"github.com/google/go-cmp/cmp"
)

type mergoOptArr []func(*mergo.Config)

func (arr mergoOptArr) names() []string {
	names := make([]string, len(arr))
	for i, opt := range arr {
		names[i] = runtime.FuncForPC(reflect.ValueOf(opt).Pointer()).Name()
	}

	sort.Strings(names)

	return names
}

func TestMergoConfiguration(t *testing.T) {
	valTrue := true

	tests := map[string]struct {
		mo   *MergeOptions
		want mergoOptArr
	}{
		"DefaultOptionsNil": {
			want: mergoOptArr{
				mergo.WithOverride,
			},
		},
		"DefaultOptionsEmptyStruct": {
			mo: &MergeOptions{},
			want: mergoOptArr{
				mergo.WithOverride,
			},
		},
		"MapKeepOnly": {
			mo: &MergeOptions{
				KeepMapValues: &valTrue,
			},
			want: mergoOptArr{},
		},
		"AppendSliceOnly": {
			mo: &MergeOptions{
				AppendSlice: &valTrue,
			},
			want: mergoOptArr{
				mergo.WithAppendSlice,
				mergo.WithOverride,
			},
		},
		"MapKeepAppendSlice": {
			mo: &MergeOptions{
				AppendSlice:   &valTrue,
				KeepMapValues: &valTrue,
			},
			want: mergoOptArr{
				mergo.WithAppendSlice,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want.names(), mergoOptArr(tc.mo.MergoConfiguration()).names()); diff != "" {
				t.Errorf("\nmo.MergoConfiguration(): -want, +got:\n %s", diff)
			}
		})
	}
}
