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
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var (
	crossplane  = "v0.11.1"
	providerDep = "crossplane/provider-aws"
	version     = "v0.1.1"
)

func TestHookPre(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		hook Hook
		pkg  runtime.Object
		rev  v1alpha1.PackageRevision
	}

	type want struct {
		err error
		rev v1alpha1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHook{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ErrNotConfiguration": {
			reason: "Should return error if not configuration.",
			args: args{
				hook: &ConfigurationHook{},
			},
			want: want{
				err: errors.New(errNotConfiguration),
			},
		},
		"ProviderActive": {
			reason: "Should only update status if provider revision is active.",
			args: args{
				hook: &ProviderHook{},
				pkg: &pkgmeta.Provider{
					Spec: pkgmeta.ProviderSpec{
						Crossplane: &crossplane,
						DependsOn: []pkgmeta.Dependency{{
							Provider: &providerDep,
							Version:  version,
						}},
					},
				},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
					Status: v1alpha1.PackageRevisionStatus{
						Crossplane: crossplane,
						DependsOn: []v1alpha1.Dependency{{
							Package: providerDep,
							Version: version,
						}},
					},
				},
			},
		},
		"Configuration": {
			reason: "Should always update status for configuration revisions.",
			args: args{
				hook: &ConfigurationHook{},
				pkg: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						Crossplane: &crossplane,
						DependsOn: []pkgmeta.Dependency{{
							Provider: &providerDep,
							Version:  version,
						}},
					},
				},
				rev: &v1alpha1.ConfigurationRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ConfigurationRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
					Status: v1alpha1.PackageRevisionStatus{
						Crossplane: crossplane,
						DependsOn: []v1alpha1.Dependency{{
							Package: providerDep,
							Version: version,
						}},
					},
				},
			},
		},
		"ErrProviderDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment for inactive provider revision.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o runtime.Object) error {
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
				pkg: &pkgmeta.Provider{
					Spec: pkgmeta.ProviderSpec{
						Crossplane: &crossplane,
						DependsOn: []pkgmeta.Dependency{{
							Provider: &providerDep,
							Version:  version,
						}},
					},
				},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
					Status: v1alpha1.PackageRevisionStatus{
						Crossplane: crossplane,
						DependsOn: []v1alpha1.Dependency{{
							Package: providerDep,
							Version: version,
						}},
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderDeployment),
			},
		},
		"ErrProviderDeleteSA": {
			reason: "Should return error if we fail to delete service account for inactive provider revision.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o runtime.Object) error {
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
				pkg: &pkgmeta.Provider{
					Spec: pkgmeta.ProviderSpec{
						Crossplane: &crossplane,
						DependsOn: []pkgmeta.Dependency{{
							Provider: &providerDep,
							Version:  version,
						}},
					},
				},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
					Status: v1alpha1.PackageRevisionStatus{
						Crossplane: crossplane,
						DependsOn: []v1alpha1.Dependency{{
							Package: providerDep,
							Version: version,
						}},
					},
				},
				err: errors.Wrap(errBoom, errDeleteProviderSA),
			},
		},
		"SuccessfulProviderDelete": {
			reason: "Should update status and not return error when deployment and service account deleted successfully.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockDelete: test.NewMockDeleteFn(nil, func(o runtime.Object) error {
								return nil
							}),
						},
					},
				},
				pkg: &pkgmeta.Provider{
					Spec: pkgmeta.ProviderSpec{
						Crossplane: &crossplane,
						DependsOn: []pkgmeta.Dependency{{
							Provider: &providerDep,
							Version:  version,
						}},
					},
				},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
					Status: v1alpha1.PackageRevisionStatus{
						Crossplane: crossplane,
						DependsOn: []v1alpha1.Dependency{{
							Package: providerDep,
							Version: version,
						}},
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
		hook Hook
		pkg  runtime.Object
		rev  v1alpha1.PackageRevision
	}

	type want struct {
		err error
		rev v1alpha1.PackageRevision
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			args: args{
				hook: &ProviderHook{},
			},
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ProviderInactive": {
			reason: "Should do nothing if provider revision is inactive.",
			args: args{
				hook: &ProviderHook{},
				pkg:  &pkgmeta.Provider{},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionInactive,
					},
				},
			},
		},
		"ErrProviderApplySA": {
			reason: "Should return error if we fail to apply service account for active providerrevision.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
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
				pkg: &pkgmeta.Provider{},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderSA),
			},
		},
		"ErrProviderApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active provider revision.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
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
				pkg: &pkgmeta.Provider{},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderDeployment),
			},
		},
		"SuccessfulProviderApply": {
			reason: "Should not return error if successfully applied service account and deployment for active provider revision.",
			args: args{
				hook: &ProviderHook{
					client: resource.ClientApplicator{
						Applicator: resource.ApplyFn(func(_ context.Context, o runtime.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
				},
				pkg: &pkgmeta.Provider{},
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
					},
				},
			},
			want: want{
				rev: &v1alpha1.ProviderRevision{
					Spec: v1alpha1.PackageRevisionSpec{
						DesiredState: v1alpha1.PackageRevisionActive,
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
