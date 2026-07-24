/*
Copyright 2025 The Crossplane Authors.

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

package runtime

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

func TestEnqueueProviderRevisionsForMRDs(t *testing.T) {
	revisionName := "provider-foo-1234"

	mrd := func(state extv1alpha1.ManagedResourceDefinitionState, owner *metav1.OwnerReference) *extv1alpha1.ManagedResourceDefinition {
		m := &extv1alpha1.ManagedResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "buckets.example.org"},
			Spec:       extv1alpha1.ManagedResourceDefinitionSpec{State: state},
		}
		if owner != nil {
			m.OwnerReferences = []metav1.OwnerReference{*owner}
		}

		return m
	}

	controller := &metav1.OwnerReference{
		APIVersion: v1.SchemeGroupVersion.String(),
		Kind:       v1.ProviderRevisionKind,
		Name:       revisionName,
		UID:        types.UID("some-uid"),
		Controller: ptr.To(true),
	}

	cases := map[string]struct {
		reason string
		obj    client.Object
		want   []reconcile.Request
	}{
		"ActiveMRDControlledByProviderRevision": {
			reason: "An active MRD controlled by a provider revision should enqueue that revision.",
			obj:    mrd(extv1alpha1.ManagedResourceDefinitionActive, controller),
			want:   []reconcile.Request{{NamespacedName: types.NamespacedName{Name: revisionName}}},
		},
		"InactiveMRD": {
			reason: "An inactive MRD should not enqueue anything.",
			obj:    mrd(extv1alpha1.ManagedResourceDefinitionInactive, controller),
			want:   nil,
		},
		"NoController": {
			reason: "An active MRD without a controller owner should not enqueue anything.",
			obj:    mrd(extv1alpha1.ManagedResourceDefinitionActive, nil),
			want:   nil,
		},
		"NonProviderRevisionController": {
			reason: "An active MRD controlled by something other than a provider revision should not enqueue anything.",
			obj: mrd(extv1alpha1.ManagedResourceDefinitionActive, &metav1.OwnerReference{
				APIVersion: "example.org/v1",
				Kind:       "Composite",
				Name:       "some-owner",
				UID:        types.UID("some-uid"),
				Controller: ptr.To(true),
			}),
			want: nil,
		},
		"ProviderRevisionKindInOtherGroup": {
			reason: "An active MRD controlled by a ProviderRevision kind in another API group should not enqueue anything.",
			obj: mrd(extv1alpha1.ManagedResourceDefinitionActive, &metav1.OwnerReference{
				APIVersion: "example.org/v1",
				Kind:       v1.ProviderRevisionKind,
				Name:       "some-owner",
				UID:        types.UID("some-uid"),
				Controller: ptr.To(true),
			}),
			want: nil,
		},
		"NotAnMRD": {
			reason: "An object that is not an MRD should not enqueue anything.",
			obj:    &v1.ProviderRevision{},
			want:   nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
			defer q.ShutDown()
			EnqueueProviderRevisionsForMRDs(logging.NewNopLogger()).Create(context.Background(), event.CreateEvent{Object: tc.obj}, q)

			var got []reconcile.Request
			for q.Len() > 0 {
				i, _ := q.Get()
				got = append(got, i)
				q.Done(i)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nEnqueueProviderRevisionsForMRDs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMRDActivatedPredicate(t *testing.T) {
	mrd := func(state extv1alpha1.ManagedResourceDefinitionState) *extv1alpha1.ManagedResourceDefinition {
		return &extv1alpha1.ManagedResourceDefinition{
			Spec: extv1alpha1.ManagedResourceDefinitionSpec{State: state},
		}
	}

	p := mrdActivated()

	cases := map[string]struct {
		reason string
		got    bool
		want   bool
	}{
		"CreateActive": {
			reason: "Creating an active MRD should pass, to replay activations after an informer restart.",
			got:    p.Create(event.CreateEvent{Object: mrd(extv1alpha1.ManagedResourceDefinitionActive)}),
			want:   true,
		},
		"CreateInactive": {
			reason: "Creating an inactive MRD should not pass.",
			got:    p.Create(event.CreateEvent{Object: mrd(extv1alpha1.ManagedResourceDefinitionInactive)}),
			want:   false,
		},
		"UpdateActivated": {
			reason: "An MRD transitioning from inactive to active should pass.",
			got: p.Update(event.UpdateEvent{
				ObjectOld: mrd(extv1alpha1.ManagedResourceDefinitionInactive),
				ObjectNew: mrd(extv1alpha1.ManagedResourceDefinitionActive),
			}),
			want: true,
		},
		"UpdateStillActive": {
			reason: "An update to an already active MRD should not pass, to avoid reconciles on status churn.",
			got: p.Update(event.UpdateEvent{
				ObjectOld: mrd(extv1alpha1.ManagedResourceDefinitionActive),
				ObjectNew: mrd(extv1alpha1.ManagedResourceDefinitionActive),
			}),
			want: false,
		},
		"UpdateStillInactive": {
			reason: "An update to an inactive MRD should not pass.",
			got: p.Update(event.UpdateEvent{
				ObjectOld: mrd(extv1alpha1.ManagedResourceDefinitionInactive),
				ObjectNew: mrd(extv1alpha1.ManagedResourceDefinitionInactive),
			}),
			want: false,
		},
		"Delete": {
			reason: "Deleting an MRD should not pass. MRD activation is one-way.",
			got:    p.Delete(event.DeleteEvent{Object: mrd(extv1alpha1.ManagedResourceDefinitionActive)}),
			want:   false,
		},
		"Generic": {
			reason: "Generic events should not pass.",
			got:    p.Generic(event.GenericEvent{Object: mrd(extv1alpha1.ManagedResourceDefinitionActive)}),
			want:   false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, tc.got); diff != "" {
				t.Errorf("\n%s\nmrdActivated(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
