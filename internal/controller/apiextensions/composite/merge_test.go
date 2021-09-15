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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	k8s "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

type strObject string

func (strObject) GetObjectKind() schema.ObjectKind {
	return nil
}

func (strObject) DeepCopyObject() k8s.Object {
	return nil
}

type object struct {
	P1 *p1
}
type p1 struct {
	P2 *p2
	S  *string
}
type p2 struct {
	S *string
	B *bool
	A []string
}

func (object) GetObjectKind() schema.ObjectKind {
	return nil
}

func (object) DeepCopyObject() k8s.Object {
	return nil
}

var (
	valStringDst   = "value-from-dst"
	valStringSrc   = "value-from-src"
	valBoolTrue    = true
	valBoolFalse   = false
	valArrDst      = []string{valStringDst}
	valArrSrc      = []string{valStringSrc}
	valArrAppended = []string{valStringDst, valStringSrc}
	valTrue        = true
)

func dstObject() *object {
	return &object{
		P1: &p1{
			S: &valStringDst,
			P2: &p2{
				S: &valStringDst,
				B: &valBoolTrue,
				A: valArrDst,
			},
		},
	}
}

func srcObject() *object {
	return &object{
		P1: &p1{
			S: &valStringSrc,
			P2: &p2{
				S: &valStringSrc,
				B: &valBoolFalse,
				A: valArrSrc,
			},
		},
	}
}

