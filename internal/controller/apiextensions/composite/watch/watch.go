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

// Package watch implements a garbage collector for composite resource (XR)
// dependency watches.
package watch

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/dependency"
	"github.com/crossplane/crossplane/v2/internal/engine"
)

// A ControllerEngine can get and stop watches for a controller.
type ControllerEngine interface {
	GetWatches(name string) ([]engine.WatchID, error)
	StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
}

// A GarbageCollector garbage collects dependency watches for a single composite
// resource (XR) controller. A watch is eligible for garbage collection when no
// XR the controller owns depends on its GVK any longer.
type GarbageCollector struct {
	controllerName string

	engine  ControllerEngine
	tracker dependency.Tracker

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

// NewGarbageCollector creates a new dependency watch garbage collector for a
// controller.
func NewGarbageCollector(name string, ce ControllerEngine, t dependency.Tracker, o ...GarbageCollectorOption) *GarbageCollector {
	gc := &GarbageCollector{
		controllerName: name,
		engine:         ce,
		tracker:        t,
		log:            logging.NewNopLogger(),
	}
	for _, fn := range o {
		fn(gc)
	}

	return gc
}

// GarbageCollectWatches runs garbage collection at the specified interval,
// until the supplied context is cancelled. It stops any dependency watches for
// kinds no composite resource (XR) depends on anymore.
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

// GarbageCollectWatchesNow stops any dependency watches for kinds no composite
// resource (XR) depends on anymore. It's safe to call from multiple goroutines.
func (gc *GarbageCollector) GarbageCollectWatchesNow(ctx context.Context) error {
	running, err := gc.engine.GetWatches(gc.controllerName)
	if err != nil {
		return errors.Wrap(err, "cannot get running watches")
	}

	// The kinds at least one XR still depends on.
	depends := make(map[schema.GroupVersionKind]bool)
	for _, gvk := range gc.tracker.GVKs() {
		depends[gvk] = true
	}

	stop := make([]engine.WatchID, 0)
	for _, wid := range running {
		// Only stop dependency watches. The other watch types (the XR itself,
		// its composition revision) are stopped when the controller stops.
		if wid.Type != engine.WatchTypeDependency {
			continue
		}
		if depends[wid.GVK] {
			continue
		}

		stop = append(stop, wid)
	}

	if len(stop) == 0 {
		return nil
	}

	// It's possible an XR came to depend on one of these kinds since we read the
	// tracker - it would have started the watch again as it reconciled. We might
	// stop it here regardless. That's fine: the XR will restart the watch next
	// time it reconciles, at worst within Crossplane's sync interval.
	gc.log.Debug("Garbage collecting watches", "count", len(stop))
	_, err = gc.engine.StopWatches(ctx, gc.controllerName, stop...)

	return errors.Wrap(err, "cannot stop watches for kinds no composite resource depends on")
}
