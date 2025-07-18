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

// Package watched implements a controller for resources watched by WatchOperations.
package watched

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// NewReconciler returns a Reconciler that watches resources on behalf of
// a WatchOperation.
func NewReconciler(c client.Client, wo *v1alpha1.WatchOperation, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:      c,
		watchOpName: wo.GetName(),
		watchedGVK:  schema.FromAPIVersionAndKind(wo.Spec.Watch.APIVersion, wo.Spec.Watch.Kind),
		log:         logging.NewNopLogger(),
		record:      event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}
