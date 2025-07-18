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

// Package watchoperation manages the lifecycle of Watched controllers.
package watchoperation

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	opscontroller "github.com/crossplane/crossplane/internal/controller/ops/controller"
	"github.com/crossplane/crossplane/internal/engine"
)

// A ControllerEngine can start and stop Kubernetes controllers on demand.
//
//nolint:interfacebloat // We use this interface to stub the engine for testing, and we need all of its functionality.
type ControllerEngine interface {
	Start(name string, o ...engine.ControllerOption) error
	Stop(ctx context.Context, name string) error
	IsRunning(name string) bool
	GetWatches(name string) ([]engine.WatchID, error)
	StartWatches(ctx context.Context, name string, ws ...engine.Watch) error
	StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	GetCached() client.Client
	GetUncached() client.Client
	GetFieldIndexer() client.FieldIndexer
}

// A NopEngine does nothing.
type NopEngine struct{}

// Start does nothing.
func (e *NopEngine) Start(_ string, _ ...engine.ControllerOption) error {
	return nil
}

// Stop does nothing.
func (e *NopEngine) Stop(_ context.Context, _ string) error { return nil }

// IsRunning always returns true.
func (e *NopEngine) IsRunning(_ string) bool { return true }

// GetWatches does nothing.
func (e *NopEngine) GetWatches(_ string) ([]engine.WatchID, error) { return nil, nil }

// StartWatches does nothing.
func (e *NopEngine) StartWatches(_ context.Context, _ string, _ ...engine.Watch) error { return nil }

// StopWatches does nothing.
func (e *NopEngine) StopWatches(_ context.Context, _ string, _ ...engine.WatchID) (int, error) {
	return 0, nil
}

// GetCached returns a nil client.
func (e *NopEngine) GetCached() client.Client {
	return nil
}

// GetUncached returns a nil client.
func (e *NopEngine) GetUncached() client.Client {
	return nil
}

// GetFieldIndexer returns a nil field indexer.
func (e *NopEngine) GetFieldIndexer() client.FieldIndexer {
	return nil
}

// Setup adds a controller that reconciles WatchOperations by starting a
// controller to watch the specified resource type.
func Setup(mgr ctrl.Manager, o opscontroller.Options) error {
	name := "watchoperation/" + strings.ToLower(v1alpha1.WatchOperationGroupKind)

	r := NewReconciler(mgr.GetClient(),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithControllerEngine(o.ControllerEngine),
		WithOptions(o))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.WatchOperation{}).
		Owns(&v1alpha1.Operation{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithControllerEngine specifies how the Reconciler should manage controllers.
func WithControllerEngine(ce *engine.ControllerEngine) ReconcilerOption {
	return func(r *Reconciler) {
		r.engine = ce
	}
}

// WithOptions specifies how the Reconciler should configure controllers.
func WithOptions(o opscontroller.Options) ReconcilerOption {
	return func(r *Reconciler) {
		r.options = o
	}
}

// NewReconciler returns a Reconciler of WatchOperations.
func NewReconciler(c client.Client, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     c,
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		finalizer:  resource.NewAPIFinalizer(c, finalizer),
		engine:     &NopEngine{},
		options: opscontroller.Options{
			Options: controller.DefaultOptions(),
		},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
