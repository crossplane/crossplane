/*
Copyright 2023 The Crossplane Authors.

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

package runtime

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	xpManagedSA = "xp-managed-sa"
)

var errBoom = errors.New("boom")

func TestProviderPreHook(t *testing.T) {
	type args struct {
		client    client.Client
		pkg       runtime.Object
		rev       v1.PackageRevisionWithRuntime
		manifests ManifestBuilder
	}

	type want struct {
		err error
		rev v1.PackageRevisionWithRuntime
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Successful run of pre hook.",
			args: args{
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSClientSecretName: ptr.To("some-client-secret"),
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(_ ...ServiceOverride) *corev1.Service {
						return &corev1.Service{}
					},
					TLSClientSecretFn: func() *corev1.Secret {
						return &corev1.Secret{}
					},
					TLSServerSecretFn: func() *corev1.Secret {
						return &corev1.Secret{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return nil
					},
					MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSClientSecretName: ptr.To("some-client-secret"),
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, xpkg.DefaultRegistry)
			err := h.Pre(context.TODO(), tc.args.rev, tc.args.manifests)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestProviderPostHook(t *testing.T) {
	type args struct {
		client    client.Client
		pkg       runtime.Object
		rev       v1.PackageRevisionWithRuntime
		manifests ManifestBuilder
	}

	type want struct {
		err error
		rev v1.PackageRevisionWithRuntime
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderInactive": {
			reason: "Should do nothing if provider revision is inactive.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
		},
		"ErrApplySA": {
			reason: "Should return error if we fail to apply service account for active provider revision.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errBoom
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, "cannot patch object"), errApplyProviderSA),
			},
		},
		"ErrApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active provider revision.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, "cannot patch object"), errApplyProviderDeployment),
			},
		},
		"ErrDeploymentNoAvailableConditionYet": {
			reason: "Should return error if deployment for active provider revision has no available condition yet.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				err: errors.New(errNoAvailableConditionProviderDeployment),
			},
		},
		"ErrUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for provider revision.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:    appsv1.DeploymentAvailable,
								Status:  corev1.ConditionFalse,
								Message: errBoom.Error(),
							}}
							return nil
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				err: errors.Errorf(errFmtUnavailableProviderDeployment, errBoom.Error()),
			},
		},
		"Successful": {
			reason: "Should not return error if successfully applied service account and deployment for active provider revision and the deployment is ready.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							}}
							return nil
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
			},
		},
		"SuccessWithExtraSecret": {
			reason: "Should not return error if successfully applied service account with additional secret.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							sa.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "test_secret"}}
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							}}
							return nil
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
			},
		},
		"SuccessfulWithExternallyManagedSA": {
			reason: "Should be successful without creating an SA, when the SA is managed externally",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{
							ObjectMeta: metav1.ObjectMeta{
								Name: "xp-managed-sa",
							},
						}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{
							Spec: appsv1.DeploymentSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										ServiceAccountName: "external-sa",
									},
								},
							},
						}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == "xp-managed-sa" {
								return kerrors.NewNotFound(corev1.Resource("serviceaccount"), "xp-managed-sa")
							}
						}
						return nil
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == "xp-managed-sa" {
								t.Error("unexpected call to create SA when SA is managed externally")
							}
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							}}
							return nil
						}
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == "xp-managed-sa" {
								t.Error("unexpected call to patch SA when the SA is managed externally")
							}
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.PackageRevisionStatus{
						ResolvedPackage: providerImage,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, xpkg.DefaultRegistry)
			err := h.Post(context.TODO(), tc.args.rev, tc.args.manifests)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestProviderDeactivateHook(t *testing.T) {
	type args struct {
		client    client.Client
		rev       v1.PackageRevisionWithRuntime
		manifests ManifestBuilder
	}

	type want struct {
		err error
		rev v1.PackageRevisionWithRuntime
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment.",
			args: args{
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteProviderDeployment),
			},
		},
		"Successful": {
			reason: "Should not return error if successfully deleted service account and deployment.",
			args: args{
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-sa",
							},
						}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-deployment",
							},
						}
					},
					ServiceFn: func(overrides ...ServiceOverride) *corev1.Service {
						s := &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-service",
							},
						}
						for _, o := range overrides {
							o(s)
						}
						return s
					},
				},
				client: &test.MockClient{
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						switch obj.(type) {
						case *corev1.ServiceAccount:
							return errors.New("service account should not be deleted during deactivation")
						case *appsv1.Deployment:
							if obj.GetName() != "some-deployment" {
								return errors.New("unexpected deployment name")
							}
							return nil
						case *corev1.Service:
							// Service name should be overridden
							if obj.GetName() != "some-name" {
								return errors.New("unexpected service name")
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, xpkg.DefaultRegistry)
			err := h.Deactivate(context.TODO(), tc.args.rev, tc.args.manifests)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
