/*
Copyright 2024 The Crossplane Authors.

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

// Package watch implements a garbage collector for the composed and required
// resource watches a composite resource (XR) controller starts.
package watch

import (
	"context"
	"time"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/v2/internal/engine"
)

// A ControllerEngine can get and stop watches for a controller.
type ControllerEngine interface {
	GetWatches(name string) ([]engine.WatchID, error)
	StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	GetCached() client.Client
	GetUncached() client.Client
}

// A RequiredResourceProvider knows which kinds of resource an XR's function
// pipeline requires. The garbage collector uses it to learn which required
// resource watches are still in use. It's typically backed by the function
// runner's requirements cache.
type RequiredResourceProvider interface {
	// RequiredGVKs returns the kinds of resource required by the XR with the
	// supplied UID.
	RequiredGVKs(xrUID string) []schema.GroupVersionKind

	// RetainForKind forgets what it knows about XRs of the supplied kind whose
	// UID isn't in the supplied set of live UIDs, so it doesn't leak memory for
	// deleted XRs.
	RetainForKind(gvk schema.GroupVersionKind, live map[string]bool)
}

// A GarbageCollector garbage collects watches for a single composite resource
// (XR) controller. A watch is eligible for garbage collection when none of the
// XRs the controller owns still needs it - either because no XR composes the
// watched kind (composed resource watches) or because no XR's function pipeline
// requires it (required resource watches). The garbage collector periodically
// lists all the controller's XRs to determine what they still need.
type GarbageCollector struct {
	controllerName string
	gvk            schema.GroupVersionKind
	schema         composite.Schema

	engine   ControllerEngine
	required RequiredResourceProvider

	log logging.Logger
}

// A GarbageCollectorOption configures a GarbageCollector.
type GarbageCollectorOption func(gc *GarbageCollector)

// WithLogger configures how a GarbageCollector should log.
func WithLogger(l logging.Logger) GarbageCollectorOption {
	return func(gc *GarbageCollector) {
		gc.log = l
	}
}

// WithCompositeSchema configures whether to garbage collect a modern or a
// legacy composite resource.
func WithCompositeSchema(s composite.Schema) GarbageCollectorOption {
	return func(gc *GarbageCollector) {
		gc.schema = s
	}
}

// WithRequiredResourceProvider configures the GarbageCollector to also garbage
// collect required resource watches, using the supplied provider to learn which
// required resources the controller's XRs still need.
func WithRequiredResourceProvider(p RequiredResourceProvider) GarbageCollectorOption {
	return func(gc *GarbageCollector) {
		gc.required = p
	}
}

// A nopRequiredResourceProvider reports that no required resources are needed.
// It's the default, so that unless a provider is configured the garbage
// collector only collects composed resource watches.
type nopRequiredResourceProvider struct{}

func (nopRequiredResourceProvider) RequiredGVKs(_ string) []schema.GroupVersionKind            { return nil }
func (nopRequiredResourceProvider) RetainForKind(_ schema.GroupVersionKind, _ map[string]bool) {}

// NewGarbageCollector creates a new watch garbage collector for a controller.
func NewGarbageCollector(name string, of schema.GroupVersionKind, ce ControllerEngine, o ...GarbageCollectorOption) *GarbageCollector {
	gc := &GarbageCollector{
		controllerName: name,
		gvk:            of,
		engine:         ce,
		required:       nopRequiredResourceProvider{},
		log:            logging.NewNopLogger(),
	}
	for _, fn := range o {
		fn(gc)
	}

	return gc
}

// GarbageCollectWatches runs garbage collection at the specified interval,
// until the supplied context is cancelled. It stops any watches for resource
// types that are no longer composed by any composite resource (XR).
func (gc *GarbageCollector) GarbageCollectWatches(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			gc.log.Debug("Stopping composite controller watch garbage collector", "error", ctx.Err())
			return
		case <-t.C:
			if err := gc.GarbageCollectWatchesNow(ctx); err != nil {
				gc.log.Info("Cannot garbage collect composite controller watches", "error", err)
			}
		}
	}
}

// GarbageCollectWatchesNow stops any composed resource watches for kinds no XR
// composes, and any required resource watches for kinds no XR's function
// pipeline requires. It's safe to call from multiple goroutines.
func (gc *GarbageCollector) GarbageCollectWatchesNow(ctx context.Context) error {
	// Get the set of running watches.
	running, err := gc.engine.GetWatches(gc.controllerName)
	if err != nil {
		return errors.Wrap(err, "cannot get running watches")
	}

	// We only garbage collect composed and required resource watches. The other
	// watch types should only be stopped when the XR controller stops.
	collectable := make([]engine.WatchID, 0, len(running))
	for _, wid := range running {
		if wid.Type == engine.WatchTypeComposedResource || wid.Type == engine.WatchTypeRequiredResource {
			collectable = append(collectable, wid)
		}
	}

	// No collectable watches exist. Nothing to do.
	if len(collectable) == 0 {
		return nil
	}

	// Determine which watches look unused, based on a (possibly stale) cached
	// list of XRs.
	used, _, err := gc.usedGVKs(ctx, gc.engine.GetCached())
	if err != nil {
		return err
	}
	stop := unusedWatches(collectable, used)

	// We listed from cache, so the set of 'used' GVKs could be stale. For
	// example a watch for kind: X exists, our stale cache told us an XR was
	// still using kind: X, but really it wasn't. That's okay. We'll stop the
	// watch on the next GC cycle.
	//
	// What we really want to avoid is stopping a watch that we shouldn't.
	// For example a watch for kind: X exists, our stale cache told us no XR
	// was still using kind: X, but really one was. e.g Because it started the
	// watch very recently.
	//
	// So if it looks like there's no watches to stop we return early. If it
	// looks like there _are_ watches to stop, we double check using an
	// uncached client before stopping them.
	if len(stop) == 0 {
		return nil
	}

	// Recompute the used set using an uncached list of XRs.
	used, live, err := gc.usedGVKs(ctx, gc.engine.GetUncached())
	if err != nil {
		return err
	}
	stop = unusedWatches(collectable, used)

	// Forget what we remember about XRs of our kind that no longer exist, so we
	// don't leak memory for deleted XRs. We evict using the authoritative
	// uncached list, so we never forget a live XR's requirements (which would
	// wrongly stop its required resource watches). This means we only evict on
	// cycles where there's at least one watch to stop, but that's fine: a
	// deleted XR's watch is exactly such a watch, so we evict whenever it
	// matters.
	gc.required.RetainForKind(gc.gvk, live)

	// No need to call StopWatches if there's nothing to stop.
	if len(stop) == 0 {
		return nil
	}

	// Stop any watches that are running, but not used.
	//
	// It's possible watches were started or stopped since we called GetWatches.
	// That's fine. Stopping a watch that doesn't exist is a no-op, and if a
	// watch was started that needs garbage collecting we'll get it eventually
	// when GC runs again.
	//
	// It's also possible we'll stop a watch that's actually in use. This'd
	// happen if an XR started composing its type after we listed XRs using
	// the uncached client. We'll recover from this the next time the XR is
	// reconciled, when it'll call StartWatches again. That could take some
	// time though - at worst up to Crossplane's sync interval.
	gc.log.Debug("Garbage collecting watches", "count", len(stop))
	_, err = gc.engine.StopWatches(ctx, gc.controllerName, stop...)

	return errors.Wrap(err, "cannot stop watches for resource types no longer needed by any composite resource")
}

// usedGVKs lists the controller's XRs using the supplied client, and returns the
// kinds of resource they still need, keyed by the type of watch that delivers
// changes to those kinds. A composed resource kind is needed while an XR
// composes it; a required resource kind is needed while an XR's function
// pipeline requires it. It also returns the set of live XR UIDs.
func (gc *GarbageCollector) usedGVKs(ctx context.Context, c client.Reader) (map[engine.WatchType]map[schema.GroupVersionKind]bool, map[string]bool, error) {
	l := &kunstructured.UnstructuredList{}
	l.SetAPIVersion(gc.gvk.GroupVersion().String())
	l.SetKind(gc.gvk.Kind + "List")

	if err := c.List(ctx, l); err != nil {
		return nil, nil, errors.Wrap(err, "cannot list composite resources")
	}

	composed := make(map[schema.GroupVersionKind]bool)
	required := make(map[schema.GroupVersionKind]bool)
	live := make(map[string]bool, len(l.Items))

	for _, u := range l.Items {
		xr := &composite.Unstructured{Unstructured: u, Schema: gc.schema}
		live[string(xr.GetUID())] = true
		for _, ref := range xr.GetResourceReferences() {
			composed[schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)] = true
		}
		for _, gvk := range gc.required.RequiredGVKs(string(xr.GetUID())) {
			required[gvk] = true
		}
	}

	return map[engine.WatchType]map[schema.GroupVersionKind]bool{
		engine.WatchTypeComposedResource: composed,
		engine.WatchTypeRequiredResource: required,
	}, live, nil
}

// unusedWatches returns the watches whose GVK isn't in the used set for their
// watch type.
func unusedWatches(watches []engine.WatchID, used map[engine.WatchType]map[schema.GroupVersionKind]bool) []engine.WatchID {
	stop := make([]engine.WatchID, 0)
	for _, wid := range watches {
		if used[wid.Type][wid.GVK] {
			continue
		}
		stop = append(stop, wid)
	}
	return stop
}
