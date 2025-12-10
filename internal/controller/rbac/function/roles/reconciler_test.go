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

package roles

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/roles"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	now := metav1.Now()

	type args struct {
		mgr  manager.Manager
		opts []ReconcilerOption
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
		"FunctionRevisionNotFound": {
			reason: "We should not return an error if the FunctionRevision was not found.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetFunctionRevisionError": {
			reason: "We should return any other error encountered while getting a FunctionRevision.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetFR),
			},
		},
		"FunctionRevisionDeleted": {
			reason: "We should return early if the FunctionRevision was deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.FunctionRevision)
								d.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"PauseReconcile": {
			reason: "We should not reconcile a paused FunctionRevision.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								fr := o.(*v1.FunctionRevision)
								fr.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ApplyClusterRoleError": {
			reason: "We should return an error encountered applying a ClusterRole.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithLogger(testLog),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.FunctionRevision, []roles.Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtApplyRole, ""),
			},
		},
		"SuccessfulNoOp": {
			reason: "We should not requeue when no ClusterRoles need applying.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithLogger(testLog),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
							// Simulate a no-op change by not allowing the update.
							return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.FunctionRevision, []roles.Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulApply": {
			reason: "We should not requeue when we successfully apply our ClusterRoles.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithLogger(testLog),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							// Simulate a successful apply by setting a resource version.
							o.SetResourceVersion("1")
							return nil
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v1.FunctionRevision, []roles.Resource) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{
							ObjectMeta: metav1.ObjectMeta{
								Name: "test-role",
							},
						}}
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.opts...)
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
