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

// Package engine manages the lifecycle of a set of controllers.
package engine

import (
	"context"
	"sync"
	"time"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kcache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// A ControllerEngine manages a set of controllers that can be dynamically
// started and stopped. It also manages a dynamic set of watches per controller,
// and the informers that back them.
type ControllerEngine struct {
	// The manager of this engine's controllers. Controllers managed by the
	// engine use the engine's client and cache, not the manager's.
	mgr manager.Manager

	// The engine must have exclusive use of these informers. All controllers
	// managed by the engine should use these informers.
	infs TrackingInformers

	// The client used by the engine's controllers. The client must be backed by
	// the above TrackingInformers.
	client client.Client

	log logging.Logger

	// Protects everything below.
	mx sync.RWMutex

	// Running controllers, by name.
	controllers map[string]*controller
}

// TrackingInformers is a set of Informers. It tracks which are active.
type TrackingInformers interface {
	cache.Informers
	ActiveInformers() []schema.GroupVersionKind
}

// New creates a new controller engine.
func New(mgr manager.Manager, infs TrackingInformers, c client.Client, o ...ControllerEngineOption) *ControllerEngine {
	e := &ControllerEngine{
		mgr:         mgr,
		infs:        infs,
		client:      c,
		log:         logging.NewNopLogger(),
		controllers: make(map[string]*controller),
	}

	for _, fn := range o {
		fn(e)
	}

	return e
}

// An ControllerEngineOption configures a controller engine.
type ControllerEngineOption func(*ControllerEngine)

// WithLogger configures an Engine to use a logger.
func WithLogger(l logging.Logger) ControllerEngineOption {
	return func(e *ControllerEngine) {
		e.log = l
	}
}

type controller struct {
	// The running controller.
	ctrl kcontroller.Controller

	// Called to stop the controller.
	cancel context.CancelFunc

	// Protects the below map.
	mx sync.RWMutex

	// The controller's sources, by watched GVK.
	sources map[WatchID]*StoppableSource
}

// A WatchGarbageCollector periodically garbage collects watches.
type WatchGarbageCollector interface {
	GarbageCollectWatches(ctx context.Context, interval time.Duration)
}

// A NewControllerFn can start a new controller-runtime controller.
type NewControllerFn func(name string, mgr manager.Manager, options kcontroller.Options) (kcontroller.Controller, error)

// ControllerOptions configure a controller.
type ControllerOptions struct {
	runtime kcontroller.Options
	nc      NewControllerFn
	gc      WatchGarbageCollector
}

// A ControllerOption configures a controller.
type ControllerOption func(o *ControllerOptions)

// WithRuntimeOptions configures the underlying controller-runtime controller.
func WithRuntimeOptions(ko kcontroller.Options) ControllerOption {
	return func(o *ControllerOptions) {
		o.runtime = ko
	}
}

// WithWatchGarbageCollector specifies an optional garbage collector this
// controller should use to remove unused watches.
func WithWatchGarbageCollector(gc WatchGarbageCollector) ControllerOption {
	return func(o *ControllerOptions) {
		o.gc = gc
	}
}

// WithNewControllerFn configures how the engine starts a new controller-runtime
// controller.
func WithNewControllerFn(fn NewControllerFn) ControllerOption {
	return func(o *ControllerOptions) {
		o.nc = fn
	}
}

// GetClient gets a client backed by the controller engine's cache.
func (e *ControllerEngine) GetClient() client.Client {
	return e.client
}

// GetFieldIndexer returns a FieldIndexer that can be used to add indexes to the
// controller engine's cache.
func (e *ControllerEngine) GetFieldIndexer() client.FieldIndexer {
	return e.infs
}

// Start a new controller.
func (e *ControllerEngine) Start(name string, o ...ControllerOption) error {
	e.mx.Lock()
	defer e.mx.Unlock()

	// Start is a no-op if the controller is already running.
	if _, running := e.controllers[name]; running {
		return nil
	}

	co := &ControllerOptions{nc: kcontroller.NewUnmanaged}
	for _, fn := range o {
		fn(co)
	}

	c, err := co.nc(name, e.mgr, co.runtime)
	if err != nil {
		return errors.Wrap(err, "cannot create new controller")
	}

	// The caller will usually be a reconcile method. We want the controller
	// to keep running when the reconcile ends, so we create a new context
	// instead of taking one as an argument.
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Don't start the controller until the manager is elected.
		<-e.mgr.Elected()

		e.log.Debug("Starting new controller", "controller", name)

		// Run the controller until its context is cancelled.
		if err := c.Start(ctx); err != nil {
			e.log.Info("Controller stopped with an error", "name", name, "error", err)

			// Make a best effort attempt to cleanup the controller so that
			// IsRunning will return false.
			_ = e.Stop(ctx, name)
			return
		}

		e.log.Debug("Stopped controller", "controller", name)
	}()

	if co.gc != nil {
		go func() {
			// Don't start the garbage collector until the manager is elected.
			<-e.mgr.Elected()

			e.log.Debug("Starting watch garbage collector for controller", "controller", name)

			// Run the collector every minute until its context is cancelled.
			co.gc.GarbageCollectWatches(ctx, 1*time.Minute)

			e.log.Debug("Stopped watch garbage collector for controller", "controller", name)
		}()
	}

	r := &controller{
		ctrl:    c,
		cancel:  cancel,
		sources: make(map[WatchID]*StoppableSource),
	}

	e.controllers[name] = r

	return nil
}

