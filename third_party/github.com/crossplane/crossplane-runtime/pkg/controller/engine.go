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

// Package controller provides utilties for working with controllers.
package controller

import (
	"context"
	"sync"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings
const (
	errCreateCache      = "cannot create new cache"
	errCreateController = "cannot create new controller"
	errCrashCache       = "cache error"
	errCrashController  = "controller error"
	errWatch            = "cannot setup watch"
)

// A NewCacheFn creates a new controller-runtime cache.
type NewCacheFn func(cfg *rest.Config, o cache.Options) (cache.Cache, error)

// A NewControllerFn creates a new controller-runtime controller.
type NewControllerFn func(name string, m manager.Manager, o controller.Options) (controller.Controller, error)

// The default new cache and new controller functions.
var (
	DefaultNewCacheFn      NewCacheFn      = cache.New
	DefaultNewControllerFn NewControllerFn = controller.NewUnmanaged
)

// An Engine manages the lifecycles of controller-runtime controllers (and their
// caches). The lifecycles of the controllers are not coupled to lifecycle of
// the engine, nor to the lifecycle of the controller manager it uses.
type Engine struct {
	mgr manager.Manager

	started map[string]context.CancelFunc
	errors  map[string]error
	mx      sync.RWMutex

	newCache NewCacheFn
	newCtrl  NewControllerFn
}

// An EngineOption configures an Engine.
type EngineOption func(*Engine)

// WithNewCacheFn may be used to configure a different cache implementation.
// DefaultNewCacheFn is used by default.
func WithNewCacheFn(fn NewCacheFn) EngineOption {
	return func(e *Engine) {
		e.newCache = fn
	}
}

// WithNewControllerFn may be used to configure a different controller
// implementation. DefaultNewControllerFn is used by default.
func WithNewControllerFn(fn NewControllerFn) EngineOption {
	return func(e *Engine) {
		e.newCtrl = fn
	}
}

// NewEngine produces a new Engine.
func NewEngine(mgr manager.Manager, o ...EngineOption) *Engine {
	e := &Engine{
		mgr: mgr,

		started: make(map[string]context.CancelFunc),
		errors:  make(map[string]error),

		newCache: DefaultNewCacheFn,
		newCtrl:  DefaultNewControllerFn,
	}

	for _, eo := range o {
		eo(e)
	}

	return e
}

// IsRunning indicates whether the named controller is running - i.e. whether it
// has been started and does not appear to have crashed.
func (e *Engine) IsRunning(name string) bool {
	e.mx.RLock()
	defer e.mx.RUnlock()

	_, running := e.started[name]
	return running
}

// Err returns any error encountered by the named controller. The returned error
// is always nil if the named controller is running.
func (e *Engine) Err(name string) error {
	e.mx.RLock()
	defer e.mx.RUnlock()

	return e.errors[name]
}

// Stop the named controller.
func (e *Engine) Stop(name string) {
	e.done(name, nil)
}

func (e *Engine) done(name string, err error) {
	e.mx.Lock()
	defer e.mx.Unlock()

	stop, ok := e.started[name]
	if ok {
		stop()
		delete(e.started, name)
	}

	// Don't overwrite the first error if done is called multiple times.
	if e.errors[name] != nil {
		return
	}
	e.errors[name] = err
}

// Watch an object.
type Watch struct {
	kind       client.Object
	handler    handler.EventHandler
	predicates []predicate.Predicate
}

// For returns a Watch for the supplied kind of object. Events will be handled
// by the supplied EventHandler, and may be filtered by the supplied predicates.
func For(kind client.Object, h handler.EventHandler, p ...predicate.Predicate) Watch {
	return Watch{kind: kind, handler: h, predicates: p}
}

// Start the named controller. Each controller is started with its own cache
// whose lifecycle is coupled to the controller. The controller is started with
// the supplied options, and configured with the supplied watches. Start does
// not block.
func (e *Engine) Start(name string, o controller.Options, w ...Watch) error {
	if e.IsRunning(name) {
		return nil
	}

	ctx, stop := context.WithCancel(context.Background())
	e.mx.Lock()
	e.started[name] = stop
	e.errors[name] = nil
	e.mx.Unlock()

	// Each controller gets its own cache because there's currently no way to
	// stop an informer. In practice a controller-runtime cache is a map of
	// kinds to informers. If we delete the CRD for a kind we need to stop the
	// relevant informer, or it will spew errors about the kind not existing. We
	// work around this by stopping the entire cache.
	ca, err := e.newCache(e.mgr.GetConfig(), cache.Options{Scheme: e.mgr.GetScheme(), Mapper: e.mgr.GetRESTMapper()})
	if err != nil {
		return errors.Wrap(err, errCreateCache)
	}

	ctrl, err := e.newCtrl(name, e.mgr, o)
	if err != nil {
		return errors.Wrap(err, errCreateController)
	}

	for _, wt := range w {
		if err := ctrl.Watch(source.Kind(ca, wt.kind), wt.handler, wt.predicates...); err != nil {
			return errors.Wrap(err, errWatch)
		}
	}

	go func() {
		<-e.mgr.Elected()
		e.done(name, errors.Wrap(ca.Start(ctx), errCrashCache))
	}()
	go func() {
		<-e.mgr.Elected()
		e.done(name, errors.Wrap(ctrl.Start(ctx), errCrashController))
	}()

	return nil
}
