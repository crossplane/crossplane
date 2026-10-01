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

package runtime

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	fakexpkg "github.com/crossplane/crossplane-runtime/v2/pkg/xpkg/fake"

	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/features"
)

const (
	testNamespace  = "crossplane-system"
	crossplaneName = "crossplane"
)

var _ Hooks = &MockHooks{}

// MockHooks is a mock implementation of the Hooks interface.
type MockHooks struct {
	MockPre        func(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error
	MockPost       func(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error
	MockDeactivate func(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error
}

// Pre calls MockPre if set, otherwise returns nil.
func (m *MockHooks) Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if m.MockPre != nil {
		return m.MockPre(ctx, pr, rc)
	}

	return nil
}

// Post calls MockPost if set, otherwise returns nil.
func (m *MockHooks) Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if m.MockPost != nil {
		return m.MockPost(ctx, pr, rc)
	}

	return nil
}

// Deactivate calls MockDeactivate if set, otherwise returns nil.
func (m *MockHooks) Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
	if m.MockDeactivate != nil {
		return m.MockDeactivate(ctx, pr, rc)
	}

	return nil
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))

	type args struct {
		mgr manager.Manager
		rec []ReconcilerOption
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"PackageRevisionNotFound": {
			reason: "We should not return an error and not requeue if package revision not found.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrGetPackageRevision": {
			reason: "We should return an error if getting package revision fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPackageRevision),
			},
		},
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							pr := o.(*v1.ProviderRevision)
							pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							pr.SetDesiredState(v1.PackageRevisionActive)
							pr.SetAnnotations(map[string]string{
								meta.AnnotationKeyReconciliationPaused: "true",
							})
							return nil
						}),
						// No status update should occur for paused reconciliation
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"DeletedRevision": {
			reason: "Do not reconcile if the package revision is marked for deletion.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							pr := o.(*v1.ProviderRevision)
							pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							pr.SetDesiredState(v1.PackageRevisionActive)
							pr.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
							return nil
						}),
						// No status update should occur for deleted reconciliation
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrPreHook": {
			reason: "We should return an error if pre-hook fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("pre establish runtime hook failed for package: boom"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errPreHook),
			},
		},
		"WaitForEstablished": {
			reason: "We should wait if package revision is not yet established.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								// No installed condition
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errors.New("cannot update package revision status"), func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("Package revision is not healthy yet"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.New("cannot update package revision status"), errUpdateStatus),
			},
		},
		"ErrPostHook": {
			reason: "We should return an error if post-hook fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								obj.SetConditions(v1.RevisionHealthy())
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RevisionHealthy())
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("post establish runtime hook failed for package: boom"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errPostHook),
			},
		},
		"ErrDeactivateRevision": {
			reason: "We should report an unhealthy runtime and return an error if deactivation fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionInactive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionInactive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("deactivation runtime hook failed for package: boom"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeactivateHook),
			},
		},
		"DeactivateRevisionConflict": {
			reason: "We should requeue without an error if we lose a race while deactivating.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionInactive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return kerrors.NewConflict(schema.GroupResource{Resource: "services"}, "test-provider", errBoom)
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ErrNoRuntimeConfig": {
			reason: "Should return error when beta deployment runtime configs are enabled but no runtime config is referenced.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							pr := o.(*v1.ProviderRevision)
							pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							pr.SetDesiredState(v1.PackageRevisionActive)
							pr.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							// No runtime config reference
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("cannot resolve deployment runtime config for package: no deployment runtime config set"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
				},
			},
			want: want{
				err: errors.Wrap(errors.New(errNoRuntimeConfig), errRuntimeConfig),
			},
		},
		"ErrGetImageConfig": {
			reason: "Should return error when image config cannot be looked up.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							obj := o.(*v1.ProviderRevision)
							obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							obj.SetDesiredState(v1.PackageRevisionActive)
							obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							obj.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "default"})
							obj.SetResolvedSource("example.com/provider:v1.0.0")
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "default"})
							want.SetResolvedSource("example.com/provider:v1.0.0")
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("cannot resolve deployment runtime config for package: failed to look up runtime ImageConfig for example.com/provider:v1.0.0: boom"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockRuntimeConfigFor: fakexpkg.NewMockRuntimeConfigForFn("", nil, errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.New("failed to look up runtime ImageConfig for example.com/provider:v1.0.0: boom"), errRuntimeConfig),
			},
		},
		"ErrGetRuntimeConfig": {
			reason: "Should return error when runtime config cannot be retrieved.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								obj.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "test-runtime-config"})
								return nil
							case *v1beta1.DeploymentRuntimeConfig:
								return errors.New("runtime config not found")
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "test-runtime-config"})
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("cannot resolve deployment runtime config for package: cannot get referenced deployment runtime config: runtime config not found"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockRuntimeConfigFor: fakexpkg.NewMockRuntimeConfigForFn("", nil, nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.New("runtime config not found"), errGetRuntimeConfig), errRuntimeConfig),
			},
		},
		"SuccessfulHealthyRevision": {
			reason: "A healthy revision should complete successfully.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								obj.SetConditions(v1.RevisionHealthy())
								obj.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "test-image-config",
									Reason: v1.ImageConfigReasonSetPullSecret,
								})
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							case *v1beta1.ImageConfig:
								obj.SetGroupVersionKind(v1beta1.ImageConfigGroupVersionKind)
								obj.SetName("test-image-config")
								obj.Spec.Registry = &v1beta1.RegistryConfig{
									Authentication: &v1beta1.RegistryAuthentication{
										PullSecretRef: corev1.LocalObjectReference{
											Name: "test-pull-secret",
										},
									},
								}
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RevisionHealthy())
							want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
								Name:   "test-image-config",
								Reason: v1.ImageConfigReasonSetPullSecret,
							})

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"RuntimeConditionsFromPost": {
			reason: "Conditions the runtime hooks mark in Post should be persisted.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if obj, ok := o.(*v1.ProviderRevision); ok {
								setRuntimeActivationRevision(obj, pkgmetav1.ProviderCapabilitySafeStart)
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							setRuntimeActivationRevision(want, pkgmetav1.ProviderCapabilitySafeStart)
							want.SetConditions(v1.RuntimeHealthy(), v1.RuntimeAwaitingActivation().WithMessage("Package runtime is scaled to zero; awaiting the first ManagedResourceDefinition to be activated"))

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPost: func(_ context.Context, pr v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							pr.SetConditions(v1.RuntimeHealthy(), v1.RuntimeAwaitingActivation().WithMessage("Package runtime is scaled to zero; awaiting the first ManagedResourceDefinition to be activated"))
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulHealthyRevisionWithImageConfigRuntimeConfig": {
			reason: "Builder should use DeploymentRuntimeConfig from ImageConfig when configured.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								obj.SetConditions(v1.RevisionHealthy())
								obj.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "default-runtime-config"})
								obj.SetResolvedSource("example.com/test-provider:v1.0.0")
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							case *v1beta1.DeploymentRuntimeConfig:
								obj.SetGroupVersionKind(v1beta1.DeploymentRuntimeConfigGroupVersionKind)
								obj.Spec.DeploymentTemplate = &v1beta1.DeploymentTemplate{
									Metadata: &v1beta1.ObjectMeta{
										Name: new("deployment-name-override"),
									},
								}
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RevisionHealthy())
							want.SetRuntimeConfigRef(&v1.RuntimeConfigReference{Name: "default-runtime-config"})
							want.SetResolvedSource("example.com/test-provider:v1.0.0")
							want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
								Name:   "test-image-config",
								Reason: v1.ImageConfigReasonRuntime,
							})

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, rc *v1beta1.DeploymentRuntimeConfig) error {
							// Validate that the runtime config the ImageConfig
							// pointed at is the one we were handed.
							if rc == nil || rc.Spec.DeploymentTemplate == nil || rc.Spec.DeploymentTemplate.Metadata == nil {
								return errors.New("deployment runtime config was not resolved")
							}
							if got := ptr.Deref(rc.Spec.DeploymentTemplate.Metadata.Name, ""); got != "deployment-name-override" {
								return errors.Errorf("got unexpected deployment name %q; deployment runtime config was not applied", got)
							}
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
					}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockRuntimeConfigFor: func() func(context.Context, string) (string, *v1beta1.ImageRuntime, error) {
							return fakexpkg.NewMockRuntimeConfigForFn(
								"test-image-config",
								&v1beta1.ImageRuntime{
									ConfigReference: &v1beta1.RuntimeConfigReference{
										APIVersion: new(v1beta1.SchemeGroupVersion.String()),
										Kind:       &v1beta1.DeploymentRuntimeConfigKind,
										Name:       "image-runtime-config",
									},
								},
								nil,
							)
						}(),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulInactiveRevision": {
			reason: "An inactive revision should deactivate successfully and report a healthy runtime.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionInactive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								// set a previous RuntimeUnhealthy condition on the object, so we
								// know the later status update clears it back to healthy
								obj.SetConditions(v1.RuntimeUnhealthy().WithMessage("deactivation runtime hook failed for package: boom"))
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionInactive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
							want.SetConditions(v1.RuntimeHealthy())

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrUpdateStatusInactiveRevision": {
			reason: "We should return an error if we cannot report a deactivated revision's healthy runtime.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionInactive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								return nil
							case *corev1.ServiceAccount:
								obj.Name = crossplaneName
								obj.Namespace = testNamespace
								return nil
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ *v1beta1.DeploymentRuntimeConfig) error {
							return nil
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc := tc
			r := NewReconciler(tc.args.mgr, tc.args.rec...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

const runtimeActivationUID = "12345678-1234-1234-1234-123456789012"

// setRuntimeActivationRevision configures a healthy, active provider revision
// with the given capabilities for runtime activation test cases.
func setRuntimeActivationRevision(obj *v1.ProviderRevision, capabilities ...string) {
	obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
	obj.SetUID(runtimeActivationUID)
	obj.SetDesiredState(v1.PackageRevisionActive)
	obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
	obj.SetConditions(v1.RevisionHealthy())
	obj.SetCapabilities(capabilities)
}

// FlagsWithFeatures is a helper function to create feature.Flags with specific features enabled.
func flagsWithFeatures(features ...feature.Flag) *feature.Flags {
	flags := &feature.Flags{}
	for _, f := range features {
		flags.Enable(f)
	}

	return flags
}
