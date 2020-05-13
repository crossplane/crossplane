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

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/workload/v1alpha1"
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

	templateA = v1alpha1.KubernetesApplicationResourceTemplate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTemplateA"},
		Spec: v1alpha1.KubernetesApplicationResourceSpec{
			Target:  targetRef,
			Secrets: []corev1.LocalObjectReference{{Name: "coolSecretA"}},
		},
	}
	templateB = v1alpha1.KubernetesApplicationResourceTemplate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTemplateB"},
		Spec: v1alpha1.KubernetesApplicationResourceSpec{
			Secrets: []corev1.LocalObjectReference{{Name: "coolSecretB"}},
		},
	}

	target = &v1alpha1.KubernetesTarget{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "coolTarget"},
	}

	targetRef = &v1alpha1.KubernetesTargetReference{Name: target.GetName()}
)

type karModifier func(*v1alpha1.KubernetesApplicationResource)

func karWithState(s v1alpha1.KubernetesApplicationResourceState) karModifier {
	return func(r *v1alpha1.KubernetesApplicationResource) { r.Status.State = s }
}

func kar(rm ...karModifier) *v1alpha1.KubernetesApplicationResource {
	k := &v1alpha1.KubernetesApplicationResource{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   objectMeta.GetNamespace(),
			Name:        templateA.GetName(),
			Labels:      templateA.GetLabels(),
			Annotations: templateA.GetAnnotations(),
			OwnerReferences: []metav1.OwnerReference{
				*(metav1.NewControllerRef(kubeApp(withUID(uid)), v1alpha1.KubernetesApplicationGroupVersionKind)),
			},
		},
		Spec: templateA.Spec,
		Status: v1alpha1.KubernetesApplicationResourceStatus{
			State: v1alpha1.KubernetesApplicationResourceStateScheduled,
		},
	}

	for _, m := range rm {
		m(k)
	}

	return k
}

type kubeAppModifier func(*v1alpha1.KubernetesApplication)

func withConditions(c ...runtimev1alpha1.Condition) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.Status.SetConditions(c...) }
}

func withDeletionTimestamp(t *metav1.Time) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) { r.DeletionTimestamp = t }
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

func withTarget(name string) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.Spec.Target = &v1alpha1.KubernetesTargetReference{Name: name}
	}
}

func withUID(u types.UID) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.UID = u
	}
}

func withTemplates(t ...v1alpha1.KubernetesApplicationResourceTemplate) kubeAppModifier {
	return func(r *v1alpha1.KubernetesApplication) {
		r.Spec.ResourceTemplates = t
	}
}

