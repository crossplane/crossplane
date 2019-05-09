/*
Copyright 2018 The Crossplane Authors.

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

package application

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

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/core"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "coolNamespace"
	name      = "coolApp"
	uid       = types.UID("definitely-a-uuid")
)

var (
	errorBoom = errors.New("boom")
	meta      = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
	ctx       = context.Background()

	templateA = v1alpha1.KubernetesApplicationResourceTemplate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTemplateA"},
		Spec: v1alpha1.KubernetesApplicationResourceSpec{
			Secrets: []corev1.LocalObjectReference{{Name: "coolSecretA"}},
		},
	}
	templateB = v1alpha1.KubernetesApplicationResourceTemplate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTemplateB"},
		Spec: v1alpha1.KubernetesApplicationResourceSpec{
			Secrets: []corev1.LocalObjectReference{{Name: "coolSecretB"}},
		},
	}

	resourceA = &v1alpha1.KubernetesApplicationResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   meta.GetNamespace(),
			Name:        templateA.GetName(),
			Labels:      templateA.GetLabels(),
			Annotations: templateA.GetAnnotations(),
			OwnerReferences: []metav1.OwnerReference{
				*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
			},
		},
		Spec: templateA.Spec,
		Status: v1alpha1.KubernetesApplicationResourceStatus{
			State:   v1alpha1.KubernetesApplicationResourceStateScheduled,
			Cluster: cluster.ObjectReference(),
		},
	}

	cluster = &computev1alpha1.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolCluster"},
	}
)

// Frequently used conditions.
var (
	deleting = corev1alpha1.Condition{Type: corev1alpha1.Deleting, Status: corev1.ConditionTrue}
	ready    = corev1alpha1.Condition{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}
	pending  = corev1alpha1.Condition{Type: corev1alpha1.Pending, Status: corev1.ConditionTrue}
)

type kubeAppModifier func(*v1alpha1.KubernetesApplication)

func withFinalizers(f ...string) kubeAppModifier {
	if f == nil {
		f = []string{}
	}
	return func(r *v1alpha1.KubernetesApplication) { r.ObjectMeta.Finalizers = f }
}

func withDeletionTimestamp(t time.Time) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t}
	}
}

func withConditions(c ...corev1alpha1.Condition) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.Status.ConditionedStatus.Conditions = c }
}

func withState(s v1alpha1.KubernetesApplicationState) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.Status.State = s }
}

func withDesiredResources(i int) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.Status.DesiredResources = i }
}

func withSubmittedResources(i int) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.Status.SubmittedResources = i }
}

func withCluster(c *corev1.ObjectReference) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.Status.Cluster = c
	}
}

func withTemplates(t ...v1alpha1.KubernetesApplicationResourceTemplate) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.Spec.ResourceTemplates = t
	}
}

func kubeApp(rm ...kubeAppModifier) *v1alpha1.KubernetesApplication {
	r := &v1alpha1.KubernetesApplication{ObjectMeta: meta}

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
			name: "ScheduledCluster",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplication{
					Status: v1alpha1.KubernetesApplicationStatus{
						Cluster: cluster.ObjectReference(),
					},
				},
			},
			want: true,
		},
		{
			name: "UnscheduledCluster",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplication{},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplication",
			event: event.CreateEvent{
				Object: &v1alpha1.KubernetesApplicationResource{},
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
			name: "ScheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplication{
					Status: v1alpha1.KubernetesApplicationStatus{
						Cluster: cluster.ObjectReference(),
					},
				},
			},
			want: true,
		},
		{
			name: "UnscheduledCluster",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplication{},
			},
			want: false,
		},
		{
			name: "NotAKubernetesApplication",
			event: event.UpdateEvent{
				ObjectNew: &v1alpha1.KubernetesApplicationResource{},
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

type mockARSyncFn func(ctx context.Context, template *v1alpha1.KubernetesApplicationResource) (bool, error)

func newMockARSyncFn(submitted bool, err error) mockARSyncFn {
	return func(_ context.Context, _ *v1alpha1.KubernetesApplicationResource) (bool, error) {
		return submitted, err
	}
}

type mockARSyncer struct {
	mockSync mockARSyncFn
}

func (tp *mockARSyncer) sync(ctx context.Context, template *v1alpha1.KubernetesApplicationResource) (bool, error) {
	return tp.mockSync(ctx, template)
}

type mockProcessFn func(ctx context.Context, app *v1alpha1.KubernetesApplication) error

func newMockProcessFn(err error) mockProcessFn {
	return func(ctx context.Context, app *v1alpha1.KubernetesApplication) error { return err }
}

type mockGarbageCollector struct {
	mockProcess mockProcessFn
}

func (gc *mockGarbageCollector) process(ctx context.Context, app *v1alpha1.KubernetesApplication) error {
	return gc.mockProcess(ctx, app)
}

func TestSync(t *testing.T) {
	cases := []struct {
		name       string
		syncer     syncer
		app        *v1alpha1.KubernetesApplication
		wantApp    *v1alpha1.KubernetesApplication
		wantResult reconcile.Result
	}{
		{
			name: "NoResourcesSubmitted",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, nil)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app: kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(
				withTemplates(templateA),
				withFinalizers(finalizerName),
				withState(v1alpha1.KubernetesApplicationStateScheduled),
				withConditions(pending),
				withDesiredResources(1),
				withSubmittedResources(0),
			),
			wantResult: core.RequeueSoon,
		},
		{
			name: "PartialResourcesSubmitted",
			syncer: &localCluster{
				ar: &mockARSyncer{
					mockSync: func(_ context.Context, template *v1alpha1.KubernetesApplicationResource) (bool, error) {
						// Simulate one resource in the submitted state. We're
						// called once for each template, so we set this to 1
						// each time.
						if template.GetName() == templateA.GetName() {
							return true, nil
						}
						return false, nil
					},
				},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app: kubeApp(withTemplates(templateA, templateB)),
			wantApp: kubeApp(
				withTemplates(templateA, templateB),
				withFinalizers(finalizerName),
				withState(v1alpha1.KubernetesApplicationStatePartial),
				withConditions(pending),
				withDesiredResources(2),
				withSubmittedResources(1),
			),
			wantResult: core.RequeueSoon,
		},
		{
			name: "AllResourcesSubmitted",
			syncer: &localCluster{
				ar: &mockARSyncer{
					mockSync: func(_ context.Context, _ *v1alpha1.KubernetesApplicationResource) (bool, error) {
						// Simulate all resources in the submitted state.
						return true, nil
					},
				},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app: kubeApp(withTemplates(templateA, templateB)),
			wantApp: kubeApp(
				withTemplates(templateA, templateB),
				withFinalizers(finalizerName),
				withState(v1alpha1.KubernetesApplicationStateSubmitted),
				withConditions(ready),
				withDesiredResources(2),
				withSubmittedResources(2),
			),
			wantResult: reconcile.Result{Requeue: false},
		},
		{
			name: "GarbageCollectionFailed",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, nil)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(errorBoom)},
			},
			app: kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(
				withTemplates(templateA),
				withFinalizers(finalizerName),
				withState(v1alpha1.KubernetesApplicationStateFailed),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonGCResources,
						Message: errorBoom.Error(),
					},
				),
				withDesiredResources(1),
			),
			wantResult: reconcile.Result{Requeue: true},
		},
		{
			name: "SyncApplicationResourceFailed",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, errorBoom)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app: kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(
				withTemplates(templateA),
				withFinalizers(finalizerName),
				withState(v1alpha1.KubernetesApplicationStateFailed),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonSyncingResource,
						Message: errorBoom.Error(),
					},
				),
				withDesiredResources(1),
			),
			wantResult: reconcile.Result{Requeue: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult := tc.syncer.sync(ctx, tc.app)

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.sd.Sync(...): want != got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantApp, tc.app); diff != "" {
				t.Errorf("app: want != got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name       string
		deleter    deleter
		app        *v1alpha1.KubernetesApplication
		wantApp    *v1alpha1.KubernetesApplication
		wantResult reconcile.Result
	}{
		{
			name:    "Successful",
			deleter: &localCluster{},
			app:     kubeApp(withFinalizers(finalizerName)),
			wantApp: kubeApp(
				withFinalizers(),
				withConditions(deleting),
			),
			wantResult: reconcile.Result{Requeue: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult := tc.deleter.delete(ctx, tc.app)

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.sd.Sync(...): want != got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantApp, tc.app); diff != "" {
				t.Errorf("app: want != got:\n%s", diff)
			}
		})
	}
}

type mockSyncFn func(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result

func newMockSyncFn(r reconcile.Result) mockSyncFn {
	return func(_ context.Context, _ *v1alpha1.KubernetesApplication) reconcile.Result { return r }
}

type mockDeleteFn func(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result

func newMockDeleteFn(r reconcile.Result) mockDeleteFn {
	return func(_ context.Context, _ *v1alpha1.KubernetesApplication) reconcile.Result { return r }
}

type mockSyncDeleter struct {
	mockSync   mockSyncFn
	mockDelete mockDeleteFn
}

func (sd *mockSyncDeleter) sync(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result {
	return sd.mockSync(ctx, app)
}

func (sd *mockSyncDeleter) delete(ctx context.Context, app *v1alpha1.KubernetesApplication) reconcile.Result {
	return sd.mockDelete(ctx, app)
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, name)
					},
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
			wantErr:    errors.Wrapf(errorBoom, "cannot get %s %s/%s", v1alpha1.KubernetesApplicationKind, namespace, name),
		},
		{
			name: "ApplicationDeletedSuccessfully",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplication) = *(kubeApp(withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				local: &mockSyncDeleter{mockDelete: newMockDeleteFn(reconcile.Result{Requeue: false})},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
		},

		{
			name: "ApplicationDeleteFailure",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplication) = *(kubeApp(withDeletionTimestamp(time.Now())))
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(errorBoom),
				},
				local: &mockSyncDeleter{mockDelete: newMockDeleteFn(reconcile.Result{Requeue: false})},
			},
			req: reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},

			// Dirty, not Error, because we return the result and call update
			// at the same time and thus did not know to expect the error.
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrapf(errorBoom, "cannot update %s %s/%s", v1alpha1.KubernetesApplicationKind, namespace, name),
		},
		{
			name: "ApplicationSyncedSuccessfully",
			rec: &Reconciler{
				kube:  &test.MockClient{MockGet: test.NewMockGetFn(nil), MockUpdate: test.NewMockUpdateFn(nil)},
				local: &mockSyncDeleter{mockSync: newMockSyncFn(reconcile.Result{Requeue: false})},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
		},
		{
			name: "ApplicationSyncFailure",
			rec: &Reconciler{
				kube:  &test.MockClient{MockGet: test.NewMockGetFn(nil), MockUpdate: test.NewMockUpdateFn(errorBoom)},
				local: &mockSyncDeleter{mockSync: newMockSyncFn(reconcile.Result{Requeue: false})},
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrapf(errorBoom, "cannot update %s %s/%s", v1alpha1.KubernetesApplicationKind, namespace, name),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, gotErr := tc.rec.Reconcile(tc.req)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantResult, gotResult); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestGarbageCollect(t *testing.T) {
	cases := []struct {
		name    string
		gc      garbageCollector
		app     *v1alpha1.KubernetesApplication
		wantApp *v1alpha1.KubernetesApplication
		wantErr error
	}{
		{
			name: "SuccessfulResourceDeletion",
			gc: &applicationResourceGarbageCollector{
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ *client.ListOptions, obj runtime.Object) error {
						ref := metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)
						m := meta.DeepCopy()
						m.SetOwnerReferences([]metav1.OwnerReference{*ref})
						*obj.(*v1alpha1.KubernetesApplicationResourceList) = v1alpha1.KubernetesApplicationResourceList{
							Items: []v1alpha1.KubernetesApplicationResource{{ObjectMeta: *m}},
						}
						return nil
					},
					MockDelete: test.NewMockDeleteFn(nil),
				},
			},
			app:     kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(withTemplates(templateA)),
		},
		{
			name: "FailedResourceDeletion",
			gc: &applicationResourceGarbageCollector{
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ *client.ListOptions, obj runtime.Object) error {
						ref := metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)
						m := meta.DeepCopy()
						m.SetOwnerReferences([]metav1.OwnerReference{*ref})
						*obj.(*v1alpha1.KubernetesApplicationResourceList) = v1alpha1.KubernetesApplicationResourceList{
							Items: []v1alpha1.KubernetesApplicationResource{{ObjectMeta: *m}},
						}
						return nil
					},
					MockDelete: test.NewMockDeleteFn(errorBoom),
				},
			},
			app: kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(
				withTemplates(templateA),
				withConditions(
					corev1alpha1.Condition{
						Type:    corev1alpha1.Failed,
						Status:  corev1.ConditionTrue,
						Reason:  reasonGCResources,
						Message: errorBoom.Error(),
					},
				),
			),
		},
		{
			name: "FailedListResources",
			gc: &applicationResourceGarbageCollector{
				kube: &test.MockClient{MockList: test.NewMockListFn(errorBoom)},
			},
			app:     kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(withTemplates(templateA)),
			wantErr: errors.Wrapf(errorBoom, "cannot garbage collect %s", v1alpha1.KubernetesApplicationResourceKind),
		},
		{
			name: "ResourceNotControlledByApplication",
			gc: &applicationResourceGarbageCollector{
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ *client.ListOptions, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplicationResourceList) = v1alpha1.KubernetesApplicationResourceList{
							Items: []v1alpha1.KubernetesApplicationResource{{ObjectMeta: meta}},
						}
						return nil
					},
				},
			},
			app:     kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(withTemplates(templateA)),
		},
		{
			name: "ResourceIsTemplated",
			gc: &applicationResourceGarbageCollector{
				kube: &test.MockClient{
					MockList: func(_ context.Context, _ *client.ListOptions, obj runtime.Object) error {
						ref := metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)
						m := templateA.ObjectMeta.DeepCopy()
						m.SetOwnerReferences([]metav1.OwnerReference{*ref})
						*obj.(*v1alpha1.KubernetesApplicationResourceList) = v1alpha1.KubernetesApplicationResourceList{
							Items: []v1alpha1.KubernetesApplicationResource{{ObjectMeta: *m}},
						}
						return nil
					},
				},
			},
			app:     kubeApp(withTemplates(templateA)),
			wantApp: kubeApp(withTemplates(templateA)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := tc.gc.process(ctx, tc.app)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantApp, tc.app); diff != "" {
				t.Errorf("app: want != got:\n%s", diff)
			}
		})
	}
}

func TestSyncApplicationResource(t *testing.T) {
	cases := []struct {
		name          string
		ar            applicationResourceSyncer
		template      *v1alpha1.KubernetesApplicationResource
		wantSubmitted bool
		wantErr       error
	}{
		{
			name: "Successful",
			ar: &applicationResourceClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ types.NamespacedName, obj runtime.Object) error {
						r := resourceA.DeepCopy()
						r.Status.State = v1alpha1.KubernetesApplicationResourceStateSubmitted

						*obj.(*v1alpha1.KubernetesApplicationResource) = *r
						return nil
					},
				},
			},
			template:      resourceA,
			wantSubmitted: true,
		},
		{
			name: "ExistingResourceHasDifferentController",
			ar: &applicationResourceClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ types.NamespacedName, obj runtime.Object) error {
						r := resourceA.DeepCopy()
						r.SetOwnerReferences([]metav1.OwnerReference{})

						*obj.(*v1alpha1.KubernetesApplicationResource) = *r
						return nil
					},
				},
			},
			template: resourceA,
			wantErr: errors.WithStack(errors.Errorf("cannot sync %s: could not mutate object for update: %s %s exists and is not controlled by %s %s",
				v1alpha1.KubernetesApplicationResourceKind,
				v1alpha1.KubernetesApplicationResourceKind,
				templateA.GetName(),
				v1alpha1.KubernetesApplicationKind,
				meta.GetName(),
			)),
		},
		{

			name: "CreateOrUpdateFailed",
			ar: &applicationResourceClient{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			template: resourceA,
			wantErr:  errors.Wrapf(errorBoom, "cannot sync %s: could not get object", v1alpha1.KubernetesApplicationResourceKind),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSubmitted, gotErr := tc.ar.sync(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.ar.sync(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantSubmitted, gotSubmitted); diff != "" {
				t.Errorf("tc.ar.Sync(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	cases := []struct {
		name     string
		app      *v1alpha1.KubernetesApplication
		template *v1alpha1.KubernetesApplicationResourceTemplate
		want     *v1alpha1.KubernetesApplicationResource
	}{
		{
			name:     "Successful",
			app:      kubeApp(withCluster(cluster.ObjectReference())),
			template: &templateA,
			want:     resourceA,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTemplate(tc.app, tc.template)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("renderTemplate(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestHasSameController(t *testing.T) {
	cases := []struct {
		name string
		a    metav1.Object
		b    metav1.Object
		want bool
	}{
		{
			name: "SameController",
			a: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			b: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			want: true,
		},
		{
			name: "AHasNoController",
			a:    &v1alpha1.KubernetesApplicationResource{},
			b: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			want: false,
		},
		{
			name: "BHasNoController",
			a: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			b:    &v1alpha1.KubernetesApplicationResource{},
			want: false,
		},
		{
			name: "ControllersDiffer",
			a: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Controller: func() *bool {
								t := true
								return &t
							}(),
							UID: "imdifferent",
						},
					},
				},
			},
			b: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hasSameController(tc.a, tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("hasSameController(...): want != got:\n%s", diff)
			}
		})
	}
}

func TestGetControllerName(t *testing.T) {
	cases := []struct {
		name string
		obj  metav1.Object
		want string
	}{
		{
			name: "HasController",
			obj: &v1alpha1.KubernetesApplicationResource{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						*(metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)),
					},
				},
			},
			want: meta.GetName(),
		},
		{
			name: "HasNoController",
			obj:  &v1alpha1.KubernetesApplicationResource{},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := getControllerName(tc.obj)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("getControllerName(...): want != got:\n%s", diff)
			}
		})
	}

}
