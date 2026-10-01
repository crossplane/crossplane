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

package composite

import (
	"context"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xcrd"
)

// TestReconcileDeleteIsOrdered covers the delete branch of Reconcile end to
// end, which TestTeardown does not: it tests teardown() in isolation, and
// nothing until now checked that Reconcile honors a pending teardown by
// keeping the XR's finalizer.
//
// That gap hid a real failure. An XR whose teardown is still in progress must
// not lose its finalizer, because the finalizer is the only thing stopping
// Kubernetes cascading to the composed resources in no particular order.
func TestReconcileDeleteIsOrdered(t *testing.T) {
	now := metav1.Now()

	// first <- second: second is the only leaf.
	graph := []reference.Composed{
		{APIVersion: "example.org/v1", Kind: "Thing", Name: "xr-first", ResourceName: "first"},
		{APIVersion: "example.org/v1", Kind: "Thing", Name: "xr-second", ResourceName: "second", DependsOn: []string{"first"}},
	}

	deletingXR := func(cr *composite.Unstructured) {
		cr.SetDeletionTimestamp(&now)
		cr.SetComposedResourceReferences(graph)
	}

	cases := map[string]struct {
		reason    string
		observed  []string
		wantFinal bool // was RemoveFinalizer called?
		wantDel   []string
	}{
		"HoldsFinalizerWhileResourcesRemain": {
			reason:    "An XR whose composed resources still exist keeps its finalizer, so Kubernetes can't cascade past the ordering.",
			observed:  []string{"first", "second"},
			wantFinal: false,
			wantDel:   []string{"second"},
		},
		"DropsFinalizerOnceEverythingIsGone": {
			reason:    "Once nothing is left there is nothing to order, so the XR can finish deleting.",
			observed:  []string{},
			wantFinal: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			removed := false
			deleted := []string{}

			r := NewReconciler(&test.MockClient{
				MockGet:          WithComposite(t, NewComposite(deletingXR)),
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
			}, schema.GroupVersionKind{},
				WithCompositeFinalizer(resource.FinalizerFns{
					RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						removed = true
						return nil
					},
				}),
				WithOrderedTeardown(
					ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						out := ComposedResourceStates{}
						for _, n := range tc.observed {
							out[ResourceName(n)] = state("Thing", "xr-"+n)
						}

						return out, nil
					}),
					ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
						for n := range observed {
							deleted = append(deleted, string(n))
						}

						return nil
					}),
				),
			)

			if _, err := r.Reconcile(context.Background(), reconcile.Request{}); err != nil {
				t.Fatalf("\n%s\nReconcile(...): unexpected error: %v", tc.reason, err)
			}

			if removed != tc.wantFinal {
				t.Errorf("\n%s\nReconcile(...): RemoveFinalizer called = %v, want %v", tc.reason, removed, tc.wantFinal)
			}

			if len(deleted) != len(tc.wantDel) {
				t.Errorf("\n%s\nReconcile(...): deleted %v, want %v", tc.reason, deleted, tc.wantDel)
			}
		})
	}
}

// TestTeardownWithRealObserver exercises teardown() against the real
// ExistingComposedResourceObserver rather than a stub. TestTeardown and
// TestReconcileDeleteIsOrdered both inject a fake observer, so the seam
// between the XR's references and the observed state - which is where the
// composition resource name has to survive a round trip through an
// ObjectReference - was never covered.
func TestTeardownWithRealObserver(t *testing.T) {
	now := metav1.Now()

	xr := composite.New()
	xr.SetName("ordered")
	xr.SetNamespace("default")
	xr.SetUID("xr-uid")
	xr.SetDeletionTimestamp(&now)
	xr.SetComposedResourceReferences([]reference.Composed{
		// Shaped as core writes them for a namespaced XR: no namespace.
		{APIVersion: "nop.crossplane.io/v1alpha1", Kind: "NopResource", Name: "ordered-aaa", ResourceName: "first"},
		{APIVersion: "nop.crossplane.io/v1alpha1", Kind: "NopResource", Name: "ordered-bbb", ResourceName: "second", DependsOn: []string{"first"}},
	})

	// A client that returns the two composed resources, annotated and
	// controlled by the XR, the way Crossplane creates them.
	byName := map[string]string{"ordered-aaa": "first", "ordered-bbb": "second"}

	c := &test.MockClient{
		MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
			n, ok := byName[key.Name]
			if !ok {
				return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
			}

			cd, ok := obj.(*composed.Unstructured)
			if !ok {
				return nil
			}

			cd.SetName(key.Name)
			cd.SetNamespace(key.Namespace)
			cd.SetAnnotations(map[string]string{xcrd.AnnotationKeyCompositionResourceName: n})
			cd.SetOwnerReferences([]metav1.OwnerReference{{
				UID:        "xr-uid",
				Controller: new(true),
				APIVersion: "ordering.example.org/v1alpha1",
				Kind:       "XOrdering",
				Name:       "ordered",
			}})

			return nil
		},
	}

	deleted := []string{}

	r := &Reconciler{
		// NewReconciler would supply these; this literal bypasses it.
		log:        logging.NewNopLogger(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		observer:   NewExistingComposedResourceObserver(c, c, NewSecretConnectionDetailsFetcher(c)),
		gc: ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
			for n := range observed {
				deleted = append(deleted, string(n))
			}

			return nil
		}),
	}

	done, err := r.teardown(context.Background(), xr, r.conditions.For(xr))
	if err != nil {
		t.Fatalf("teardown(...): unexpected error: %v", err)
	}

	if done {
		t.Errorf("teardown(...): reported done while both composed resources still exist; the XR would drop its finalizer and Kubernetes would cascade")
	}

	if diff := cmp.Diff([]string{"second"}, deleted); diff != "" {
		t.Errorf("teardown(...): -want deleted, +got deleted:\n%s", diff)
	}
}

