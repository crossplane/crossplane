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

package binding

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
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
		"ProviderRevisionNotFound": {
			reason: "We should not return an error if the ProviderRevision was not found.",
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
		"GetProviderRevisionError": {
			reason: "We should return any other error encountered while getting a ProviderRevision.",
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
				err: errors.Wrap(errBoom, errGetPR),
			},
		},
		"ProviderRevisionDeleted": {
			reason: "We should return early if the namespace was deleted.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
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
		"ListDeploymentsError": {
			reason: "We should return an error encountered listing Deployments.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								return nil
							}),
							MockList: test.NewMockListFn(errBoom),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeployments),
			},
		},
		"ApplyClusterRoleBindingError": {
			reason: "We should return an error encountered applying a ClusterRoleBinding.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								return nil
							}),
							MockList: test.NewMockListFn(nil),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyBinding),
			},
		},
		"SuccessfulApply": {
			reason: "We should not requeue when we successfully apply our ClusterRoleBindings.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								// Exercise the logic that filters out
								// ServiceAccounts that are not owned by the
								// ProviderRevision. Note the ServiceAccount's
								// owned's UID matches that of the
								// ProviderRevision because they're both the
								// empty string.
								l := o.(*appsv1.DeploymentList)
								l.Items = []appsv1.Deployment{{
									ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{}}},
								}}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(context.Context, client.Object, ...resource.ApplyOption) error {
							return nil
						}),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.ProviderRevision)
								d.SetOwnerReferences([]metav1.OwnerReference{{}})
								d.Spec.DesiredState = v1.PackageRevisionActive
								d.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
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
