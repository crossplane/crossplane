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

package definition

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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
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
		"CompositeResourceDefinitionNotFound": {
			reason: "We should not return an error if the CompositeResourceDefinition was not found.",
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
		"GetCompositeResourceDefinitionError": {
			reason: "We should return any other error encountered while getting a CompositeResourceDefinition.",
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
				err: errors.Wrap(errBoom, errGetXRD),
			},
		},
		"CompositeResourceDefinitionDeleted": {
			reason: "We should return early if the CompositeResourceDefinition was deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v2.CompositeResourceDefinition)
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
		"ApplyClusterRoleError": {
			reason: "We should return errors encountered while applying a ClusterRole.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v2.CompositeResourceDefinition) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyRole),
			},
		},
		"SuccessfulNoOp": {
			reason: "We should not requeue when no ClusterRoles need applying.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(ctx context.Context, o client.Object, _ ...resource.ApplyOption) error {
							// Simulate a no-op change by not allowing the update.
							return resource.AllowUpdateIf(func(_, _ runtime.Object) bool { return false })(ctx, o, o)
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v2.CompositeResourceDefinition) []rbacv1.ClusterRole {
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
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithClusterRoleRenderer(ClusterRoleRenderFn(func(*v2.CompositeResourceDefinition) []rbacv1.ClusterRole {
						return []rbacv1.ClusterRole{{}}
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

func TestClusterRolesDiffer(t *testing.T) {
	cases := map[string]struct {
		current runtime.Object
		desired runtime.Object
		want    bool
	}{
		"Equal": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: false,
		},
		"LabelsDiffer": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"b": "b"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			want: true,
		},
		"RulesDiffer": {
			current: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
				Rules: []rbacv1.PolicyRule{{}},
			},
			desired: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"a": "a"},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ClusterRolesDiffer(tc.current, tc.desired)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ClusterRolesDiffer(...): -want, +got\n:%s", diff)
			}
		})
	}
}