// TestTeardownWithoutGraph covers an XR whose pipeline declared no ordering.
// Composed resources are deleted with background propagation when ordering is
// enabled, so a nested XR without a graph has only its own finalizer to keep
// it around until what it composed is gone - which is what its parent's graph
// waits on. It must therefore hold the finalizer rather than cascade.
func TestTeardownWithoutGraph(t *testing.T) {
	now := metav1.Now()
	uid := types.UID("xr-uid")

	controlled := func(name string) ComposedResourceState {
		s := state("Thing", "xr-"+name)
		s.Resource.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: "example.org/v1", Kind: "XR", Name: "xr", UID: uid,
			Controller: new(true), BlockOwnerDeletion: new(true),
		}})

		return s
	}

	deleting := func(s ComposedResourceState) ComposedResourceState {
		s.Resource.SetDeletionTimestamp(&now)
		return s
	}

	cases := map[string]struct {
		reason   string
		observed ComposedResourceStates
		wantDone bool
		wantDel  []string
	}{
		"DeletesEverythingInOneWave": {
			reason:   "Without a graph there's no order to keep, so every composed resource is deleted at once - by Crossplane, while the XR holds its finalizer.",
			observed: ComposedResourceStates{"a": controlled("a"), "b": controlled("b")},
			wantDone: false,
			wantDel:  []string{"a", "b"},
		},
		"WaitsForResourcesAlreadyDeleting": {
			reason:   "A resource that's already deleting isn't asked again, but the XR keeps waiting for it to go.",
			observed: ComposedResourceStates{"a": deleting(controlled("a"))},
			wantDone: false,
		},
		"IgnoresResourcesItDoesNotControl": {
			reason:   "Kubernetes would never cascade to a resource the XR doesn't control, so waiting on one would hold the XR forever.",
			observed: ComposedResourceStates{"a": state("Thing", "someone-elses")},
			wantDone: true,
		},
		"DoneOnceEverythingIsGone": {
			reason:   "Once nothing is left the XR can finish deleting.",
			observed: ComposedResourceStates{},
			wantDone: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			deleted := []string{}

			r := NewReconciler(&test.MockClient{}, schema.GroupVersionKind{},
				WithOrderedTeardown(
					ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return tc.observed, nil
					}),
					ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
						for n := range observed {
							deleted = append(deleted, string(n))
						}

						return nil
					}),
				),
			)

			xr := composite.New()
			xr.SetUID(uid)
			xr.SetDeletionTimestamp(&now)

			// References with no dependsOn: a graph with no edges.
			refs := make([]reference.Composed, 0, len(tc.observed))
			for n := range tc.observed {
				refs = append(refs, reference.Composed{APIVersion: "example.org/v1", Kind: "Thing", Name: "xr-" + string(n), ResourceName: string(n)})
			}

			xr.SetComposedResourceReferences(refs)

			done, err := r.teardown(context.Background(), xr, r.conditions.For(xr))
			if err != nil {
				t.Fatalf("\n%s\nteardown(...): unexpected error: %v", tc.reason, err)
			}

			if done != tc.wantDone {
				t.Errorf("\n%s\nteardown(...): done = %v, want %v", tc.reason, done, tc.wantDone)
			}

			slices.Sort(deleted)
			if diff := cmp.Diff(tc.wantDel, deleted, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nteardown(...): deleted -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
