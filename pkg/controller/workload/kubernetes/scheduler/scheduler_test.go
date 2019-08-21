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
limitations under the License
*/

package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	workloadv1alpha1 "github.com/crossplaneio/crossplane/apis/workload/v1alpha1"
)

const (
	namespace = "coolNamespace"
	name      = "coolApp"
	uid       = types.UID("definitely-a-uuid")
)

var (
	errorBoom  = errors.New("boom")
	objectMeta = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
	ctx        = context.Background()

	selectorAll = &metav1.LabelSelector{}

	clusterA = &computev1alpha1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolClusterA"},
	}
	clusterB = &computev1alpha1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolClusterB"},
	}

	clusters = &computev1alpha1.KubernetesClusterList{
		Items: []computev1alpha1.KubernetesCluster{*clusterA, *clusterB},
	}
)

type kubeAppModifier func(*workloadv1alpha1.KubernetesApplication)

func withConditions(c ...runtimev1alpha1.Condition) kubeAppModifier {
	return func(r *workloadv1alpha1.KubernetesApplication) { r.Status.SetConditions(c...) }
}

func withState(s workloadv1alpha1.KubernetesApplicationState) kubeAppModifier {
	return func(r *workloadv1alpha1.KubernetesApplication) { r.Status.State = s }
}

func withDeletionTimestamp(t time.Time) kubeAppModifier {
	return func(r *workloadv1alpha1.KubernetesApplication) {
		r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	}
}

func withCluster(c *corev1.ObjectReference) kubeAppModifier {
	return func(r *workloadv1alpha1.KubernetesApplication) {
		r.Status.Cluster = c
	}
}

func withClusterSelector(s *metav1.LabelSelector) kubeAppModifier {
	return func(r *workloadv1alpha1.KubernetesApplication) {
		r.Spec.ClusterSelector = s
	}
}

func kubeApp(rm ...kubeAppModifier) *workloadv1alpha1.KubernetesApplication {
	r := &workloadv1alpha1.KubernetesApplication{ObjectMeta: objectMeta}

	for _, m := range rm {
		m(r)
	}

	return r
}

