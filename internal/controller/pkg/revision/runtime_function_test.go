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

package revision

import (
	"context"
	"fmt"
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

	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

func TestFunctionPreHook(t *testing.T) {
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
				pkg: &pkgmetav1beta1.Function{
					Spec: pkgmetav1beta1.FunctionSpec{},
				},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(overrides ...ServiceOverride) *corev1.Service {
						return &corev1.Service{}
					},
					TLSServerSecretFn: func() *corev1.Secret {
						return &corev1.Secret{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if svc, ok := obj.(*corev1.Service); ok {
							svc.Name = "some-service"
							svc.Namespace = "some-namespace"
						}
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return nil
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
					Status: v1beta1.FunctionRevisionStatus{
						Endpoint: fmt.Sprintf(serviceEndpointFmt, "some-service", "some-namespace", servicePort),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client, xpkg.DefaultRegistry)
			err := h.Pre(context.TODO(), tc.args.pkg, tc.args.rev, tc.args.manifests)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFunctionPostHook(t *testing.T) {
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
		"ErrNotFunction": {
			reason: "Should return error if not function.",
			want: want{
				err: errors.New(errNotFunction),
			},
		},
		"FunctionInactive": {
			reason: "Should do nothing if function revision is inactive.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
		},
		"ErrApplySA": {
			reason: "Should return error if we fail to apply service account for active function revision.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return errBoom
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, "cannot patch object"), errApplyFunctionSA),
			},
		},
		"ErrApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active function revision.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				err: errors.Wrap(errors.Wrap(errBoom, "cannot patch object"), errApplyFunctionDeployment),
			},
		},
		"ErrDeploymentNoAvailableConditionYet": {
			reason: "Should return error if deployment for active function revision has no available condition yet.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return nil
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				err: errors.New(errNoAvailableConditionFunctionDeployment),
			},
		},
		"ErrUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for function revision.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
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
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				err: errors.Errorf(errFmtUnavailableFunctionDeployment, errBoom.Error()),
			},
		},
		"Successful": {
			reason: "Should not return error if successfully applied service account and deployment for active function revision and the deployment is ready.",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
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
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
		},
		"SuccessfulWithExternallyManagedSA": {
			reason: "Should be successful without creating an SA, when the SA is managed externally",
			args: args{
				pkg: &pkgmetav1beta1.Function{},
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == xpManagedSA {
								return kerrors.NewNotFound(corev1.Resource("serviceaccount"), xpManagedSA)
							}
						}
						return nil
					},
					MockCreate: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == xpManagedSA {
								t.Error("unexpected call to create SA when SA is managed externally")
							}
						}
						return nil
					},
					MockPatch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							d.Status.Conditions = []appsv1.DeploymentCondition{{
								Type:   appsv1.DeploymentAvailable,
								Status: corev1.ConditionTrue,
							}}
							return nil
						}
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == xpManagedSA {
								t.Error("unexpected call to patch SA when the SA is managed externally")
							}
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client, xpkg.DefaultRegistry)
			err := h.Post(context.TODO(), tc.args.pkg, tc.args.rev, tc.args.manifests)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Pre(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFunctionDeactivateHook(t *testing.T) {
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
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockDelete: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteFunctionDeployment),
			},
		},
		"Successful": {
			reason: "Should not return error if successfully deleted service account and deployment.",
			args: args{
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-sa",
							},
						}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverride) *appsv1.Deployment {
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
					MockDelete: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
						switch obj.(type) {
						case *corev1.ServiceAccount:
							return errors.New("service account should not be deleted during deactivation")
						case *appsv1.Deployment:
							if obj.GetName() != "some-deployment" {
								return errors.New("unexpected deployment name")
							}
							return nil
						}
						return errors.New("unexpected object type")
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client, xpkg.DefaultRegistry)
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

func TestGetFunctionImage(t *testing.T) {
	type args struct {
		functionMeta     *pkgmetav1beta1.Function
		functionRevision *v1beta1.FunctionRevision
		defaultRegistry  string
	}

	type want struct {
		err   error
		image string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoOverrideFromMeta": {
			reason: "Should use the image from the package revision and add default registry when no override is present.",
			args: args{
				functionMeta: &pkgmetav1beta1.Function{
					Spec: pkgmetav1beta1.FunctionSpec{
						Image: nil,
					},
				},
				functionRevision: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "crossplane/func-bar:v1.2.3",
						},
					},
				},
				defaultRegistry: "registry.default.io",
			},
			want: want{
				err:   nil,
				image: "registry.default.io/crossplane/func-bar:v1.2.3",
			},
		},
		"WithOverrideFromMeta": {
			reason: "Should use the override from the function meta when present and add default registry.",
			args: args{
				functionMeta: &pkgmetav1beta1.Function{
					Spec: pkgmetav1beta1.FunctionSpec{
						Image: ptr.To("crossplane/func-bar-server:v1.2.3"),
					},
				},
				functionRevision: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "crossplane/func-bar:v1.2.3",
						},
					},
				},
				defaultRegistry: "registry.default.io",
			},
			want: want{
				err:   nil,
				image: "registry.default.io/crossplane/func-bar-server:v1.2.3",
			},
		},
		"RegistrySpecified": {
			reason: "Should honor the registry as specified on the package, even if its different than the default registry.",
			args: args{
				functionMeta: &pkgmetav1beta1.Function{
					Spec: pkgmetav1beta1.FunctionSpec{
						Image: nil,
					},
				},
				functionRevision: &v1beta1.FunctionRevision{
					Spec: v1beta1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package: "registry.notdefault.io/crossplane/func-bar:v1.2.3",
						},
					},
				},
				defaultRegistry: "registry.default.io",
			},
			want: want{
				err:   nil,
				image: "registry.notdefault.io/crossplane/func-bar:v1.2.3",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			image, err := getFunctionImage(tc.args.functionMeta, tc.args.functionRevision, tc.args.defaultRegistry)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngetFunctionImage(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.image, image, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ngetFunctionImage(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
