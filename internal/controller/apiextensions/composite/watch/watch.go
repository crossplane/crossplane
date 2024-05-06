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

// Package watch implements a garbage collector for composed resource watches.
package watch

import (
	"context"
	"time"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	"github.com/crossplane/crossplane/internal/engine"
)

// A ControllerEngine can get and stop watches for a controller.
type ControllerEngine interface {
	GetWatches(name string) ([]engine.WatchID, error)
	StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	GetClient() client.Client
}

// A GarbageCollector garbage collects watches for a single composite resource
// (XR) controller. A watch is eligible for garbage collection when none of the
// XRs the controller owns resource references its GVK. The garbage collector
// periodically lists all of the controller's XRs to determine what GVKs they
// still reference.
type GarbageCollector struct {
	controllerName string
	xrGVK          schema.GroupVersionKind

	engine ControllerEngine

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

// NewGarbageCollector creates a new watch garbage collector for a controller.
func NewGarbageCollector(name string, of resource.CompositeKind, ce ControllerEngine, o ...GarbageCollectorOption) *GarbageCollector {
	gc := &GarbageCollector{
		controllerName: name,
		xrGVK:          schema.GroupVersionKind(of),
		engine:         ce,
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

// GarbageCollectWatchesNow stops any watches for resource types that are no
// longer composed by any composite resource (XR). It's safe to call from
// multiple goroutines.
func (gc *GarbageCollector) GarbageCollectWatchesNow(ctx context.Context) error {
	// List all XRs of the type we're interested in.
	l := &kunstructured.UnstructuredList{}
	l.SetAPIVersion(gc.xrGVK.GroupVersion().String())
	l.SetKind(gc.xrGVK.Kind + "List")
	if err := gc.engine.GetClient().List(ctx, l); err != nil {
		return errors.Wrap(err, "cannot list composite resources")
	}

	// Build the set of GVKs they still reference.
	used := make(map[engine.WatchID]bool)
	for _, u := range l.Items {
		xr := &composite.Unstructured{Unstructured: u}
		for _, ref := range xr.GetResourceReferences() {
			used[engine.WatchID{Type: engine.WatchTypeComposedResource, GVK: schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)}] = true
		}
	}

	// Get the set of running watches.
	running, err := gc.engine.GetWatches(gc.controllerName)
	if err != nil {
		return errors.Wrap(err, "cannot get running watches")
	}

	stop := make([]engine.WatchID, 0)
	for _, wid := range running {
		if !used[wid] {
			stop = append(stop, wid)
		}
	}

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
	// It's also possible we'll stop a watch that's actually in use, if an XR
	// started composing its GVK between when we built the map of used watches
	// and here where we call StopWatches. It'll restart next time the XR
	// controller calls StartWatches.
	_, err = gc.engine.StopWatches(ctx, gc.controllerName, stop...)
	return errors.Wrap(err, "cannot stop watches for composed resource types that are no longer referenced by any composite resource")
}