func kubeApp(rm ...kubeAppModifier) *v1alpha1.KubernetesApplication {
	r := &v1alpha1.KubernetesApplication{ObjectMeta: objectMeta}

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
					Spec: v1alpha1.KubernetesApplicationSpec{
						Target: targetRef,
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
					Spec: v1alpha1.KubernetesApplicationSpec{
						Target: targetRef,
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
		name      string
		syncer    syncer
		app       *v1alpha1.KubernetesApplication
		wantState v1alpha1.KubernetesApplicationState
		wantErr   error
	}{
		{
			name: "NoResourcesSubmitted",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, nil)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app:       kubeApp(withTemplates(templateA)),
			wantState: v1alpha1.KubernetesApplicationStateScheduled,
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
			app:       kubeApp(withTemplates(templateA, templateB)),
			wantState: v1alpha1.KubernetesApplicationStatePartial,
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
			app:       kubeApp(withTemplates(templateA, templateB)),
			wantState: v1alpha1.KubernetesApplicationStateSubmitted,
		},
		{
			name: "GarbageCollectionFailed",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, nil)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(errorBoom)},
			},
			app:       kubeApp(withTemplates(templateA)),
			wantState: v1alpha1.KubernetesApplicationStateFailed,
			wantErr:   errors.Wrap(errorBoom, errGarbageCollect),
		},
		{
			name: "SyncApplicationResourceFailedSingle",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, errorBoom)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app:       kubeApp(withTemplates(templateA)),
			wantState: v1alpha1.KubernetesApplicationStateFailed,
			wantErr:   errors.Wrap(condenseErrors([]error{errorBoom}), errSyncTemplate),
		},
		{
			name: "SyncApplicationResourceFailedMultiple",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, errorBoom)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app:       kubeApp(withTemplates(templateA, templateB)),
			wantState: v1alpha1.KubernetesApplicationStateFailed,
			wantErr:   errors.Wrap(condenseErrors([]error{errorBoom, errorBoom}), errSyncTemplate),
		},
		{
			name: "SyncApplicationResourceFailedPartial",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: func(_ context.Context, r *v1alpha1.KubernetesApplicationResource) (bool, error) {
					if r.Name == "coolTemplateA" {
						return false, errorBoom
					}
					return true, nil
				}},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app:       kubeApp(withTemplates(templateA, templateB)),
			wantState: v1alpha1.KubernetesApplicationStatePartial,
			wantErr:   errors.Wrap(condenseErrors([]error{errorBoom}), errSyncTemplate),
		},
		{
			name: "ApplicationDeletedDoNotSync",
			syncer: &localCluster{
				ar: &mockARSyncer{mockSync: newMockARSyncFn(false, errorBoom)},
				gc: &mockGarbageCollector{mockProcess: newMockProcessFn(nil)},
			},
			app:       kubeApp(withDeletionTimestamp(&metav1.Time{Time: time.Now()})),
			wantState: v1alpha1.KubernetesApplicationStateDeleted,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotErr := tc.syncer.sync(ctx, tc.app)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.sd.Sync(...): -want, +got:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantState, gotState); diff != "" {
				t.Errorf("tc.sd.Sync(...): -want, +got:\n%s", diff)
			}
		})
	}
}

type mockSyncFn func(ctx context.Context, app *v1alpha1.KubernetesApplication) (v1alpha1.KubernetesApplicationState, error)

func newMockSyncFn(s v1alpha1.KubernetesApplicationState, submit bool, e error) mockSyncFn {
	return func(_ context.Context, a *v1alpha1.KubernetesApplication) (v1alpha1.KubernetesApplicationState, error) {
		if meta.WasDeleted(a) {
			return v1alpha1.KubernetesApplicationStateDeleted, e
		}
		if submit {
			a.Status.SubmittedResources = a.Status.DesiredResources
		}
		return s, e
	}
}

type mockSyncer struct {
	mockSync mockSyncFn
}

func (sd *mockSyncer) sync(ctx context.Context, app *v1alpha1.KubernetesApplication) (v1alpha1.KubernetesApplicationState, error) {
	return sd.mockSync(ctx, app)
}

