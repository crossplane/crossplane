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

package namespace

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
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
		"NamespaceNotFound": {
			reason: "We should not return an error if the Namespace was not found.",
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
		"GetNamespaceError": {
			reason: "We should return any other error encountered while getting a Namespace.",
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
				err: errors.Wrap(errBoom, errGetNamespace),
			},
		},
		"NamespaceDeleted": {
			reason: "We should return early if the namespace was deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*corev1.Namespace)
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
		"ListClusterRolesError": {
			reason: "We should return an error encountered listing ClusterRoles.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListRoles),
			},
		},
		"ApplyRoleError": {
			reason: "We should return an error encountered applying a Role.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithRoleRenderer(RoleRenderFn(func(*corev1.Namespace, []rbacv1.ClusterRole) []rbacv1.Role {
						return []rbacv1.Role{{}}
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyRole),
			},
		},
		"SuccessfulNoOp": {
			reason: "We should not requeue when no Roles need applying.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
							// Simulate a no-op change by not allowing the update.
							return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
						}),
					}),
					WithRoleRenderer(RoleRenderFn(func(*corev1.Namespace, []rbacv1.ClusterRole) []rbacv1.Role {
						return []rbacv1.Role{{}}
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulApply": {
			reason: "We should not requeue when we successfully apply our Roles.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithRoleRenderer(RoleRenderFn(func(*corev1.Namespace, []rbacv1.ClusterRole) []rbacv1.Role {
						return []rbacv1.Role{{}}
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
			r := NewReconciler(tc.args.mgr, append(tc.args.opts, WithLogger(testLog))...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRolesDiffer(t *testing.T) {
	cases := map[string]struct {
		current runtime.Object
		desired runtime.Object
		want    bool
	}{
		"Equal": {
			current: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: false,
		},
		"EqualMixedNonCrossplane": {
			current: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rbac.crossplane.io/a":        "a",
						"not-managed-by-crossplane/b": "b",
					},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: false,
		},
		"AnnotationsDiffer": {
			current: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/b": "b"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: true,
		},
		"RulesDiffer": {
			current: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"rbac.crossplane.io/a": "a"},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := RolesDiffer(tc.current, tc.desired)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("RolesDiffer(...): -want, +got\n:%s", diff)
			}
		})
	}
}
