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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/util/json"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestMergeValue(t *testing.T) {
	const (
		pathTest = "a"
		valSrc   = "e1-from-source"
		valSrc2  = "e1-from-source-2"
		valDst   = "e1-from-destination"
	)
	formatArr := func(arr []string) string {
		return fmt.Sprintf(`{"%s": ["%s"]}`, pathTest, strings.Join(arr, `", "`))
	}
	formatMap := func(val string) string {
		return fmt.Sprintf(`{"%s": {"%s": "%s"}}`, pathTest, pathTest, val)
	}
	appendArr := func(dst, src []string) []string {
		return reflect.AppendSlice(reflect.ValueOf(dst), reflect.ValueOf(src)).Interface().([]string)
	}

	arrSrc := []string{valSrc}
	fnMapSrc := func() map[string]any {
		return map[string]any{pathTest: valSrc}
	}
	arrDst := []string{valDst}
	fnMapDst := func() map[string]any {
		return map[string]any{pathTest: map[string]any{pathTest: valDst}}
	}
	valFalse, valTrue := false, true

	type fields struct {
		object map[string]any
	}
	type args struct {
		path  string
		value any
		mo    *xpv1.MergeOptions
	}
	type want struct {
		serialized string
		err        error
	}
	tests := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		"MergeArrayNoMergeOptions": {
			reason: "If no merge options are given, default is to override an array",
			fields: fields{
				object: map[string]any{
					pathTest: valDst,
				},
			},
			args: args{
				path:  pathTest,
				value: arrSrc,
			},
			want: want{
				serialized: formatArr(arrSrc),
			},
		},
		"MergeArrayNoAppend": {
			reason: "If MergeOptions.AppendSlice is false, an array should be overridden when merging",
			fields: fields{
				object: map[string]any{
					pathTest: arrDst,
				},
			},
			args: args{
				path:  pathTest,
				value: arrSrc,
				mo: &xpv1.MergeOptions{
					AppendSlice: &valFalse,
				},
			},
			want: want{
				serialized: formatArr(arrSrc),
			},
		},
		"MergeArrayAppend": {
			reason: "If MergeOptions.AppendSlice is true, dst array should be merged with the src array",
			fields: fields{
				object: map[string]any{
					pathTest: arrDst,
				},
			},
			args: args{
				path:  pathTest,
				value: arrSrc,
				mo: &xpv1.MergeOptions{
					AppendSlice: &valTrue,
				},
			},
			want: want{
				serialized: formatArr(appendArr(arrDst, arrSrc)),
			},
		},
		"MergeArrayAppendDuplicate": {
			reason: "If MergeOptions.AppendSlice is true, dst array should be merged with the src array not allowing duplicates",
			fields: fields{
				object: map[string]any{
					pathTest: []string{valDst, valSrc},
				},
			},
			args: args{
				path:  pathTest,
				value: []string{valSrc, valSrc2},
				mo: &xpv1.MergeOptions{
					AppendSlice: &valTrue,
				},
			},
			want: want{
				serialized: formatArr([]string{valDst, valSrc, valSrc2}),
			},
		},
		"MergeMapNoMergeOptions": {
			reason: "If no merge options are given, default is to override a map key",
			fields: fields{
				object: fnMapDst(),
			},
			args: args{
				path:  pathTest,
				value: fnMapSrc(),
			},
			want: want{
				serialized: formatMap(valSrc),
			},
		},
		"MergeMapNoKeep": {
			reason: "If MergeOptions.KeepMapValues is false, a map key should be overridden",
			fields: fields{
				object: fnMapDst(),
			},
			args: args{
				path:  pathTest,
				value: fnMapSrc(),
				mo: &xpv1.MergeOptions{
					KeepMapValues: &valFalse,
				},
			},
			want: want{
				serialized: formatMap(valSrc),
			},
		},
		"MergeMapKeep": {
			reason: "If MergeOptions.KeepMapValues is true, a dst map key should preserve its value",
			fields: fields{
				object: fnMapDst(),
			},
			args: args{
				path:  pathTest,
				value: fnMapSrc(),
				mo: &xpv1.MergeOptions{
					KeepMapValues: &valTrue,
				},
			},
			want: want{
				serialized: formatMap(valDst),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			want := make(map[string]any)
			if err := json.Unmarshal([]byte(tc.want.serialized), &want); err != nil {
				t.Fatalf("Test case error: Unable to unmarshall JSON doc: %v", err)
			}

			p := &Paved{
				object: tc.fields.object,
			}
			err := p.MergeValue(tc.args.path, tc.args.value, tc.args.mo)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\np.MergeValue(%s, %v): %s: -want error, +got error:\n%s",
					tc.args.path, tc.args.value, tc.reason, diff)
			}
			if diff := cmp.Diff(want, p.object); diff != "" {
				t.Fatalf("\np.MergeValue(%s, %v): %s: -want, +got:\n%s",
					tc.args.path, tc.args.value, tc.reason, diff)
			}
		})
	}
}
