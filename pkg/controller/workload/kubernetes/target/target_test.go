/*
Copyright 2019 The Crossplane Authors.

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

package target

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	computev1alpha1 "github.com/crossplane/crossplane/apis/compute/v1alpha1"
	workloadv1alpha1 "github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	namespace = "coolNamespace"
	name      = "coolCluster"
	uid       = types.UID("definitely-a-uuid")
)

var (
	errorBoom  = errors.New("boom")
	objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
)

type kubeClusterModifier func(*computev1alpha1.KubernetesCluster)

func withDeletionTimestamp(t time.Time) kubeClusterModifier {
	return func(r *computev1alpha1.KubernetesCluster) {
		r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	}
}

func withWriteConnectionSecretToRef(s *runtimev1alpha1.LocalSecretReference) kubeClusterModifier {
	return func(r *computev1alpha1.KubernetesCluster) {
		r.Spec.WriteConnectionSecretToReference = s
	}
}

func withLabels(l map[string]string) kubeClusterModifier {
	return func(r *computev1alpha1.KubernetesCluster) {
		r.SetLabels(l)
	}
}

func kubeCluster(rm ...kubeClusterModifier) *computev1alpha1.KubernetesCluster {
	r := &computev1alpha1.KubernetesCluster{ObjectMeta: objectMeta}

	for _, m := range rm {
		m(r)
	}

	return r
}

func TestClusterIsBound(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"Bound": {
			obj: &computev1alpha1.KubernetesCluster{
				Status: runtimev1alpha1.ResourceClaimStatus{
					BindingStatus: runtimev1alpha1.BindingStatus{
						Phase: runtimev1alpha1.BindingPhaseBound,
					},
				},
			},
			want: true,
		},
		"Unbound": {
			obj: &computev1alpha1.KubernetesCluster{
				Status: runtimev1alpha1.ResourceClaimStatus{
					BindingStatus: runtimev1alpha1.BindingStatus{
						Phase: runtimev1alpha1.BindingPhaseUnbound,
					},
				},
			},
			want: false,
		},
		"NotAKubernetesCluster": {
			obj:  nil,
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := clusterIsBound(tc.obj)
			if got != tc.want {
				t.Errorf("clusterIsBound(...): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	cases := map[string]struct {
		rec        *Reconciler
		req        reconcile.Request
		wantResult reconcile.Result
		wantErr    error
	}{
		"FailedToGetNonExistentKubernetesCluster": {
			rec: &Reconciler{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name)),
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		"FailedToGetExtantKubernetesCluster": {
			rec: &Reconciler{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errorBoom),
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrap(errorBoom, errGetKubernetesCluster),
		},
		"KubernetesClusterDeleted": {
			rec: &Reconciler{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
							*obj.(*computev1alpha1.KubernetesCluster) = *(kubeCluster(withDeletionTimestamp(time.Now())))
							return nil
						},
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		"FailedToApplyTarget": {
			rec: &Reconciler{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							), withLabels(map[string]string{"dev": "true"}))
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
						return errorBoom
					}),
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{},
			wantErr:    errors.Wrap(errorBoom, errCreateOrUpdateTarget),
		},
		"SuccessfulApplyTarget": {
			rec: &Reconciler{
				client: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							), withLabels(map[string]string{"dev": "true"}))
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, got runtime.Object, _ ...resource.ApplyOption) error {
						want := &workloadv1alpha1.KubernetesTarget{}
						want.SetNamespace(kubeCluster().GetNamespace())
						want.SetName(kubeCluster().GetName())
						want.SetWriteConnectionSecretToReference(&runtimev1alpha1.LocalSecretReference{Name: "super-secret"})
						want.SetLabels(kubeCluster().GetLabels())
						meta.AddLabels(want, map[string]string{"dev": "true"})
						meta.AddLabels(want, map[string]string{LabelKeyAutoTarget: kubeCluster().GetName()})
						meta.AddOwnerReference(want, meta.AsController(meta.ReferenceTo(kubeCluster(), computev1alpha1.KubernetesClusterGroupVersionKind)))

						if diff := cmp.Diff(got, want); diff != "" {
							t.Errorf("Apply: -want, +got:\n %s", diff)
						}
						return nil
					}),
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{},
			wantErr:    nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestMustHaveLabel(t *testing.T) {
	key := "cool"
	value := "very"

	type args struct {
		ctx     context.Context
		current runtime.Object
		desired runtime.Object
	}
	cases := map[string]struct {
		k    string
		v    string
		args args
		want error
	}{
		"MissingLabel": {
			k: key,
			v: value,
			args: args{
				current: &workloadv1alpha1.KubernetesTarget{},
			},
			want: errors.Errorf("existing object is not labelled '%s: %s'", key, value),
		},
		"HasLabel": {
			k: key,
			v: value,
			args: args{
				current: &workloadv1alpha1.KubernetesTarget{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{key: value}}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := MustHaveLabel(tc.k, tc.v)(tc.args.ctx, tc.args.current, tc.args.desired)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("MustHaveLabel: want error != got error:\n%s", diff)
			}
		})
	}
}
