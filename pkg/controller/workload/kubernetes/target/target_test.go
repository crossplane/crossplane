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

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"

	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	workloadv1alpha1 "github.com/crossplaneio/crossplane/apis/workload/v1alpha1"
)

const (
	namespace  = "coolNamespace"
	name       = "coolCluster"
	uid        = types.UID("definitely-a-uuid")
	targetName = "definitely-a-uuid"
)

var (
	errorBoom  = errors.New("boom")
	objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
	targetMeta = metav1.ObjectMeta{Namespace: namespace, Name: targetName}
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

type kubeTargetModifier func(*workloadv1alpha1.KubernetesTarget)

func withOwnerReferences(o []metav1.OwnerReference) kubeTargetModifier {
	return func(r *workloadv1alpha1.KubernetesTarget) {
		r.SetOwnerReferences(o)
	}
}

func withConnectionSecretRef(s *runtimev1alpha1.LocalSecretReference) kubeTargetModifier {
	return func(r *workloadv1alpha1.KubernetesTarget) {
		r.SetWriteConnectionSecretToReference(s)
	}
}

func withTargetLabels(l map[string]string) kubeTargetModifier {
	return func(r *workloadv1alpha1.KubernetesTarget) {
		r.SetLabels(l)
	}
}

func kubeTarget(rm ...kubeTargetModifier) *workloadv1alpha1.KubernetesTarget {
	r := &workloadv1alpha1.KubernetesTarget{ObjectMeta: targetMeta}

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
	controller := true

	cases := map[string]struct {
		rec        *Reconciler
		req        reconcile.Request
		wantResult reconcile.Result
		wantErr    error
	}{
		"FailedToGetNonExistentKubernetesCluster": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name)),
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		"FailedToGetExtantKubernetesCluster": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errorBoom),
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrap(errorBoom, errGetKubernetesCluster),
		},
		"KubernetesClusterDeleted": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*computev1alpha1.KubernetesCluster) = *(kubeCluster(withDeletionTimestamp(time.Now())))
						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		"FailedToUpdateKubernetesTarget": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							))
						case client.ObjectKey{Namespace: namespace, Name: targetName}:
							*obj.(*workloadv1alpha1.KubernetesTarget) = *kubeTarget(withOwnerReferences(
								[]metav1.OwnerReference{
									{
										UID:        uid,
										Name:       name,
										Kind:       computev1alpha1.KubernetesClusterKind,
										APIVersion: computev1alpha1.SchemeGroupVersion.String(),
										Controller: &controller,
									},
								},
							), withConnectionSecretRef(&runtimev1alpha1.LocalSecretReference{Name: "wrong-secret"}))
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						return errorBoom
					}),
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{},
			wantErr:    errors.Wrap(errorBoom, errCreateOrUpdateTarget),
		},
		"KubernetesTargetConflict": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster()
						case client.ObjectKey{Namespace: namespace, Name: targetName}:
							*obj.(*workloadv1alpha1.KubernetesTarget) = *kubeTarget(withOwnerReferences(
								[]metav1.OwnerReference{
									{
										UID:        "wrong-uid",
										Controller: &controller,
									},
								},
							))
						}
						return nil
					},
				},
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{},
			wantErr:    errors.Wrap(errors.New(errTargetConflict), errCreateOrUpdateTarget),
		},
		"SuccessfulCreateTarget": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						clusterKey := client.ObjectKey{Namespace: namespace, Name: name}
						if key == clusterKey {
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							), withLabels(map[string]string{"dev": "true"}))
							return nil
						}

						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
					MockCreate: test.NewMockCreateFn(nil, func(got runtime.Object) error {
						want := kubeTarget(withOwnerReferences(
							[]metav1.OwnerReference{
								{
									UID:        uid,
									Name:       name,
									Kind:       computev1alpha1.KubernetesClusterKind,
									APIVersion: computev1alpha1.SchemeGroupVersion.String(),
									Controller: &controller,
								},
							},
						),
							withConnectionSecretRef(&runtimev1alpha1.LocalSecretReference{Name: "super-secret"}),
							withTargetLabels(map[string]string{"dev": "true"}))
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
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
		"SuccessfulUpdateExistingFilledTarget": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							), withLabels(map[string]string{"dev": "true"}))
						case client.ObjectKey{Namespace: namespace, Name: targetName}:
							*obj.(*workloadv1alpha1.KubernetesTarget) = *kubeTarget(withOwnerReferences(
								[]metav1.OwnerReference{
									{
										UID:        uid,
										Name:       name,
										Kind:       computev1alpha1.KubernetesClusterKind,
										APIVersion: computev1alpha1.SchemeGroupVersion.String(),
										Controller: &controller,
									},
								},
							),
								withConnectionSecretRef(&runtimev1alpha1.LocalSecretReference{Name: "super-secret"}),
								withTargetLabels(map[string]string{"not": "dev"}))
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := kubeTarget(withOwnerReferences(
							[]metav1.OwnerReference{
								{
									UID:        uid,
									Name:       name,
									Kind:       computev1alpha1.KubernetesClusterKind,
									APIVersion: computev1alpha1.SchemeGroupVersion.String(),
									Controller: &controller,
								},
							},
						),
							withConnectionSecretRef(&runtimev1alpha1.LocalSecretReference{Name: "super-secret"}),
							withTargetLabels(map[string]string{"dev": "true"}))
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
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
		"SuccessfulUpdateExistingEmptyTarget": {
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch key {
						case client.ObjectKey{Namespace: namespace, Name: name}:
							*obj.(*computev1alpha1.KubernetesCluster) = *kubeCluster(withWriteConnectionSecretToRef(
								&runtimev1alpha1.LocalSecretReference{Name: "super-secret"},
							), withLabels(map[string]string{"dev": "true"}))
						case client.ObjectKey{Namespace: namespace, Name: targetName}:
							*obj.(*workloadv1alpha1.KubernetesTarget) = *kubeTarget(withOwnerReferences(
								[]metav1.OwnerReference{
									{
										UID:        uid,
										Name:       name,
										Kind:       computev1alpha1.KubernetesClusterKind,
										APIVersion: computev1alpha1.SchemeGroupVersion.String(),
										Controller: &controller,
									},
								},
							))
						}
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil, func(got runtime.Object) error {
						want := kubeTarget(withOwnerReferences(
							[]metav1.OwnerReference{
								{
									UID:        uid,
									Name:       name,
									Kind:       computev1alpha1.KubernetesClusterKind,
									APIVersion: computev1alpha1.SchemeGroupVersion.String(),
									Controller: &controller,
								},
							},
						),
							withConnectionSecretRef(&runtimev1alpha1.LocalSecretReference{Name: "super-secret"}),
							withTargetLabels(map[string]string{"dev": "true"}))
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("-want, +got:\n%s", diff)
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
