/*
Copyright 2026 The Crossplane Authors.

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

package watchoperation

import (
	"context"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
)

// The handler reads the WatchOperation on every event, so a selector or
// namespace change applies without restarting the watch. Regression test for
// the stale-selector bug: a handler built while the selector was track=old
// kept dropping track=new resources after the selector changed.
func TestWatchedResourceHandlerFollowsSpecChanges(t *testing.T) {
	key := types.NamespacedName{Name: "selector-update-repro"}

	current := &v1alpha1.WatchOperation{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name},
		Spec: v1alpha1.WatchOperationSpec{
			Watch: v1alpha1.WatchSpec{
				APIVersion:  "v1",
				Kind:        "ConfigMap",
				Namespace:   "default",
				MatchLabels: map[string]string{"track": "old"},
			},
		},
	}

	kube := &test.MockClient{
		MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
			current.DeepCopyInto(obj.(*v1alpha1.WatchOperation))
			return nil
		}),
	}

	h := NewWatchedResourceHandler(kube, key)

	cm := func(name, namespace, track string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("v1")
		u.SetKind("ConfigMap")
		u.SetName(name)
		u.SetNamespace(namespace)
		u.SetLabels(map[string]string{"track": track})
		return u
	}

	enqueued := func(obj *unstructured.Unstructured) bool {
		q := &recordingQueue{}
		h.Create(context.Background(), event.CreateEvent{Object: obj}, q)
		return len(q.names) > 0
	}

	// Initial selector: only track=old in default matches.
	if !enqueued(cm("selector-old", "default", "old")) {
		t.Error("old-labelled resource should enqueue under the initial selector")
	}
	if enqueued(cm("selector-new", "default", "new")) {
		t.Error("new-labelled resource should not enqueue under the initial selector")
	}

	// Selector changes to track=new: the same handler must follow it.
	current.Spec.Watch.MatchLabels = map[string]string{"track": "new"}
	if !enqueued(cm("selector-new", "default", "new")) {
		t.Error("new-labelled resource should enqueue after the selector change")
	}
	if enqueued(cm("selector-old", "default", "old")) {
		t.Error("old-labelled resource should not enqueue after the selector change")
	}

	// Namespace changes too: default-namespace resources stop matching.
	current.Spec.Watch.Namespace = "other"
	if enqueued(cm("selector-new", "default", "new")) {
		t.Error("default-namespace resource should not enqueue after the namespace change")
	}
	if !enqueued(cm("selector-new", "other", "new")) {
		t.Error("other-namespace resource should enqueue after the namespace change")
	}
}

func TestWatchedResourceHandlerDropsOnGetError(t *testing.T) {
	kube := &test.MockClient{
		MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "watchoperations"}, "gone")),
	}
	h := NewWatchedResourceHandler(kube, types.NamespacedName{Name: "gone"})

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName("selector-new")
	u.SetNamespace("default")

	q := &recordingQueue{}
	h.Create(context.Background(), event.CreateEvent{Object: u}, q)
	if len(q.names) > 0 {
		t.Errorf("deleted WatchOperation should enqueue nothing, got %v", q.names)
	}
}

type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[reconcile.Request]

	names []string
}

func (q *recordingQueue) Add(r reconcile.Request) {
	q.names = append(q.names, r.Name)
}
