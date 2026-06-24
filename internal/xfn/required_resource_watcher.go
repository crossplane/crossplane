/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"

	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// A RequiredResourceWatcher starts watches for the resources a composite
// resource's (XR's) function pipeline requires. The composite resource (XR)
// controller calls a function pipeline both when the XR or its composed
// resources change and - if this watcher is wired up - when a resource the
// pipeline requires changes.
//
// The FetchingFunctionRunner calls a RequiredResourceWatcher once a function's
// requirements have stabilized, passing the XR the function ran for and the
// kinds of resource it required. The watcher is responsible for translating
// those into watches on the right XR controller. It dispatches on the XR's
// GroupVersionKind, because the runner is shared by every XR controller but each
// controller watches resources independently.
type RequiredResourceWatcher interface {
	// WatchRequiredResources ensures the controller for the supplied composite
	// resource (XR) is watching the supplied required resource kinds. Implementations
	// must be safe to call from multiple goroutines, and idempotent - the runner
	// calls it on every reconcile that resolves requirements.
	WatchRequiredResources(ctx context.Context, xr schema.GroupVersionKind, required []schema.GroupVersionKind) error
}

// A RequiredResourceWatcherFn is a function that satisfies RequiredResourceWatcher.
type RequiredResourceWatcherFn func(ctx context.Context, xr schema.GroupVersionKind, required []schema.GroupVersionKind) error

// WatchRequiredResources calls fn.
func (fn RequiredResourceWatcherFn) WatchRequiredResources(ctx context.Context, xr schema.GroupVersionKind, required []schema.GroupVersionKind) error {
	return fn(ctx, xr, required)
}

// A NopRequiredResourceWatcher does nothing. It's the default - required
// resource watches are only started when the watcher is explicitly registered
// for an XR's kind, behind a feature flag.
type NopRequiredResourceWatcher struct{}

// WatchRequiredResources does nothing.
func (NopRequiredResourceWatcher) WatchRequiredResources(_ context.Context, _ schema.GroupVersionKind, _ []schema.GroupVersionKind) error {
	return nil
}

// A RequiredResourceWatcherRegistry dispatches to a per-XR-kind
// RequiredResourceWatcher. The FetchingFunctionRunner is shared by every XR
// controller, so it holds a single registry. Each XR controller registers a
// watcher for its own kind when it starts, and deregisters it when it stops.
//
// An XR kind with no registered watcher is silently ignored. This is the common
// case: required resource watches are an opt-in alpha feature, so most XR kinds
// never register a watcher.
//
// A RequiredResourceWatcherRegistry is safe for concurrent use, and satisfies
// RequiredResourceWatcher itself.
type RequiredResourceWatcherRegistry struct {
	mx       sync.RWMutex
	watchers map[schema.GroupVersionKind]RequiredResourceWatcher
}

// A RequiredResourceWatcherRegistry is itself a RequiredResourceWatcher.
var _ RequiredResourceWatcher = &RequiredResourceWatcherRegistry{}

// NewRequiredResourceWatcherRegistry creates a RequiredResourceWatcherRegistry.
func NewRequiredResourceWatcherRegistry() *RequiredResourceWatcherRegistry {
	return &RequiredResourceWatcherRegistry{watchers: make(map[schema.GroupVersionKind]RequiredResourceWatcher)}
}

// Register the supplied watcher for the supplied XR kind. It replaces any
// watcher already registered for the kind.
func (r *RequiredResourceWatcherRegistry) Register(xr schema.GroupVersionKind, w RequiredResourceWatcher) {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.watchers[xr] = w
}

// Deregister the watcher for the supplied XR kind, if any. It's called when an
// XR controller stops, so we don't dispatch to a watcher for a controller that's
// no longer running.
func (r *RequiredResourceWatcherRegistry) Deregister(xr schema.GroupVersionKind) {
	r.mx.Lock()
	defer r.mx.Unlock()
	delete(r.watchers, xr)
}

// WatchRequiredResources dispatches to the watcher registered for the supplied
// XR kind. It's a no-op if no watcher is registered for the kind.
func (r *RequiredResourceWatcherRegistry) WatchRequiredResources(ctx context.Context, xr schema.GroupVersionKind, required []schema.GroupVersionKind) error {
	r.mx.RLock()
	w, ok := r.watchers[xr]
	r.mx.RUnlock()

	if !ok {
		return nil
	}
	return w.WatchRequiredResources(ctx, xr, required)
}

// requiredGVKs returns the distinct GroupVersionKinds of the resources the
// supplied requirements select. It ignores schema requirements, which don't
// correspond to watchable resources.
func requiredGVKs(r *fnv1.Requirements) []schema.GroupVersionKind {
	seen := make(map[schema.GroupVersionKind]bool)
	gvks := make([]schema.GroupVersionKind, 0)

	add := func(s *fnv1.ResourceSelector) {
		gvk := schema.FromAPIVersionAndKind(s.GetApiVersion(), s.GetKind())
		if gvk.Empty() || seen[gvk] {
			return
		}
		seen[gvk] = true
		gvks = append(gvks, gvk)
	}

	//nolint:staticcheck // We must account for the deprecated extra_resources field.
	for _, s := range r.GetExtraResources() {
		add(s)
	}
	for _, s := range r.GetResources() {
		add(s)
	}

	return gvks
}
