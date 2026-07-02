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

// Package dependency tracks the resources composite resources (XRs) depend on,
// so that a change to one can be mapped back to the XRs that depend on it.
package dependency

import (
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// A Reference is a resource an XR depends on. An XR depends on a resource
// because it composes it, or because a composition function required it. A
// reference either matches a single resource by name, or a set of resources by
// label. A reference with no name and no labels matches every resource of its
// GVK. For a label reference an empty Namespace matches resources in any
// namespace; for a name reference it addresses a cluster-scoped resource.
type Reference struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
	Labels    map[string]string
}

// A Requirement is a resource a composition function required. It records the
// pipeline step and the requirement name the function used, so the resource can
// be seeded back into the function's request - under the same name - on the next
// reconcile, letting the function resolve it in a single call.
type Requirement struct {
	Step      string
	Name      string
	Reference Reference
}

// A Tracker records what each XR depends on, and maps a changed object back to
// the XRs that depend on it. Implementations must be safe for concurrent use.
type Tracker interface {
	// Track records the resources an XR depends on: those it composes, and
	// those its functions required. It replaces any references recorded by a
	// previous call.
	Track(xr client.ObjectKey, composed []Reference, required []Requirement)

	// Forget drops all references recorded for an XR.
	Forget(xr client.ObjectKey)

	// Requirements returns the resources a pipeline step required when the XR
	// was last tracked.
	Requirements(xr client.ObjectKey, step string) []Requirement

	// Dependants returns the XRs that depend on the supplied object.
	Dependants(obj client.Object) []client.ObjectKey

	// GVKs returns the GVK of every tracked reference - i.e. the kinds that
	// must be watched.
	GVKs() []schema.GroupVersionKind
}

// A nameRef indexes references that match a single resource by name.
type nameRef struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

// A labelScope buckets references that match by label. Their selectors must be
// tested against a changed object's labels, so they're grouped by the GVK and
// namespace they apply to.
type labelScope struct {
	gvk       schema.GroupVersionKind
	namespace string
}

// InMemory is an in-memory Tracker.
type InMemory struct {
	mu sync.RWMutex

	// The references recorded for each XR, kept so Track can replace them and
	// Forget can drop them. Composed and required are stored separately so
	// Requirements can return just what a step required.
	composed map[client.ObjectKey][]Reference
	required map[client.ObjectKey][]Requirement

	// Reverse indices from a changed object to the XRs that depend on it.
	byName  map[nameRef]map[client.ObjectKey]bool
	byLabel map[labelScope]map[client.ObjectKey][]labels.Selector

	// The number of references to each GVK. A GVK is dropped as soon as its
	// last reference goes away.
	gvks map[schema.GroupVersionKind]int
}

// NewInMemory creates an in-memory Tracker.
func NewInMemory() *InMemory {
	return &InMemory{
		composed: make(map[client.ObjectKey][]Reference),
		required: make(map[client.ObjectKey][]Requirement),
		byName:   make(map[nameRef]map[client.ObjectKey]bool),
		byLabel:  make(map[labelScope]map[client.ObjectKey][]labels.Selector),
		gvks:     make(map[schema.GroupVersionKind]int),
	}
}

// Track records the resources an XR depends on, replacing any previously
// recorded for it.
func (t *InMemory) Track(xr client.ObjectKey, composed []Reference, required []Requirement) {
	// Compile label selectors before taking the lock. SelectorFromSet sorts
	// and allocates; keeping it out of the write critical section means it
	// can't stall the Dependants readers on the hot watch-event path.
	csel := make([]labels.Selector, len(composed))
	for i, r := range composed {
		if r.Name == "" {
			csel[i] = labels.SelectorFromSet(r.Labels)
		}
	}
	rsel := make([]labels.Selector, len(required))
	for i, r := range required {
		if r.Reference.Name == "" {
			rsel[i] = labels.SelectorFromSet(r.Reference.Labels)
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// We hold the write lock across the remove and add, so readers never
	// see the intermediate state - a GVK referenced both before and after
	// this call never appears to have zero references.
	t.forget(xr)

	for i, r := range composed {
		t.add(xr, r, csel[i])
	}
	for i, r := range required {
		t.add(xr, r.Reference, rsel[i])
	}

	t.composed[xr] = composed
	t.required[xr] = required
}

// Forget drops all references recorded for an XR.
func (t *InMemory) Forget(xr client.ObjectKey) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.forget(xr)
}

// Requirements returns the resources a pipeline step required when the XR was
// last tracked. The returned requirements must not be modified.
func (t *InMemory) Requirements(xr client.ObjectKey, step string) []Requirement {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var out []Requirement
	for _, r := range t.required[xr] {
		if r.Step == step {
			out = append(out, r)
		}
	}

	return out
}

// Dependants returns the XRs that depend on the supplied object. It's called on
// every watch event, under a read lock (so calls run concurrently). A by-name
// match is O(1); a by-label match costs one selector test per XR that depends on
// the object's kind in its namespace - so a broadly-matched, high-churn kind
// with many dependent XRs is the cost to watch at scale.
func (t *InMemory) Dependants(obj client.Object) []client.ObjectKey {
	gvk := obj.GetObjectKind().GroupVersionKind()
	lbls := labels.Set(obj.GetLabels())

	t.mu.RLock()
	defer t.mu.RUnlock()

	dependant := make(map[client.ObjectKey]bool)

	// XRs that depend on this object by name. We look up the object's exact
	// namespace only. A name reference either addresses a namespaced resource
	// (matched here) or a cluster-scoped one (whose namespace is ""). Unlike a
	// label reference, an empty namespace isn't a wildcard here - you can't Get
	// a resource by name across namespaces.
	for xr := range t.byName[nameRef{gvk: gvk, namespace: obj.GetNamespace(), name: obj.GetName()}] {
		dependant[xr] = true
	}

	// XRs that depend on this object by label. A reference in the object's own
	// namespace matches, as does one that matches any namespace.
	scopes := []labelScope{{gvk: gvk, namespace: obj.GetNamespace()}}
	if obj.GetNamespace() != "" {
		scopes = append(scopes, labelScope{gvk: gvk, namespace: ""})
	}
	for _, scope := range scopes {
		for xr, selectors := range t.byLabel[scope] {
			if dependant[xr] {
				continue
			}
			for _, s := range selectors {
				if s.Matches(lbls) {
					dependant[xr] = true
					break
				}
			}
		}
	}

	out := make([]client.ObjectKey, 0, len(dependant))
	for xr := range dependant {
		out = append(out, xr)
	}

	return out
}

// GVKs returns the GVK of every tracked reference.
func (t *InMemory) GVKs() []schema.GroupVersionKind {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]schema.GroupVersionKind, 0, len(t.gvks))
	for gvk := range t.gvks {
		out = append(out, gvk)
	}

	return out
}

