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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

const (
	xpManagedSA = "xp-managed-sa"
)

var errBoom = errors.New("boom")

func TestProviderPreHook(t *testing.T) {
	type args struct {
		client   client.Client
		pkg      runtime.Object
		rev      v1.PackageRevisionWithRuntime
		migrator DeploymentSelectorMigrator
	}

	type want struct {
		err error
		rev v1.PackageRevisionWithRuntime
	}

	// A test provider revision that shares three objects with its package's
	// other revisions.
	sharedRev := &v1.ProviderRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:   incoming.Name,
			UID:    incoming.UID,
			Labels: map[string]string{v1.LabelParentPackage: "shared-service"},
		},
		Spec: v1.ProviderRevisionSpec{
			PackageRevisionSpec: v1.PackageRevisionSpec{DesiredState: v1.PackageRevisionActive},
			PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
				TLSClientSecretName: new("client-tls"),
				TLSServerSecretName: new("server-tls"),
			},
		},
	}
	sharedRevSynced := sharedRev.DeepCopy()
	sharedRevSynced.Status.TLSClientSecretName = new("client-tls")
	sharedRevSynced.Status.TLSServerSecretName = new("server-tls")

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
							TLSClientSecretName: new("some-client-secret"),
							TLSServerSecretName: new("some-server-secret"),
						},
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
							TLSClientSecretName: new("some-client-secret"),
							TLSServerSecretName: new("some-server-secret"),
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionRuntimeStatus: v1.PackageRevisionRuntimeStatus{
							TLSClientSecretName: new("some-client-secret"),
							TLSServerSecretName: new("some-server-secret"),
						},
					},
				},
			},
		},
		"TakesControlFromOutgoingRevision": {
			reason: "Should demote the outgoing revision's owner reference in the same apply that claims the shared object.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: sharedRev.DeepCopy(),
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == "shared-service" || key.Name == "client-tls" || key.Name == "server-tls" {
							obj.SetOwnerReferences([]metav1.OwnerReference{outgoing})
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if diff := cmp.Diff([]metav1.OwnerReference{incoming, demoted}, obj.GetOwnerReferences()); diff != "" {
							t.Errorf("h.Pre(...): %s: -want owner references, +got:\n%s", obj.GetName(), diff)
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			want: want{rev: sharedRevSynced.DeepCopy()},
		},
		"DropsOwnerReferencesThatNoLongerControl": {
			reason: "Should stop declaring an owner reference once it is non-controlling, so that server-side apply prunes it.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: sharedRev.DeepCopy(),
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == "shared-service" || key.Name == "client-tls" || key.Name == "server-tls" {
							obj.SetOwnerReferences([]metav1.OwnerReference{demoted, incoming})
						}
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if diff := cmp.Diff([]metav1.OwnerReference{incoming}, obj.GetOwnerReferences()); diff != "" {
							t.Errorf("h.Pre(...): %s: -want owner references, +got:\n%s", obj.GetName(), diff)
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			want: want{rev: sharedRevSynced.DeepCopy()},
		},
		"CreatesSharedObjectsThatDoNotExist": {
			reason: "Should apply the shared objects with only our owner reference when there is no incumbent to displace.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: sharedRev.DeepCopy(),
				client: &test.MockClient{
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					MockCreate: test.NewMockCreateFn(nil),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if diff := cmp.Diff([]metav1.OwnerReference{incoming}, obj.GetOwnerReferences()); diff != "" {
							t.Errorf("h.Pre(...): %s: -want owner references, +got:\n%s", obj.GetName(), diff)
						}
						return nil
					},
				},
			},
			want: want{rev: sharedRevSynced.DeepCopy()},
		},
		"ErrGetSharedObject": {
			reason: "Should return an error if we cannot read the shared object to see who controls it.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: sharedRev.DeepCopy(),
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.Errorf("%s should not be applied when we can't tell who controls it", obj.GetName())
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGetSharedRuntimeObject), errApplyProviderService),
				rev: sharedRevSynced.DeepCopy(),
			},
		},
		"MigratorError": {
			reason: "Pre should return an error, and leave the revision untouched, if the deployment selector migration fails.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSClientSecretName: new("some-client-secret"),
							TLSServerSecretName: new("some-server-secret"),
						},
					},
				},
				client: &test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.Errorf("%s should not be applied when the migration fails", obj.GetName())
					},
				},
				migrator: &MockDeploymentSelectorMigrator{
					MockMigrateDeploymentSelector: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *appsv1.Deployment) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errMigrateProviderDeployment),
				// The migration runs before we record the observed TLS secret
				// names, so a failure must not leave them set: the reconciler
				// writes the revision's status on the error path.
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSClientSecretName: new("some-client-secret"),
							TLSServerSecretName: new("some-server-secret"),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, namespace, crossplaneName, tc.args.migrator)

			err := h.Pre(context.TODO(), tc.args.rev, nil)
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
		client client.Client
		pkg    runtime.Object
		rev    v1.PackageRevisionWithRuntime
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderSA),
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
					},
				},
				err: errors.Wrap(errBoom, errApplyProviderDeployment),
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ConditionedStatus: xpv2.ConditionedStatus{Conditions: []xpv2.Condition{v1.RuntimeHealthy(), v1.RuntimeActive()}},
							ResolvedPackage:   providerImage,
						},
					},
				},
			},
		},
		"SuccessfulScaledToZero": {
			reason: "Should not return error for a deployment scaled to zero replicas, which Kubernetes marks as available.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							// A deployment with zero replicas has a minimum
							// availability of zero, so Kubernetes marks it
							// available.
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ConditionedStatus: xpv2.ConditionedStatus{Conditions: []xpv2.Condition{v1.RuntimeHealthy(), v1.RuntimeActive()}},
							ResolvedPackage:   providerImage,
						},
					},
				},
			},
		},
		"SuccessfulAwaitingActivation": {
			reason: "Should mark the runtime awaiting activation when a safe-start revision owns only inactive MRDs.",
			args: args{
				pkg: &pkgmetav1.Provider{},
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{UID: "owner-uid"},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
							Capabilities:    []string{pkgmetav1.ProviderCapabilitySafeStart},
						},
					},
				},
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
						return nil
					},
					MockList: test.NewMockListFn(nil, func(l client.ObjectList) error {
						l.(*extv1alpha1.ManagedResourceDefinitionList).Items = []extv1alpha1.ManagedResourceDefinition{{
							ObjectMeta: metav1.ObjectMeta{
								Name:            "ours",
								OwnerReferences: []metav1.OwnerReference{{UID: "owner-uid", Controller: new(true)}},
							},
							Spec: extv1alpha1.ManagedResourceDefinitionSpec{State: extv1alpha1.ManagedResourceDefinitionInactive},
						}}
						return nil
					}),
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						if d, ok := obj.(*appsv1.Deployment); ok {
							if d.Spec.Replicas == nil || *d.Spec.Replicas != 0 {
								t.Error("deployment awaiting activation should be scaled to zero")
							}
							// A deployment with zero replicas has a minimum
							// availability of zero, so Kubernetes marks it
							// available.
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
					ObjectMeta: metav1.ObjectMeta{UID: "owner-uid"},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      providerImage,
							DesiredState: v1.PackageRevisionActive,
						},
					},
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ConditionedStatus: xpv2.ConditionedStatus{Conditions: []xpv2.Condition{v1.RuntimeHealthy(), v1.RuntimeAwaitingActivation().WithMessage(msgAwaitingActivation)}},
							ResolvedPackage:   providerImage,
							Capabilities:      []string{pkgmetav1.ProviderCapabilitySafeStart},
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ConditionedStatus: xpv2.ConditionedStatus{Conditions: []xpv2.Condition{v1.RuntimeHealthy(), v1.RuntimeActive()}},
							ResolvedPackage:   providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ResolvedPackage: providerImage,
						},
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
					Status: v1.ProviderRevisionStatus{
						PackageRevisionStatus: v1.PackageRevisionStatus{
							ConditionedStatus: xpv2.ConditionedStatus{Conditions: []xpv2.Condition{v1.RuntimeHealthy(), v1.RuntimeActive()}},
							ResolvedPackage:   providerImage,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, namespace, crossplaneName, NewNopDeploymentSelectorMigrator())

			err := h.Post(context.TODO(), tc.args.rev, nil)
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
		client client.Client
		rev    v1.PackageRevisionWithRuntime
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
				rev: &v1.ProviderRevision{},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{Controller: new(true)}})
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
				err: errors.Wrap(errBoom, errDeleteProviderDeployment),
				rev: &v1.ProviderRevision{},
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{Controller: new(true)}})
						return nil
					}),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						switch obj.(type) {
						case *corev1.ServiceAccount:
							return errors.New("service account should not be deleted during deactivation")
						case *appsv1.Deployment:
							if obj.GetName() != "some-name" {
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
					// Deactivation doesn't touch owner references. The revision taking over demotes
					// ours when it claims the objects we share with it.
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.Errorf("deactivation should not have patched %s", obj.GetName())
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
		"DeploymentControlledByDifferentRevision": {
			reason: "Should not delete deployment controlled by a different package revision.",
			args: args{
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
						UID:  "inactive-uid",
					},
				},
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetOwnerReferences([]metav1.OwnerReference{{UID: "active-uid", Controller: new(true)}})
						return nil
					}),
					MockDelete: func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
						if _, ok := obj.(*appsv1.Deployment); ok {
							return errors.New("deployment should not be deleted")
						}
						return nil
					},
					// Deactivation doesn't touch owner references. The revision taking over demotes
					// ours when it claims the objects we share with it.
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.Errorf("deactivation should not have patched %s", obj.GetName())
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-name",
						UID:  "inactive-uid",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client, namespace, crossplaneName, NewNopDeploymentSelectorMigrator())

			err := h.Deactivate(context.TODO(), tc.args.rev, nil)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Deactivate(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rev, tc.args.rev, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Deactivate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestOwnedMRDs(t *testing.T) {
	errBoom := errors.New("boom")

	owner := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{UID: "owner-uid"}}
	owner.SetCapabilities([]string{pkgmetav1.ProviderCapabilitySafeStart})

	mrd := func(name string, controller types.UID) extv1alpha1.ManagedResourceDefinition {
		return extv1alpha1.ManagedResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: []metav1.OwnerReference{{UID: controller, Controller: new(true)}},
			},
		}
	}

	type want struct {
		mrds []extv1alpha1.ManagedResourceDefinition
		err  error
	}

	cases := map[string]struct {
		reason   string
		client   client.Client
		revision v1.PackageRevisionWithRuntime
		want     want
	}{
		"NoSafeStartCapability": {
			reason:   "We should not list MRDs at all for a revision without the safe-start capability.",
			client:   &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			revision: &v1.ProviderRevision{},
			want:     want{},
		},
		"ErrListMRDs": {
			reason:   "We should return an error if we can't list MRDs.",
			client:   &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			revision: owner,
			want:     want{err: errors.Wrap(errBoom, errListMRDs)},
		},
		"OnlyOwnedMRDs": {
			reason: "We should return only the MRDs our revision controls.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(l client.ObjectList) error {
				l.(*extv1alpha1.ManagedResourceDefinitionList).Items = []extv1alpha1.ManagedResourceDefinition{
					mrd("ours", "owner-uid"),
					mrd("theirs", "other-uid"),
				}
				return nil
			})},
			revision: owner,
			want:     want{mrds: []extv1alpha1.ManagedResourceDefinition{mrd("ours", "owner-uid")}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := NewProviderHooks(tc.client, namespace, crossplaneName, nil).ownedMRDs(context.Background(), tc.revision)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nownedMRDs(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.mrds, got); diff != "" {
				t.Errorf("\n%s\nownedMRDs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
