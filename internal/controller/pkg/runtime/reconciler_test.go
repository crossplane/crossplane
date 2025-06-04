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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/features"
)

const (
	testNamespace  = "crossplane-system"
	crossplaneName = "crossplane"
)

var _ Hooks = &MockHooks{}

// MockHooks is a mock implementation of Hooks interface.
type MockHooks struct {
	MockPre        func(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
	MockPost       func(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
	MockDeactivate func(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error
}

// Pre calls MockPre if set, otherwise returns nil.
func (m *MockHooks) Pre(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error {
	if m.MockPre != nil {
		return m.MockPre(ctx, pr, b)
	}
	return nil
}

// Post calls MockPost if set, otherwise returns nil.
func (m *MockHooks) Post(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error {
	if m.MockPost != nil {
		return m.MockPost(ctx, pr, b)
	}
	return nil
}

// Deactivate calls MockDeactivate if set, otherwise returns nil.
func (m *MockHooks) Deactivate(ctx context.Context, pr v1.PackageRevisionWithRuntime, b ManifestBuilder) error {
	if m.MockDeactivate != nil {
		return m.MockDeactivate(ctx, pr, b)
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"WaitingForSignatureVerification": {
			reason: "We should wait for signature verification to complete.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							pr := o.(*v1.ProviderRevision)
							pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							pr.SetDesiredState(v1.PackageRevisionActive)
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.ProviderRevision{}
							want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("Waiting for signature verification to complete"))

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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableAlphaSignatureVerification)),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return errBoom
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return errBoom
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errPostHook),
			},
		},
		"ErrDeactivateRevision": {
			reason: "We should return an error if deactivation fails.",
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return errBoom
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to run deactivation hook"),
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
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("cannot prepare runtime manifest builder options: no deployment runtime config set"))

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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				err: errors.Wrap(errors.New(errNoRuntimeConfig), errManifestBuilderOptions),
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
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("cannot prepare runtime manifest builder options: cannot get referenced deployment runtime config: runtime config not found"))

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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithFeatureFlags(flagsWithFeatures(features.EnableBetaDeploymentRuntimeConfigs)),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errors.New("runtime config not found"), errGetRuntimeConfig), errManifestBuilderOptions),
			},
		},
		"MigratorNop": {
			reason: "Should use nop migrator for function revisions (no migration needed).",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.FunctionRevision:
								obj.SetGroupVersionKind(v1.FunctionRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-function"})
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
							want := &v1.FunctionRevision{}
							want.SetGroupVersionKind(v1.FunctionRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetLabels(map[string]string{v1.LabelParentPackage: "test-function"})
							want.SetConditions(v1.RevisionHealthy())
							want.SetConditions(v1.RuntimeHealthy())

							if diff := cmp.Diff(want, o); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithNewPackageRevisionWithRuntimeFn(func() v1.PackageRevisionWithRuntime { return &v1.FunctionRevision{} }),
					WithLogger(testLog),
					WithRecorder(event.NewNopRecorder()),
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"MigratorError": {
			reason: "Should return error when migrator fails.",
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
							want.SetConditions(v1.RuntimeUnhealthy().WithMessage("failed to run deployment selector migration: boom"))

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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{}),
					WithDeploymentSelectorMigrator(&MockDeploymentSelectorMigrator{
						MockMigrateDeploymentSelector: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "failed to run deployment selector migration"),
			},
		},
		"MigratorSuccess": {
			reason: "Should proceed normally when migrator succeeds.",
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
					}),
					WithDeploymentSelectorMigrator(&MockDeploymentSelectorMigrator{
						MockMigrateDeploymentSelector: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil // Migration succeeds
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
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
							want.SetConditions(v1.RuntimeHealthy())
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, b ManifestBuilder) error {
							d := b.Deployment("test-sa")
							// Verify that the deployment has the pull secret from the image config
							if len(d.Spec.Template.Spec.ImagePullSecrets) == 0 {
								return errors.New("expected deployment to have pull secret from image config")
							}
							if d.Spec.Template.Spec.ImagePullSecrets[0].Name != "test-pull-secret" {
								return errors.New("expected deployment to have pull secret named 'test-pull-secret'")
							}
							return nil
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulHealthyRevisionWithSignatureVerification": {
			reason: "A healthy revision with signature verification should complete successfully.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1.ProviderRevision:
								obj.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								obj.SetDesiredState(v1.PackageRevisionActive)
								obj.SetLabels(map[string]string{v1.LabelParentPackage: "test-provider"})
								obj.SetConditions(v1.VerificationSucceeded("foo"))
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
							want.SetConditions(v1.VerificationSucceeded("foo"))
							want.SetConditions(v1.RevisionHealthy())
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockPre: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
						MockPost: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
					}),
					WithFeatureFlags(flagsWithFeatures(features.EnableAlphaSignatureVerification)),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulInactiveRevision": {
			reason: "An inactive revision should deactivate successfully.",
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
					WithNamespace(testNamespace),
					WithServiceAccount(crossplaneName),
					WithRuntimeHooks(&MockHooks{
						MockDeactivate: func(_ context.Context, _ v1.PackageRevisionWithRuntime, _ ManifestBuilder) error {
							return nil
						},
					}),
					WithDeploymentSelectorMigrator(NewNopDeploymentSelectorMigrator()),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
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

// FlagsWithFeatures is a helper function to create feature.Flags with specific features enabled.
func flagsWithFeatures(features ...feature.Flag) *feature.Flags {
	flags := &feature.Flags{}
	for _, f := range features {
		flags.Enable(f)
	}
	return flags
}
