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

package engine

import (
	"context"

	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

var _ source.Source = &StoppableSource{}

// NewStoppableSource returns a new watch source that can be stopped.
func NewStoppableSource(infs cache.Informers, t client.Object, h handler.EventHandler, ps ...predicate.Predicate) *StoppableSource {
	return &StoppableSource{infs: infs, Type: t, handler: h, predicates: ps}
}

// A StoppableSource is a controller-runtime watch source that can be stopped.
type StoppableSource struct {
	infs cache.Informers

	Type       client.Object
	handler    handler.EventHandler
	predicates []predicate.Predicate

	reg kcache.ResourceEventHandlerRegistration
}

// Start is internal and should be called only by the Controller to register
// an EventHandler with the Informer to enqueue reconcile.Requests.
func (s *StoppableSource) Start(ctx context.Context, q workqueue.RateLimitingInterface) error {
	i, err := s.infs.GetInformer(ctx, s.Type, cache.BlockUntilSynced(true))
	if err != nil {
		return errors.Wrapf(err, "cannot get informer for %T", s.Type)
	}

	reg, err := i.AddEventHandler(NewEventHandler(ctx, q, s.handler, s.predicates...).HandlerFuncs())
	if err != nil {
		return errors.Wrapf(err, "cannot add event handler")
	}
	s.reg = reg

	return nil
}

// Stop removes the EventHandler from the source's Informer. The Informer will
// stop sending events to the source.
func (s *StoppableSource) Stop(ctx context.Context) error {
	if s.reg == nil {
		return nil
	}

	i, err := s.infs.GetInformer(ctx, s.Type)
	if err != nil {
		return errors.Wrapf(err, "cannot get informer for %T", s.Type)
	}

	if err := i.RemoveEventHandler(s.reg); err != nil {
		return errors.Wrap(err, "cannot remove event handler")
	}

	s.reg = nil
	return nil
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(ctx context.Context, q workqueue.RateLimitingInterface, h handler.EventHandler, ps ...predicate.Predicate) *EventHandler {
	return &EventHandler{
		ctx:        ctx,
		handler:    h,
		queue:      q,
		predicates: ps,
	}
}

// An EventHandler converts a controller-runtime handler and predicates into a
// client-go ResourceEventHandler. It's a stripped down version of
// controller-runtime's internal implementation.
// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.18.2/pkg/internal/source/event_handler.go#L35
type EventHandler struct {
	ctx context.Context //nolint:containedctx // Kept for compatibility with controller-runtime.

	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

// HandlerFuncs converts EventHandler to a ResourceEventHandlerFuncs.
func (e *EventHandler) HandlerFuncs() kcache.ResourceEventHandlerFuncs {
	return kcache.ResourceEventHandlerFuncs{
		AddFunc:    e.OnAdd,
		UpdateFunc: e.OnUpdate,
		DeleteFunc: e.OnDelete,
	}
}

// OnAdd creates CreateEvent and calls Create on EventHandler.
func (e *EventHandler) OnAdd(obj interface{}) {
	o, ok := obj.(client.Object)
	if !ok {
		return
	}

	c := event.CreateEvent{Object: o}
	for _, p := range e.predicates {
		if !p.Create(c) {
			return
		}
	}

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	e.handler.Create(ctx, c, e.queue)
}

// OnUpdate creates UpdateEvent and calls Update on EventHandler.
func (e *EventHandler) OnUpdate(oldObj, newObj interface{}) {
	o, ok := oldObj.(client.Object)
	if !ok {
		return
	}

	n, ok := newObj.(client.Object)
	if !ok {
		return
	}

	u := event.UpdateEvent{ObjectOld: o, ObjectNew: n}

	for _, p := range e.predicates {
		if !p.Update(u) {
			return
		}
	}

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	e.handler.Update(ctx, u, e.queue)
}

// OnDelete creates DeleteEvent and calls Delete on EventHandler.
func (e *EventHandler) OnDelete(obj interface{}) {
	var d event.DeleteEvent

	switch o := obj.(type) {
	case client.Object:
		d = event.DeleteEvent{Object: o}

	// Deal with tombstone events by pulling the object out. Tombstone events
	// wrap the object in a DeleteFinalStateUnknown struct, so the object needs
	// to be pulled out.
	case kcache.DeletedFinalStateUnknown:
		wrapped, ok := o.Obj.(client.Object)
		if !ok {
			return
		}
		d = event.DeleteEvent{DeleteStateUnknown: true, Object: wrapped}

	default:
		return
	}

	for _, p := range e.predicates {
		if !p.Delete(d) {
			return
		}
	}

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	e.handler.Delete(ctx, d, e.queue)
}