// Stop a controller.
func (e *ControllerEngine) Stop(ctx context.Context, name string) error {
	e.mx.Lock()
	defer e.mx.Unlock()

	c, running := e.controllers[name]

	// Stop is a no-op if the controller isn't running.
	if !running {
		return nil
	}

	c.mx.Lock()
	defer c.mx.Unlock()

	// Stop the controller's watches.
	for wid, w := range c.sources {
		if err := w.Stop(ctx); err != nil {
			c.mx.Unlock()
			return errors.Wrapf(err, "cannot stop %q watch for %q", wid.Type, wid.GVK)
		}
		delete(c.sources, wid)
		e.log.Debug("Stopped watching GVK", "controller", name, "watch-type", wid.Type, "watched-gvk", wid.GVK)
	}

	// Stop and delete the controller.
	c.cancel()
	delete(e.controllers, name)

	e.log.Debug("Stopped controller", "controller", name)
	return nil
}

// IsRunning returns true if the named controller is running.
func (e *ControllerEngine) IsRunning(name string) bool {
	e.mx.RLock()
	defer e.mx.RUnlock()
	_, running := e.controllers[name]
	return running
}

// A WatchType uniquely identifies a "type" of watch - i.e. a handler and a set
// of predicates. The controller engine uniquely identifies a Watch by its
// (kind, watch type) tuple. The engine will only start one watch of each (kind,
// watch type) tuple. To watch the same kind of resource multiple times, use
// different watch types.
type WatchType string

// Common watch types.
const (
	WatchTypeClaim               WatchType = "Claim"
	WatchTypeCompositeResource   WatchType = "CompositeResource"
	WatchTypeComposedResource    WatchType = "ComposedResource"
	WatchTypeCompositionRevision WatchType = "CompositionRevision"
)

// Watch an object.
type Watch struct {
	wt         WatchType
	kind       client.Object
	handler    handler.EventHandler
	predicates []predicate.Predicate
}

// A WatchID uniquely identifies a watch.
type WatchID struct {
	Type WatchType
	GVK  schema.GroupVersionKind
}

// WatchFor returns a Watch for the supplied kind of object. Events will be
// handled by the supplied EventHandler, and may be filtered by the supplied
// predicates.
func WatchFor(kind client.Object, wt WatchType, h handler.EventHandler, p ...predicate.Predicate) Watch {
	return Watch{kind: kind, wt: wt, handler: h, predicates: p}
}