// add indexes a single reference for an XR, using its pre-compiled label
// selector (nil when the reference matches by name). The caller must hold the
// write lock.
func (t *InMemory) add(xr client.ObjectKey, r Reference, sel labels.Selector) {
	t.gvks[r.GVK]++

	if r.Name != "" {
		k := nameRef{gvk: r.GVK, namespace: r.Namespace, name: r.Name}
		if t.byName[k] == nil {
			t.byName[k] = make(map[client.ObjectKey]bool)
		}
		t.byName[k][xr] = true

		return
	}

	s := labelScope{gvk: r.GVK, namespace: r.Namespace}
	if t.byLabel[s] == nil {
		t.byLabel[s] = make(map[client.ObjectKey][]labels.Selector)
	}
	t.byLabel[s][xr] = append(t.byLabel[s][xr], sel)
}

// forget removes all of an XR's references from the indices. The caller must
// hold the write lock.
func (t *InMemory) forget(xr client.ObjectKey) {
	for _, r := range t.composed[xr] {
		t.remove(xr, r)
	}
	for _, r := range t.required[xr] {
		t.remove(xr, r.Reference)
	}

	delete(t.composed, xr)
	delete(t.required, xr)
}

// remove de-indexes a single reference for an XR. The caller must hold the write
// lock.
func (t *InMemory) remove(xr client.ObjectKey, r Reference) {
	if t.gvks[r.GVK]--; t.gvks[r.GVK] <= 0 {
		delete(t.gvks, r.GVK)
	}

	if r.Name != "" {
		k := nameRef{gvk: r.GVK, namespace: r.Namespace, name: r.Name}
		delete(t.byName[k], xr)
		if len(t.byName[k]) == 0 {
			delete(t.byName, k)
		}

		return
	}

	s := labelScope{gvk: r.GVK, namespace: r.Namespace}
	delete(t.byLabel[s], xr)
	if len(t.byLabel[s]) == 0 {
		delete(t.byLabel, s)
	}
}

// A NopTracker does nothing. It's the default for controllers that don't track
// dependencies, e.g. when realtime compositions are disabled.
type NopTracker struct{}

// Track does nothing.
func (NopTracker) Track(_ client.ObjectKey, _ []Reference, _ []Requirement) {}

// Forget does nothing.
func (NopTracker) Forget(_ client.ObjectKey) {}

// Requirements returns nil.
func (NopTracker) Requirements(_ client.ObjectKey, _ string) []Requirement { return nil }

// Dependants returns nil.
func (NopTracker) Dependants(_ client.Object) []client.ObjectKey { return nil }

// GVKs returns nil.
func (NopTracker) GVKs() []schema.GroupVersionKind { return nil }

// A NewTrackerFn creates a Tracker.
type NewTrackerFn func() Tracker

// DefaultNewTracker returns a new in-memory Tracker. It's the NewTrackerFn used
// outside of tests.
func DefaultNewTracker() Tracker {
	return NewInMemory()
}

// Trackers is a registry of per-controller Trackers.
type Trackers struct {
	mu         sync.Mutex
	newTracker NewTrackerFn
	trackers   map[string]Tracker
}

// NewTrackers returns a registry of per-controller Trackers, each built by the
// supplied NewTrackerFn.
func NewTrackers(fn NewTrackerFn) *Trackers {
	return &Trackers{
		newTracker: fn,
		trackers:   make(map[string]Tracker),
	}
}

// Get returns the named controller's Tracker, creating it if necessary.
func (r *Trackers) Get(controller string) Tracker {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.trackers[controller]
	if !ok {
		t = r.newTracker()
		r.trackers[controller] = t
	}

	return t
}

// Delete removes the named controller's Tracker.
func (r *Trackers) Delete(controller string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.trackers, controller)
}
