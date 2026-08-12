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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/revision"
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
				pkg: &pkgmetav1.Function{
					Spec: pkgmetav1.FunctionSpec{},
				},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(_ ...ServiceOverride) *corev1.Service {
						return &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "some-service",
								Namespace: "some-namespace",
							},
						}
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
					Status: v1.FunctionRevisionStatus{
						Endpoint: fmt.Sprintf(ServiceEndpointFmt, "some-service", "some-namespace", revision.ServicePort),
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: ptr.To("some-server-secret"),
						},
					},
				},
			},
		},
		"WaitsForTLSServerSecretName": {
			reason: "Should wait, rather than panic, when the package manager has not set the revision's TLS server secret name yet.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: incoming.Name, UID: incoming.UID},
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{DesiredState: v1.PackageRevisionActive},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(_ ...ServiceOverride) *corev1.Service {
						return &corev1.Service{ObjectMeta: metav1.ObjectMeta{
							Name:      "shared-service",
							Namespace: "some-namespace",
						}}
					},
					// The builder returns nil for the secret until the revision knows
					// what its secret is called.
					TLSServerSecretFn: func() *corev1.Secret {
						return nil
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if _, ok := obj.(*corev1.Secret); ok {
							return errors.New("should not apply a secret before we know its name")
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: incoming.Name, UID: incoming.UID},
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{DesiredState: v1.PackageRevisionActive},
					},
					Status: v1.FunctionRevisionStatus{
						Endpoint: fmt.Sprintf(ServiceEndpointFmt, "shared-service", "some-namespace", revision.ServicePort),
					},
				},
			},
		},
		"TakesControlFromOutgoingRevision": {
			reason: "Should demote the outgoing revision's owner reference in the same apply that claims the shared object.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: incoming.Name, UID: incoming.UID},
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{DesiredState: v1.PackageRevisionActive},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("server-tls"),
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(_ ...ServiceOverride) *corev1.Service {
						return &corev1.Service{ObjectMeta: metav1.ObjectMeta{
							Name:            "shared-service",
							Namespace:       "some-namespace",
							OwnerReferences: []metav1.OwnerReference{incoming},
						}}
					},
					TLSServerSecretFn: func() *corev1.Secret {
						return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
							Name:            "server-tls",
							OwnerReferences: []metav1.OwnerReference{incoming},
						}}
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == "shared-service" || key.Name == "server-tls" {
							obj.SetOwnerReferences([]metav1.OwnerReference{outgoing})
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						// The incoming revision should claim the shared objects,
						// demoting the outgoing revision in the same apply.
						if diff := cmp.Diff([]metav1.OwnerReference{incoming, demoted}, obj.GetOwnerReferences()); diff != "" {
							t.Errorf("h.Pre(...): %s: -want owner references, +got:\n%s", obj.GetName(), diff)
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{Name: incoming.Name, UID: incoming.UID},
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{DesiredState: v1.PackageRevisionActive},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSServerSecretName: ptr.To("server-tls"),
						},
					},
					Status: v1.FunctionRevisionStatus{
						Endpoint: fmt.Sprintf(ServiceEndpointFmt, "shared-service", "some-namespace", revision.ServicePort),
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSServerSecretName: ptr.To("server-tls"),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client)

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
		"FunctionInactive": {
			reason: "Should do nothing if function revision is inactive.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionInactive,
						},
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
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
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
				err: errors.Wrap(errBoom, errApplyFunctionSA),
			},
		},
		"ErrApplyDeployment": {
			reason: "Should return error if we fail to apply deployment for active function revision.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
					MockPatch: func(_ context.Context, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							if got := p.Type(); got != types.ApplyPatchType {
								t.Fatalf("expected SSA patch type, got %s", got)
							}
							po := &client.PatchOptions{}
							for _, opt := range opts {
								opt.ApplyToPatch(po)
							}
							if po.FieldManager != FieldOwnerRuntime || po.Force == nil || !*po.Force {
								t.Fatalf("expected SSA field owner and force ownership options")
							}
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
				err: errors.Wrap(errBoom, errApplyFunctionDeployment),
			},
		},
		"ErrDeploymentNoAvailableConditionYet": {
			reason: "Should return error if deployment for active function revision has no available condition yet.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
				err: errors.New(errNoAvailableConditionFunctionDeployment),
			},
		},
		"ErrUnavailableDeployment": {
			reason: "Should return error if deployment is unavailable for function revision.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
				err: errors.Errorf(errFmtUnavailableFunctionDeployment, errBoom.Error()),
			},
		},
		"Successful": {
			reason: "Should not return error if successfully applied service account and deployment for active function revision and the deployment is ready.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
			},
		},
		"SuccessWithExtraSecret": {
			reason: "Should not return error if successfully applied service account with additional secret.",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
			},
		},
		"SuccessfulWithExternallyManagedSA": {
			reason: "Should be successful without creating an SA, when the SA is managed externally",
			args: args{
				pkg: &pkgmetav1.Function{},
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
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
							if sa.GetName() == xpManagedSA {
								return kerrors.NewNotFound(corev1.Resource("serviceaccount"), xpManagedSA)
							}
						}
						return nil
					},
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						if sa, ok := obj.(*corev1.ServiceAccount); ok {
							if sa.GetName() == xpManagedSA {
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
							if sa.GetName() == xpManagedSA {
								t.Error("unexpected call to patch SA when the SA is managed externally")
							}
						}
						return nil
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					Spec: v1.FunctionRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      functionImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.FunctionRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: functionImage,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client)

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
		"ErrGetDeployment": {
			reason: "Should return error if we fail to get deployment before deleting it.",
			args: args{
				rev: &v1.FunctionRevision{},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
					MockDelete: func(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
						return errors.New("deployment should not be deleted")
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGetRuntimeDeployment), errDeleteFunctionDeployment),
				rev: &v1.FunctionRevision{},
			},
		},
		"ErrDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment.",
			args: args{
				rev: &v1.FunctionRevision{},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{}
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{Controller: ptr.To(true)}})
						return nil
					}),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errBoom
						}
						return nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteFunctionDeployment),
				rev: &v1.FunctionRevision{},
			},
		},
		"Successful": {
			reason: "Should not return error if successfully deleted service account and deployment.",
			args: args{
				rev: &v1.FunctionRevision{},
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
					TLSServerSecretFn: func() *corev1.Secret {
						return &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name: "server-tls",
							},
						}
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{Controller: ptr.To(true)}})
						return nil
					}),
					MockDelete: func(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						owners := obj.GetOwnerReferences()
						if len(owners) != 1 {
							return errors.Errorf("incorrect number of owner references on %T, expected 1 got %d", obj, len(owners))
						}
						if owners[0].Controller != nil && *owners[0].Controller {
							return errors.Errorf("%T has unexpected controlling owner reference %v", obj, owners[0])
						}

						return nil
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{},
			},
		},
		"DeploymentControlledByDifferentRevision": {
			reason: "Should not delete deployment controlled by a different package revision.",
			args: args{
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "inactive-uid",
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(_ ...ServiceAccountOverride) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "some-sa"}}
					},
					DeploymentFn: func(_ string, _ ...DeploymentOverride) *appsv1.Deployment {
						return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "some-deployment"}}
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
					TLSServerSecretFn: func() *corev1.Secret {
						return &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name: "server-tls",
							},
						}
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{UID: "active-uid", Controller: ptr.To(true)}})
						return nil
					}),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errors.New("deployment should not be deleted")
						}
						return nil
					},
					// Deactivation doesn't touch owner references. The
					// revision taking over demotes ours when it claims the
					// objects we share with it.
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.Errorf("deactivation should not have patched %s", obj.GetName())
					},
				},
			},
			want: want{
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						UID: "inactive-uid",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewFunctionHooks(tc.args.client)

			err := h.Deactivate(context.TODO(), tc.args.rev, tc.args.manifests)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Deactivate(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Deactivate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