func TestMergePath(t *testing.T) {
	type args struct {
		fieldPath    string
		dst          k8s.Object
		src          k8s.Object
		mergeOptions *xpv1.MergeOptions
	}
	type want struct {
		err error
		dst k8s.Object
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ReplacePath": {
			reason: "Default behavior if no merge options are supplied is to replace dst with src",
			args: args{
				fieldPath:    "p1.p2",
				dst:          dstObject(),
				src:          srcObject(),
				mergeOptions: nil,
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringSrc,
							B: &valBoolFalse,
							A: valArrSrc,
						},
					},
				},
			},
		},
		"MergePathNoSliceAppend": {
			reason: "When KeepMapValues is set but AppendSlice is not, dst should preserve its values at the merge path",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src:       srcObject(),
				mergeOptions: &xpv1.MergeOptions{
					KeepMapValues: &valTrue,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringDst,
							B: &valBoolTrue,
							A: valArrDst,
						},
					},
				},
			},
		},
		"MergePathWithSliceAppend": {
			reason: "When both KeepMapValues and AppendSlice are ser, dst should preserve map values but arrays being appended",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src:       srcObject(),
				mergeOptions: &xpv1.MergeOptions{
					KeepMapValues: &valTrue,
					AppendSlice:   &valTrue,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringDst,
							B: &valBoolTrue,
							A: valArrAppended,
						},
					},
				},
			},
		},
		"PathNotFound": {
			reason: "If specified merge path does not exist, dst should be unmodified even if replace is requested (empty src value is merged onto dst value)",
			args: args{
				fieldPath: "p1.non.existent",
				dst:       dstObject(),
				src:       srcObject(),
			},
			want: want{
				dst: dstObject(),
			},
		},
		"SrcValueEmpty": {
			reason: "If value at the specified merge path is zero in src, dst should be unmodified, even if replace is requested",
			args: args{
				fieldPath: "p1.p2",
				dst:       dstObject(),
				src: &object{
					P1: &p1{
						S: &valStringSrc,
					},
				},
			},
			want: want{
				dst: dstObject(),
			},
		},
		"DstValueEmpty": {
			reason: "If value at the specified merge path is zero in dst but not in src, should be identical to a replace, even if merge is configured",
			args: args{
				fieldPath: "p1.p2",
				src:       srcObject(),
				dst: &object{
					P1: &p1{
						S: &valStringDst,
					},
				},
				mergeOptions: &xpv1.MergeOptions{
					KeepMapValues: &valTrue,
					AppendSlice:   &valTrue,
				},
			},
			want: want{
				dst: &object{
					P1: &p1{
						S: &valStringDst,
						P2: &p2{
							S: &valStringSrc,
							B: &valBoolFalse,
							A: valArrSrc,
						},
					},
				},
			},
		},
		"ErrSrcNotPaved": {
			reason: "If src cannot be paved, MergePath should be failing",
			args: args{
				dst: dstObject(),
				src: strObject("src"),
			},
			want: want{
				err: errors.Wrap(errors.New(
					"ToUnstructured requires a non-nil pointer to an object, got composite.strObject"),
					"cannot convert object to unstructured data"),
			},
		},
		"ErrDstNotPaved": {
			reason: "If dst cannot be paved, MergePath should be failing",
			args: args{
				fieldPath: "p1.p2",
				dst:       strObject("dst"),
				src:       srcObject(),
			},
			want: want{
				err: errors.Wrap(errors.New(
					"ToUnstructured requires a non-nil pointer to an object, got composite.strObject"),
					"cannot convert object to unstructured data"),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := mergePath(tc.args.fieldPath, tc.args.dst, tc.args.src, tc.args.mergeOptions)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nMergePath(...) unexpected error: %s: -want error, +got error:\n%s", tc.reason, diff)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.want.dst, tc.args.dst); diff != "" {
				t.Errorf("\nMergePath(...) unexpected dst: %s: -want dst, +got dst:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMergeReplace(t *testing.T) {
	type args struct {
		fieldPath    string
		current      k8s.Object
		desired      k8s.Object
		mergeOptions *xpv1.MergeOptions
	}
	type want struct {
		current k8s.Object
		desired k8s.Object
		err     error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"HappyPath": {
			args: args{
				fieldPath: "data",
				current: &corev1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
					},
				},
				desired: &corev1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-desired",
						"key2": "value-from-desired",
					},
				},
				mergeOptions: &xpv1.MergeOptions{
					KeepMapValues: &valTrue,
				},
			},
			want: want{
				current: &corev1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
					},
				},
				desired: &corev1.ConfigMap{
					Data: map[string]string{
						"key1": "value-from-current",
						"key2": "value-from-desired",
					},
				},
			},
		},
		"ErrFromMergePath": {
			args: args{
				fieldPath: "data",
				current:   strObject("current"),
				desired:   strObject("desired"),
			},
			want: want{
				err: errors.Wrap(errors.New(
					"ToUnstructured requires a non-nil pointer to an object, got composite.strObject"),
					"cannot convert object to unstructured data"),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := mergeReplace(tc.args.fieldPath, tc.args.current, tc.args.desired, tc.args.mergeOptions)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Fatalf("\nMergeReplace(...) unexpected error: -want error, +got error:\n%s", diff)
			}
			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.want.current, tc.args.current); diff != "" {
				t.Errorf("\nMergeReplace(...) unexpected current: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.desired, tc.args.desired); diff != "" {
				t.Errorf("\nMergeReplace(...) unexpected desired: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMergeOptions(t *testing.T) {
	testPath := "test.path"
	type args struct {
		patches []v1.Patch
	}
	tests := map[string]struct {
		args       args
		wantLength int
	}{
		"PatchNoPolicy": {
			args: args{
				patches: []v1.Patch{
					{
						ToFieldPath: &testPath,
					},
				},
			},
		},
		"PatchNoToFieldPath": {
			args: args{
				patches: []v1.Patch{
					{
						Policy: &v1.PatchPolicy{},
					},
				},
			},
		},
		"PatchEmptyPolicy": {
			args: args{
				patches: []v1.Patch{
					{
						ToFieldPath: &testPath,
						Policy:      &v1.PatchPolicy{},
					},
				},
			},
			wantLength: 1,
		},
		"TwoPatches": {
			args: args{
				patches: []v1.Patch{
					{
						ToFieldPath: &testPath,
						Policy:      &v1.PatchPolicy{},
					},
					{
						ToFieldPath: &testPath,
						Policy:      &v1.PatchPolicy{},
					},
				},
			},
			wantLength: 2,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := mergeOptions(tc.args.patches); len(got) != tc.wantLength {
				t.Errorf("mergeOptions(...): want length %v, got length %v", tc.wantLength, len(got))
			}
		})
	}
}
