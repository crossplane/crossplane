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

package roles

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/grant"
)

type addFn func(item reconcile.Request)

func (fn addFn) Add(item reconcile.Request) {
	fn(item)
}

func TestEnqueueRequestForAllRevisionsInFamily(t *testing.T) {
	errBoom := errors.New("boom")
	family := "litfam"
	prName := "coolpr"

	cases := map[string]struct {
		ctx    context.Context
		obj    runtime.Object
		client client.Client
		queue  adder
	}{
		"ObjectIsNotAProviderRevision": {
			queue: addFn(func(_ reconcile.Request) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"NotInAnyFamily": {
			obj:   &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{}},
			queue: addFn(func(_ reconcile.Request) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"ListError": {
			obj: &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.LabelProviderFamily: family},
			}},
			client: &test.MockClient{
				MockList: test.NewMockListFn(errBoom),
			},
			queue: addFn(func(_ reconcile.Request) { t.Errorf("queue.Add() called unexpectedly") }),
		},
		"SuccessfulEnqueue": {
			obj: &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.LabelProviderFamily: family},
			}},
			client: &test.MockClient{
				MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
					l := obj.(*v1.ProviderRevisionList)
					l.Items = []v1.ProviderRevision{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:   prName,
								Labels: map[string]string{v1.LabelProviderFamily: family},
							},
						},
					}
					return nil
				}),
			},
			queue: addFn(func(got reconcile.Request) {
				want := reconcile.Request{NamespacedName: types.NamespacedName{Name: prName}}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("-want, +got:\n%s\n", diff)
				}
			}),
		},
	}

	for _, tc := range cases {
		e := &EnqueueRequestForAllRevisionsInFamily{client: tc.client}
		e.add(tc.ctx, tc.obj, tc.queue)
	}
}

func TestCRDEventsReconcileOwnersAndFamilies(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	old := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "old", UID: "old-uid", Labels: map[string]string{v1.LabelProviderFamily: "old-family"}}}
	oldFamily := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "old-family", UID: "old-family-uid", Labels: old.Labels}}
	next := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "next", UID: "next-uid", Labels: map[string]string{v1.LabelProviderFamily: "next-family"}}}
	nextFamily := &v1.ProviderRevision{ObjectMeta: metav1.ObjectMeta{Name: "next-family", UID: "next-family-uid", Labels: next.Labels}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(old, oldFamily, next, nextFamily).Build()
	e := &EnqueueRequestForAllRevisionsInFamily{client: c}
	h := handler.EnqueueRequestsFromMapFunc(e.ForCRD)
	previous := grant.CRDMetadata()
	previous.Name = "widgets.example.org"
	previous.OwnerReferences = []metav1.OwnerReference{meta.AsOwner(meta.TypedReferenceTo(old, v1.ProviderRevisionGroupVersionKind))}
	current := previous.DeepCopy()
	current.OwnerReferences = []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(next, v1.ProviderRevisionGroupVersionKind))}
	for name, tc := range map[string]struct {
		event func(context.Context, workqueue.TypedRateLimitingInterface[reconcile.Request])
		want  []string
	}{
		"Create": {event: func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			h.Create(ctx, event.CreateEvent{Object: current}, q)
		}, want: []string{"next", "next-family"}},
		"Handoff": {event: func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			h.Update(ctx, event.UpdateEvent{ObjectOld: previous, ObjectNew: current}, q)
		}, want: []string{"next", "next-family", "old", "old-family"}},
		"Delete": {event: func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			h.Delete(ctx, event.DeleteEvent{Object: previous}, q)
		}, want: []string{"old", "old-family"}},
	} {
		t.Run(name, func(t *testing.T) {
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()
			tc.event(context.Background(), q)
			var got []string
			for q.Len() > 0 {
				req, _ := q.Get()
				got = append(got, req.Name)
				q.Done(req)
			}
			sort.Strings(got)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("requests (-want +got):\n%s", diff)
			}
		})
	}
}
