package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

const (
	versionCrossplane = "v0.11.1"
	providerDep       = "crossplane/provider-aws"
	versionDep        = "v0.1.1"
)

var (
	errBoom = errors.New("boom")
)

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
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			want: want{
				err: errors.New(errNotProvider),
			},
		},
		"ErrNotProviderRevision": {
			reason: "Should return error if the supplied package revision is not a provider revision.",
			args: args{
				pkg: &pkgmetav1.Provider{},
			},
			want: want{
				err: errors.New(errNotProviderRevision),
			},
		},
		"PermissionRequestsPropagated": {
			reason: "Should propagate permission requests from provider to revision",
			args: args{
				pkg: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						Controller: pkgmetav1.ControllerSpec{
							PermissionRequests: []rbacv1.PolicyRule{
								{
									APIGroups: []string{"somegroup"},
									Resources: []string{"somekinds"},
									Verbs:     []string{"someverbs"},
								},
							},
						},
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
								Version: versionCrossplane,
							},
							DependsOn: []pkgmetav1.Dependency{{
								Provider: pointer.String(providerDep),
								Version:  versionDep,
							}},
						},
					},
				},
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{},
					},
				},
			},
			want: want{
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{},
					},
					Status: v1.PackageRevisionStatus{
						PermissionRequests: []rbacv1.PolicyRule{
							{
								APIGroups: []string{"somegroup"},
								Resources: []string{"somekinds"},
								Verbs:     []string{"someverbs"},
							},
						},
					},
				},
			},
		},
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
							TLSClientSecretName: pointer.String("some-client-secret"),
							TLSServerSecretName: pointer.String("some-server-secret"),
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceFn: func(overrides ...ServiceOverrides) *corev1.Service {
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{
							TLSClientSecretName: pointer.String("some-client-secret"),
							TLSServerSecretName: pointer.String("some-server-secret"),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client)
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
		"ErrNotProvider": {
			reason: "Should return error if not provider.",
			want: want{
				err: errors.New(errNotProvider),
			},
		},
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
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
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
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
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
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
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
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
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
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
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
				rev: &v1.ProviderRevision{
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							DesiredState: v1.PackageRevisionActive,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewProviderHooks(tc.args.client)
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
		"ErrDeleteSA": {
			reason: "Should return error if we fail to delete service account.",
			args: args{
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
				},
				client: &test.MockClient{
					MockDelete: func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteProviderSA),
			},
		},
		"ErrDeleteDeployment": {
			reason: "Should return error if we fail to delete deployment.",
			args: args{
				manifests: &MockManifestBuilder{
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
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
					ServiceAccountFn: func(overrides ...ServiceAccountOverrides) *corev1.ServiceAccount {
						return &corev1.ServiceAccount{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-sa",
							},
						}
					},
					DeploymentFn: func(serviceAccount string, overrides ...DeploymentOverrides) *appsv1.Deployment {
						return &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name: "some-deployment",
							},
						}
					},
					ServiceFn: func(overrides ...ServiceOverrides) *corev1.Service {
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
							if obj.GetName() != "some-sa" {
								return errors.New("unexpected service account name")
							}
							return nil
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
			h := NewProviderHooks(tc.args.client)
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
