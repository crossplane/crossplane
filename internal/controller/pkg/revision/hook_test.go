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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

var (
	crossplane  = "v0.11.1"
	providerDep = "crossplane/provider-aws"
	versionDep  = "v0.1.1"
)

func TestHookPre(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		hook Hooks
		pkg  runtime.Object
		rev  v1.PackageRevision
	}

	type want struct {
		err error
		rev v1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHooks{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ErrNotProviderRevision": {
			reason: "Should return error if the supplied package revision is not a provider revision.",
			args: args{
				hook: &ProviderHooks{},
				pkg:  &pkgmetav1.Provider{},
			},
			want: want{
				err: errors.New(errNotProviderRevision),
			},
		},
		"ProviderActive": {
			reason: "Should only update status if provider revision is active.",
			args: args{
				hook: &ProviderHooks{},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		"Configuration": {
			reason: "Should always update status for configuration revisions.",
			args: args{
				hook: &ConfigurationHooks{},
				pkg: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
		"ErrProviderDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment for inactive provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return errBoom
								case *corev1.ServiceAccount:
									return nil
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderDeployment),
			},
		},
		"ErrProviderDeleteSA": {
			reason: "Should return error if we fail to delete service account for inactive provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								switch o.(type) {
								case *appsv1.Deployment:
									return nil
								case *corev1.ServiceAccount:
									return errBoom
								}
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderSA),
			},
		},
		"SuccessfulProviderDelete": {
			reason: "Should update status and not return error when deployment and service account deleted successfully.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o client.Object) error {
								return nil
							}),
						},
					},
				},
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: crossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: &providerDep,
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.hook.Pre(context.TODO(), tc.args.pkg, tc.args.rev)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestHookPost(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		hook Hooks
		pkg  runtime.Object
		rev  v1.PackageRevision
	}

	type want struct {
		err error
		rev v1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHooks{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ProviderInactive": {
			reason: "Should do nothing if provider revision is inactive.",
			args: args{
				hook: &ProviderHooks{},
				pkg:  &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
		},
		"ErrProviderApplySA": {
			reason: "Should return error if we fail to apply service account for active providerrevision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return nil
							case *corev1.ServiceAccount:
								return errBoom
							}
							return nil
						}),
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderSA),
			},
		},
		"ErrProviderApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							switch o.(type) {
							case *appsv1.Deployment:
								return errBoom
							case *corev1.ServiceAccount:
								return nil
							}
							return nil
						}),
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderDeployment),
			},
		},
		"ErrProviderUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							d, ok := o.(*appsv1.Deployment)
							if !ok {
								return nil
							}
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Message: errBoom.Error(),
							}}
							return nil
						}),
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
				err: errors.Errorf("%s: %s", errUnavailableProviderDeployment, errBoom.Error()),
			},
		},
		"SuccessfulProviderApply": {
			reason: "Should not return error if successfully applied service account and deployment for active provider revision.",
			args: args{
				hook: &ProviderHooks{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
				},
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.hook.Post(context.TODO(), tc.args.pkg, tc.args.rev)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Post(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Post(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