// StartWatches instructs the named controller to start the supplied watches.
// The controller will only start a watch if it's not already watching the type
// of object specified by the supplied Watch. StartWatches blocks other
// operations on the same controller if and when it starts a watch.
func (e *ControllerEngine) StartWatches(name string, ws ...Watch) error {
	e.mx.RLock()
	c, running := e.controllers[name]
	e.mx.RUnlock()

	if !running {
		return errors.Errorf("controller %q is not running", name)
	}

	// Make sure we can get GVKs for all the watches before we take locks.
	gvks := make([]schema.GroupVersionKind, len(ws))
	for i := range ws {
		gvk, err := apiutil.GVKForObject(ws[i].kind, e.mgr.GetScheme())
		if err != nil {
			return errors.Wrapf(err, "cannot determine group, version, and kind for %T", ws[i].kind)
		}
		gvks[i] = gvk
	}

	// It's possible that we didn't explicitly stop a watch, but its backing
	// informer was removed. This implicitly stops the watch by deleting its
	// backing listener. If a watch exists but doesn't have an active informer,
	// we want to restart the watch (and, implicitly, the informer).
	//
	// There's a potential race here. Another Goroutine could remove an informer
	// between where we build the map and where we read it to check whether an
	// informer is active. We wouldn't start a watch when we should. If the
	// controller calls StartWatches repeatedly (e.g. an XR controller) this
	// will eventually self-correct.
	a := e.infs.ActiveInformers()
	activeInformer := make(map[schema.GroupVersionKind]bool, len(a))
	for _, gvk := range a {
		activeInformer[gvk] = true
	}

	// Some controllers will call StartWatches on every reconcile. Most calls
	// won't actually need to start a new watch. For example an XR controller
	// would only need to start a new watch if an XR composed a new kind of
	// resource that no other XR it controls already composes. So, we try to
	// avoid taking a write lock and blocking all reconciles unless we need to.
	c.mx.RLock()
	start := false
	for i, w := range ws {
		wid := WatchID{Type: w.wt, GVK: gvks[i]}
		// We've already created this watch and the informer backing it is still
		// running. We don't need to create a new watch.
		if _, watchExists := c.sources[wid]; watchExists && activeInformer[wid.GVK] {
			e.log.Debug("Watch exists for GVK, not starting a new one", "controller", name, "watch-type", wid.Type, "watched-gvk", wid.GVK)
			continue
		}
		// There's at least one watch to start.
		start = true
		break
	}
	c.mx.RUnlock()

	// Nothing to start.
	if !start {
		return nil
	}

	// We have at least one watch to start - take the write lock. It's possible
	// another Goroutine updated this controller's watches since we released the
	// read lock, so we compute everything again.
	c.mx.Lock()
	defer c.mx.Unlock()

	// Start new sources.
	for i, w := range ws {
		wid := WatchID{Type: w.wt, GVK: gvks[i]}

		// We've already created this watch and the informer backing it is still
		// running. We don't need to create a new watch. We don't debug log this
		// one - we'll have logged it above unless the watch was added between
		// releasing the read lock and taking the write lock.
		if _, watchExists := c.sources[wid]; watchExists && activeInformer[wid.GVK] {
			continue
		}

		// The controller's Watch method just calls the StoppableSource's Start
		// method, passing in its private work queue as an argument. This will
		// start an informer for the watched kind if there isn't one running
		// already.
		//
		// The watch will stop sending events when either the source is stopped,
		// or its backing informer is stopped. The controller's work queue will
		// stop processing events when the controller is stopped.
		src := NewStoppableSource(e.infs, w.kind, w.handler, w.predicates...)
		if err := c.ctrl.Watch(src); err != nil {
			return errors.Wrapf(err, "cannot start %q watch for %q", wid.Type, wid.GVK)
		}

		// Record that we're now running this source.
		c.sources[wid] = src

		e.log.Debug("Started watching GVK", "controller", name, "watch-type", wid.Type, "watched-gvk", wid.GVK)
	}

	return nil
}