func TestCreatePredicate(t *testing.T) {
	cases := []struct {
		name  string
		event event.CreateEvent
		want  bool
	}{
		{
			name: "UnscheduledCluster",
			event: event.CreateEvent{
				Object: &workloadv1alpha1.KubernetesApplication{
					Status: workloadv1alpha1.KubernetesApplicationStatus{
						Cluster: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "ScheduledCluster",
			event: event.CreateEvent{
				Object: &workloadv1alpha1.KubernetesApplication{
					Status: workloadv1alpha1.KubernetesApplicationStatus{
						Cluster: &corev1.ObjectReference{Name: "coolClustetr"},
					},
				},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplication",
			event: event.CreateEvent{
				Object: &workloadv1alpha1.KubernetesApplicationResource{},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CreatePredicate(tc.event)
			if got != tc.want {
				t.Errorf("CreatePredicate(...): got %v, want %v", got, tc.want)
			}
		})
	}
}
func TestUpdatePredicate(t *testing.T) {
	cases := []struct {
		name  string
		event event.UpdateEvent
		want  bool
	}{
		{
			name: "UnscheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &workloadv1alpha1.KubernetesApplication{
					Status: workloadv1alpha1.KubernetesApplicationStatus{
						Cluster: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "ScheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &workloadv1alpha1.KubernetesApplication{
					Status: workloadv1alpha1.KubernetesApplicationStatus{
						Cluster: &corev1.ObjectReference{Name: "coolCluster"},
					},
				},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplication",
			event: event.UpdateEvent{
				ObjectNew: &workloadv1alpha1.KubernetesApplicationResource{},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UpdatePredicate(tc.event)
			if got != tc.want {
				t.Errorf("UpdatePredicate(...): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSchedule(t *testing.T) {
	cases := []struct {
		name       string
		scheduler  scheduler
		app        *workloadv1alpha1.KubernetesApplication
		wantApp    *workloadv1alpha1.KubernetesApplication
		wantResult reconcile.Result
	}{
		{
			name: "SuccessfulSchedule",
			scheduler: &roundRobinScheduler{
				kube: &test.MockClient{
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						*obj.(*computev1alpha1.KubernetesClusterList) = *clusters
						return nil
					},
				},
			},
			app: kubeApp(withClusterSelector(selectorAll)),
			wantApp: kubeApp(
				withClusterSelector(selectorAll),
				withCluster(meta.ReferenceTo(clusterA, computev1alpha1.KubernetesClusterGroupVersionKind)),
				withState(workloadv1alpha1.KubernetesApplicationStateScheduled),
				withConditions(runtimev1alpha1.ReconcileSuccess()),
			),
			wantResult: reconcile.Result{Requeue: false},
		},
		{
			name: "ErrorListingClusters",
			scheduler: &roundRobinScheduler{
				kube: &test.MockClient{MockList: test.NewMockListFn(errorBoom)},
			},
			app: kubeApp(withClusterSelector(selectorAll)),
			wantApp: kubeApp(
				withClusterSelector(selectorAll),
				withState(workloadv1alpha1.KubernetesApplicationStatePending),
				withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
			),
			wantResult: reconcile.Result{Requeue: true},
		},
		{
			name: "NoMatchingClusters",
			scheduler: &roundRobinScheduler{
				kube: &test.MockClient{
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						*obj.(*computev1alpha1.KubernetesClusterList) = computev1alpha1.KubernetesClusterList{}
						return nil
					},
				},
			},
			app: kubeApp(withClusterSelector(selectorAll)),
			wantApp: kubeApp(
				withClusterSelector(selectorAll),
				withState(workloadv1alpha1.KubernetesApplicationStatePending),
				withConditions(runtimev1alpha1.ReconcileSuccess()),
			),
			wantResult: reconcile.Result{Requeue: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult := tc.scheduler.schedule(ctx, tc.app)

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.scheduler.Schedule(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantApp, tc.app); diff != "" {
				t.Errorf("app: -want, +got:\n%s", diff)
			}
		})
	}
}

type mockScheduleFn func(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) reconcile.Result

func newMockscheduleFn(r reconcile.Result) mockScheduleFn {
	return func(_ context.Context, _ *workloadv1alpha1.KubernetesApplication) reconcile.Result { return r }
}

type mockScheduler struct {
	mockSchedule mockScheduleFn
}

func (s *mockScheduler) schedule(ctx context.Context, app *workloadv1alpha1.KubernetesApplication) reconcile.Result {
	return s.mockSchedule(ctx, app)
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name       string
		rec        *Reconciler
		req        reconcile.Request
		wantResult reconcile.Result
		wantErr    error
	}{
		{
			name: "FailedToGetNonExistentKubernetesApplication",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, name)),
				},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		{
			name: "FailedToGetExtantKubernetesApplication",
			rec: &Reconciler{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrapf(errorBoom, "cannot get %s %s/%s", workloadv1alpha1.KubernetesApplicationKind, namespace, name),
		},
		{
			name: "KubernetesApplicationWasDeleted",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*workloadv1alpha1.KubernetesApplication) = *(kubeApp(withDeletionTimestamp(time.Now())))
						return nil
					},
				},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		{
			name: "KubernetesApplicationAlreadyScheduled",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*workloadv1alpha1.KubernetesApplication) = *(kubeApp(
							withCluster(&corev1.ObjectReference{Name: "coolCluster"}),
						))
						return nil
					},
				},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: requeueOnSuccess},
			wantErr:    nil,
		},
		{
			name: "SchedulingSuccessful",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*workloadv1alpha1.KubernetesApplication) = *(kubeApp())
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				scheduler: &mockScheduler{mockSchedule: newMockscheduleFn(reconcile.Result{Requeue: false})},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		{
			name: "SchedulingFailed",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*workloadv1alpha1.KubernetesApplication) = *(kubeApp())
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(errorBoom),
				},
				scheduler: &mockScheduler{mockSchedule: newMockscheduleFn(reconcile.Result{Requeue: false})},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrapf(errorBoom, "cannot update %s %s/%s", workloadv1alpha1.KubernetesApplicationKind, namespace, name),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