func TestReconcile(t *testing.T) {
	delTime := time.Now()

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
				log: logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    nil,
		},
		{
			name: "FailedToGetExtantKubernetesApplication",
			rec: &Reconciler{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
				log:  logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{Requeue: false},
			wantErr:    errors.Wrapf(errorBoom, "cannot get %s %s/%s", v1alpha1.KubernetesApplicationKind, namespace, name),
		},
		{
			name: "ApplicationSyncedSuccessfully",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplication) = *(kubeApp(withDesiredResources(3)))
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplication)

						want := kubeApp(
							withConditions(runtimev1alpha1.ReconcileSuccess()),
							withState(v1alpha1.KubernetesApplicationStateSubmitted),
							withDesiredResources(3),
							withSubmittedResources(3),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				local: &mockSyncer{mockSync: newMockSyncFn(v1alpha1.KubernetesApplicationStateSubmitted, true, nil)},
				log:   logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: longWait},
		},
		{
			name: "ApplicationDeletedDoNotSync",
			rec: &Reconciler{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						*obj.(*v1alpha1.KubernetesApplication) = *(kubeApp(
							withDesiredResources(3),
							withState(v1alpha1.KubernetesApplicationStateSubmitted),
							withDeletionTimestamp(&metav1.Time{Time: delTime}),
						))
						return nil
					},
					MockStatusUpdate: func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
						got := obj.(*v1alpha1.KubernetesApplication)

						want := kubeApp(
							withConditions(runtimev1alpha1.ReconcileSuccess()),
							withDeletionTimestamp(&metav1.Time{Time: delTime}),
							withState(v1alpha1.KubernetesApplicationStateDeleted),
							withDesiredResources(3),
						)

						if diff := cmp.Diff(want, got); diff != "" {
							return errors.Errorf("MockUpdate: -want, +got: %s", diff)
						}

						return nil
					},
				},
				local: &mockSyncer{mockSync: newMockSyncFn(v1alpha1.KubernetesApplicationStateDeleted, true, nil)},
				log:   logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: longWait},
		},
		{
			name: "ApplicationSyncFailure",
			rec: &Reconciler{
				kube:  &test.MockClient{MockGet: test.NewMockGetFn(nil), MockStatusUpdate: test.NewMockStatusUpdateFn(errorBoom)},
				local: &mockSyncer{mockSync: newMockSyncFn(v1alpha1.KubernetesApplicationStateFailed, false, errorBoom)},
				log:   logging.NewNopLogger(),
			},
			req:        reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}},
			wantResult: reconcile.Result{RequeueAfter: shortWait},
			wantErr:    errors.Wrapf(errorBoom, "cannot update status %s %s/%s", v1alpha1.KubernetesApplicationKind, namespace, name),
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
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						ref := metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)
						m := objectMeta.DeepCopy()
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
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						ref := metav1.NewControllerRef(kubeApp(), v1alpha1.KubernetesApplicationGroupVersionKind)
						m := objectMeta.DeepCopy()
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
				withConditions(runtimev1alpha1.ReconcileError(errorBoom)),
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
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
						*obj.(*v1alpha1.KubernetesApplicationResourceList) = v1alpha1.KubernetesApplicationResourceList{
							Items: []v1alpha1.KubernetesApplicationResource{{ObjectMeta: objectMeta}},
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
					MockList: func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
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
				t.Errorf("app: -want, +got:\n%s", diff)
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
						r := kar(karWithState(v1alpha1.KubernetesApplicationResourceStateSubmitted))
						r.SetOwnerReferences([]metav1.OwnerReference{})

						*obj.(*v1alpha1.KubernetesApplicationResource) = *r
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
			},
			template:      kar(),
			wantSubmitted: true,
		},
		{
			name: "ExistingResourceIsNotControllable",
			ar: &applicationResourceClient{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ types.NamespacedName, obj runtime.Object) error {
						r := kar()
						truePtr := true
						r.SetOwnerReferences([]metav1.OwnerReference{
							{
								Controller: &truePtr,
								UID:        types.UID("some-other-uid"),
							},
						})

						*obj.(*v1alpha1.KubernetesApplicationResource) = *r
						return nil
					},
				},
			},
			template: kar(),
			wantErr:  errors.WithStack(errors.Errorf("cannot sync %s: existing object is not controlled by UID \"%s\"", v1alpha1.KubernetesApplicationResourceKind, uid)),
		},
		{

			name: "CreateOrUpdateFailed",
			ar: &applicationResourceClient{
				kube: &test.MockClient{MockGet: test.NewMockGetFn(errorBoom)},
			},
			template: kar(),
			wantErr:  errors.Wrapf(errorBoom, "cannot sync %s: cannot get object", v1alpha1.KubernetesApplicationResourceKind),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotSubmitted, gotErr := tc.ar.sync(ctx, tc.template)

			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("tc.ar.sync(...): want error != got error:\n%s", diff)
			}

			if diff := cmp.Diff(tc.wantSubmitted, gotSubmitted); diff != "" {
				t.Errorf("tc.ar.Sync(...): -want, +got:\n%s", diff)
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
			app:      kubeApp(withTarget(target.GetName())),
			template: &templateA,
			want:     kar(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderTemplate(tc.app, tc.template)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("renderTemplate(...): -want, +got:\n%s", diff)
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
			want: objectMeta.GetName(),
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
				t.Errorf("getControllerName(...): -want, +got:\n%s", diff)
			}
		})
	}

}
