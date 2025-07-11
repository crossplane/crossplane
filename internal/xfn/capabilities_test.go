/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestRevisionCapabilityChecker(t *testing.T) {
	type args struct {
		ctx   context.Context
		caps  []string
		names []string
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		c      client.Reader
		args   args
		want   want
	}{
		"ListError": {
			reason: "We should return an error if we can't list FunctionRevisions",
			c: &test.MockClient{
				MockList: test.NewMockListFn(errors.New("boom")),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"test-fn"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NoRevisions": {
			reason: "We should return nil if there are no revisions",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"test-fn"},
			},
			want: want{
				err: nil,
			},
		},
		"NoActiveRevisions": {
			reason: "We should return nil if there are no active revisions",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-fn-abc123",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "test-fn",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionInactive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1", "cap2"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"test-fn"},
			},
			want: want{
				err: nil,
			},
		},
		"NoMatchingPackages": {
			reason: "We should return nil if there are no matching packages",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "different-fn-abc123",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "different-fn",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1", "cap2"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"test-fn"},
			},
			want: want{
				err: nil,
			},
		},
		"HasAllCapabilities": {
			reason: "We should return nil if the function has all required capabilities",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-fn-abc123",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "test-fn",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1", "cap2", "cap3"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1", "cap2"},
				names: []string{"test-fn"},
			},
			want: want{
				err: nil,
			},
		},
		"MissingCapabilities": {
			reason: "We should return an error if the function is missing required capabilities",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-fn-abc123",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "test-fn",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1", "cap2", "cap3"},
				names: []string{"test-fn"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MissingParentPackageLabel": {
			reason: "We should skip revisions without parent package label",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-fn-abc123",
								// No parent package label
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"test-fn"},
			},
			want: want{
				err: nil,
			},
		},
		"MultiplePackages": {
			reason: "We should check capabilities for multiple packages",
			c: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					obj.(*pkgv1.FunctionRevisionList).Items = []pkgv1.FunctionRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fn1-abc123",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "fn1",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1", "cap2"},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fn2-def456",
								Labels: map[string]string{
									pkgv1.LabelParentPackage: "fn2",
								},
							},
							Spec: pkgv1.FunctionRevisionSpec{
								PackageRevisionSpec: pkgv1.PackageRevisionSpec{
									DesiredState: pkgv1.PackageRevisionActive,
								},
							},
							Status: pkgv1.FunctionRevisionStatus{
								PackageRevisionStatus: pkgv1.PackageRevisionStatus{
									Capabilities: []string{"cap1", "cap3"},
								},
							},
						},
					}
					return nil
				}),
			},
			args: args{
				ctx:   context.Background(),
				caps:  []string{"cap1"},
				names: []string{"fn1", "fn2"},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &RevisionCapabilityChecker{
				client: tc.c,
			}

			err := c.CheckCapabilities(tc.args.ctx, tc.args.caps, tc.args.names...)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCheckCapabilities(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
