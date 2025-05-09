/*
Copyright 2025 The Crossplane Authors.

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

package manager

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

func TestRevisionActivator(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		act  *PackageRevisionActivator
		pkg  v1.Package
		revs []v1.PackageRevision
	}

	type want struct {
		err  error
		revs []v1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ManualActivation": {
			reason: "The activator should do nothing for packages using the manual activation policy.",
			args: args{
				act: &PackageRevisionActivator{},
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.ManualActivation,
						},
					},
					Status: v1.ConfigurationStatus{
						PackageStatus: v1.PackageStatus{
							CurrentRevision: "revision-2",
						},
					},
				},
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				err: nil,
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
		},
		"ErrApplyInactiveRevision": {
			reason: "The activator should return an error if it fails to inactivate a revision.",
			args: args{
				act: &PackageRevisionActivator{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							if o.(v1.PackageRevision).GetDesiredState() == v1.PackageRevisionInactive {
								return errBoom
							}
							return nil
						}),
					},
				},
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
					Status: v1.ConfigurationStatus{
						PackageStatus: v1.PackageStatus{
							CurrentRevision: "revision-2",
						},
					},
				},
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"ErrApplyActiveRevision": {
			reason: "The activator should return an error if it fails to activate a revision.",
			args: args{
				act: &PackageRevisionActivator{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							if o.(v1.PackageRevision).GetDesiredState() == v1.PackageRevisionActive {
								return errBoom
							}
							return nil
						}),
					},
				},
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
					Status: v1.ConfigurationStatus{
						PackageStatus: v1.PackageStatus{
							CurrentRevision: "revision-2",
						},
					},
				},
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"AutomaticActivationNoChange": {
			reason: "The activator should not make changes if the current revision is already active.",
			args: args{
				act: &PackageRevisionActivator{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							wantState := v1.PackageRevisionInactive
							if o.GetName() == "revision-2" {
								wantState = v1.PackageRevisionActive
							}
							want := &v1.ConfigurationRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: o.GetName(),
								},
								Spec: v1.PackageRevisionSpec{
									Revision:     o.(v1.PackageRevision).GetRevision(),
									DesiredState: wantState,
								},
							}

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
					Status: v1.ConfigurationStatus{
						PackageStatus: v1.PackageStatus{
							CurrentRevision: "revision-2",
						},
					},
				},
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
			want: want{
				err: nil,
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
		},
		"AutomaticActivationActivateCurrent": {
			reason: "The activator should activate the current revision when it's not already active.",
			args: args{
				act: &PackageRevisionActivator{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							wantState := v1.PackageRevisionInactive
							if o.GetName() == "revision-2" {
								wantState = v1.PackageRevisionActive
							}
							want := &v1.ConfigurationRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: o.GetName(),
								},
								Spec: v1.PackageRevisionSpec{
									Revision:     o.(v1.PackageRevision).GetRevision(),
									DesiredState: wantState,
								},
							}

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				pkg: &v1.Configuration{
					Spec: v1.ConfigurationSpec{
						PackageSpec: v1.PackageSpec{
							RevisionActivationPolicy: &v1.AutomaticActivation,
						},
					},
					Status: v1.ConfigurationStatus{
						PackageStatus: v1.PackageStatus{
							CurrentRevision: "revision-2",
						},
					},
				},
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				err: nil,
				revs: []v1.PackageRevision{
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-1",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     1,
							DesiredState: v1.PackageRevisionInactive,
						},
					},
					&v1.ConfigurationRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "revision-2",
						},
						Spec: v1.PackageRevisionSpec{
							Revision:     2,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.act.ActivateRevisions(context.Background(), tc.args.pkg, tc.args.revs)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nActivateRevisions(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.revs, got); diff != "" {
				t.Errorf("\n%s\nActivateRevisions(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
