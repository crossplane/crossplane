/*
Copyright 2026 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

func TestFunctionRevisionForStep(t *testing.T) {
	errBoom := errors.New("boom")

	const digest = "sha256:c0ffee1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	fn := &pkgv1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: "function-cool",
			UID:  "function-cool-uid",
		},
	}

	controlledBy := func(f *pkgv1.Function) []metav1.OwnerReference {
		return []metav1.OwnerReference{{
			APIVersion: pkgv1.FunctionGroupVersionKind.GroupVersion().String(),
			Kind:       pkgv1.FunctionKind,
			Name:       f.GetName(),
			UID:        f.GetUID(),
			Controller: new(true),
		}}
	}

	revision := func(name string, state pkgv1.PackageRevisionDesiredState, pkg string, rev int64, owners []metav1.OwnerReference) pkgv1.FunctionRevision {
		return pkgv1.FunctionRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Labels:          map[string]string{pkgv1.LabelParentPackage: "function-cool"},
				OwnerReferences: owners,
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					DesiredState: state,
					Package:      pkg,
					Revision:     rev,
				},
			},
		}
	}

	getFunction := test.NewMockGetFn(nil, func(obj client.Object) error {
		fn.DeepCopyInto(obj.(*pkgv1.Function))
		return nil
	})

	listRevisions := func(revs ...pkgv1.FunctionRevision) test.MockListFn {
		return test.NewMockListFn(nil, func(obj client.ObjectList) error {
			obj.(*pkgv1.FunctionRevisionList).Items = revs
			return nil
		})
	}

	type args struct {
		c client.Reader
		s v1.PipelineStep
	}

	type want struct {
		rev string
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FunctionRefGetFunctionError": {
			reason: "We should return an error if we can't get the referenced Function.",
			args: args{
				c: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				s: v1.PipelineStep{FunctionRef: v1.FunctionReference{Name: "function-cool"}},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetFunction),
			},
		},
		"FunctionRefListError": {
			reason: "We should return an error if we can't list FunctionRevisions.",
			args: args{
				c: &test.MockClient{
					MockGet:  getFunction,
					MockList: test.NewMockListFn(errBoom),
				},
				s: v1.PipelineStep{FunctionRef: v1.FunctionReference{Name: "function-cool"}},
			},
			want: want{
				err: errors.Wrap(errBoom, errListFunctionRevisions),
			},
		},
		"FunctionRefNoActiveControlledRevision": {
			reason: "We should return an error if the Function has no active revision that it controls, even if an uncontrolled revision is active.",
			args: args{
				c: &test.MockClient{
					MockGet: getFunction,
					MockList: listRevisions(
						revision("function-cool-inactive", pkgv1.PackageRevisionInactive, "xpkg.crossplane.io/example/function-cool:v1.0.0", 1, controlledBy(fn)),
						revision("function-cool-uncontrolled", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool@"+digest, 1, nil),
					),
				},
				s: v1.PipelineStep{FunctionRef: v1.FunctionReference{Name: "function-cool"}},
			},
			want: want{
				err: errors.Errorf(errFmtNoActiveFunctionRevision, "function-cool"),
			},
		},
		"FunctionRefActiveControlledRevision": {
			reason: "We should return the Function's active controlled revision, ignoring uncontrolled revisions and revisions controlled by something else.",
			args: args{
				c: &test.MockClient{
					MockGet: getFunction,
					MockList: listRevisions(
						revision("function-cool-uncontrolled", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool@"+digest, 5, nil),
						revision("function-cool-other", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool:v0.9.0", 4, controlledBy(&pkgv1.Function{ObjectMeta: metav1.ObjectMeta{Name: "function-cool", UID: "some-other-uid"}})),
						revision("function-cool-old", pkgv1.PackageRevisionInactive, "xpkg.crossplane.io/example/function-cool:v0.8.0", 2, controlledBy(fn)),
						revision("function-cool-current", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool:v1.0.0", 3, controlledBy(fn)),
					),
				},
				s: v1.PipelineStep{FunctionRef: v1.FunctionReference{Name: "function-cool"}},
			},
			want: want{
				rev: "function-cool-current",
			},
		},
		"FunctionRefMultipleActiveControlledRevisions": {
			reason: "We should deterministically return the newest active controlled revision if there's more than one.",
			args: args{
				c: &test.MockClient{
					MockGet: getFunction,
					MockList: listRevisions(
						revision("function-cool-new", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool:v1.0.0", 3, controlledBy(fn)),
						revision("function-cool-old", pkgv1.PackageRevisionActive, "xpkg.crossplane.io/example/function-cool:v0.9.0", 2, controlledBy(fn)),
					),
				},
				s: v1.PipelineStep{FunctionRef: v1.FunctionReference{Name: "function-cool"}},
			},
			want: want{
				rev: "function-cool-new",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rev, err := FunctionRevisionForStep(context.Background(), tc.args.c, tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFunctionRevisionForStep(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rev, rev); diff != "" {
				t.Errorf("\n%s\nFunctionRevisionForStep(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