// GetWatches returns the active watches for the supplied controller.
func (e *ControllerEngine) GetWatches(name string) ([]WatchID, error) {
	e.mx.RLock()
	c, running := e.controllers[name]
	e.mx.RUnlock()

	if !running {
		return nil, errors.Errorf("controller %q is not running", name)
	}

	c.mx.RLock()
	defer c.mx.RUnlock()

	out := make([]WatchID, 0, len(c.sources))
	for wid := range c.sources {
		out = append(out, wid)
	}
	return out, nil
}

// StopWatches stops the supplied watches. StopWatches blocks other operations
// on the same controller if and when it stops a watch. It returns the number of
// watches that it successfully stopped.
func (e *ControllerEngine) StopWatches(ctx context.Context, name string, ws ...WatchID) (int, error) {
	e.mx.RLock()
	c, running := e.controllers[name]
	e.mx.RUnlock()

	if !running {
		return 0, errors.Errorf("controller %q is not running", name)
	}

	// Don't take the write lock if we want to keep all watches.
	c.mx.RLock()
	stop := false
	for _, wid := range ws {
		if _, watchExists := c.sources[wid]; watchExists {
			stop = true
			break
		}
	}
	c.mx.RUnlock()

	if !stop {
		return 0, nil
	}

	// We have at least one watch to stop - take the write lock. It's possible
	// another Goroutine updated this controller's watches since we released the
	// read lock, so we compute everything again.
	c.mx.Lock()
	defer c.mx.Unlock()

	stopped := 0
	for _, wid := range ws {
		w, watchExists := c.sources[wid]
		if !watchExists {
			continue
		}
		if err := w.Stop(ctx); err != nil {
			return stopped, errors.Wrapf(err, "cannot stop %q watch for %q", wid.Type, wid.GVK)
		}
		delete(c.sources, wid)
		e.log.Debug("Stopped watching GVK", "controller", name, "watch-type", wid.Type, "watched-gvk", wid.GVK)
		stopped++
	}

	return stopped, nil
}

// GarbageCollectCustomResourceInformers garbage collects informers for custom
// resources (e.g. Crossplane XRs, claims and composed resources) when the CRD
// that defines them is deleted. The garbage collector runs until the supplied
// context is cancelled.
func (e *ControllerEngine) GarbageCollectCustomResourceInformers(ctx context.Context) error {
	i, err := e.infs.GetInformer(ctx, &extv1.CustomResourceDefinition{})
	if err != nil {
		return errors.Wrap(err, "cannot get informer for CustomResourceDefinitions")
	}

	h, err := i.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			o := obj
			if fsu, ok := obj.(kcache.DeletedFinalStateUnknown); ok {
				o = fsu.Obj
			}
			crd, ok := o.(*extv1.CustomResourceDefinition)
			if !ok {
				// This should never happen.
				return
			}

			for _, v := range crd.Spec.Versions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Kind:    crd.Spec.Names.Kind,
					Version: v.Name,
				}

				u := &unstructured.Unstructured{}
				u.SetGroupVersionKind(gvk)

				if err := e.infs.RemoveInformer(ctx, u); err != nil {
					e.log.Info("Cannot remove informer for type defined by deleted CustomResourceDefinition", "crd", crd.GetName(), "gvk", gvk)
					continue
				}

				e.log.Debug("Removed informer for type defined by deleted CustomResourceDefinition", "crd", crd.GetName(), "gvk", gvk)
			}
		},
	})
	if err != nil {
		return errors.Wrap(err, "cannot add garbage collector event handler to CustomResourceDefinition informer")
	}

	go func() {
		<-ctx.Done()
		if err := i.RemoveEventHandler(h); err != nil {
			e.log.Info("Cannot remove garbage collector event handler from CustomResourceDefinition informer")
		}
	}()

	return nil
}
