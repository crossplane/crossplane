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

// Package xreconcile contains a reconciler for unstructured Kubernetes objects.
package xreconcile

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/internal/xresource/unstructured/xreconcile/xlogging"
)

const (
	reconcileTimeout = 1 * time.Minute

	// ErrClientGet is the error message for client.Get() failures.
	ErrClientGet = "cannot get %s"
	// ErrClientStatusUpdate is the error message for client.Status().Update() failures.
	ErrClientStatusUpdate = "cannot update %s status"
)

// A ReconcilerOption configures a Reconciler.
type ReconcilerOption[object client.Object] func(*objectReconcilerAdapter[object])

// WithReconcileTimeout specifies the timeout for each reconciliation loop.
func WithReconcileTimeout[object client.Object](timeout time.Duration) ReconcilerOption[object] {
	return func(r *objectReconcilerAdapter[object]) {
		r.timeout = timeout
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger[object client.Object](l logging.Logger) ReconcilerOption[object] {
	return func(r *objectReconcilerAdapter[object]) {
		r.log = l
	}
}

// AsUnstructuredReconciler returns a reconcile.Reconciler implementation that wraps unstructured reconcilers that would
// like to adopt the reconcile.AsReconciler pattern but for reconcilers that do not conform to a concreate type.
func AsUnstructuredReconciler[object client.Object](client client.Client, rec reconcile.ObjectReconciler[object], constructorFn func() object, o ...ReconcilerOption[object]) reconcile.Reconciler {
	r := &objectReconcilerAdapter[object]{
		objReconciler: rec,
		client:        client,
		constructorFn: constructorFn,
		timeout:       reconcileTimeout,
		log:           logging.NewNopLogger(),
	}

	for _, ro := range o {
		ro(r)
	}

	return r
}

type objectReconcilerAdapter[object client.Object] struct {
	objReconciler     reconcile.ObjectReconciler[object]
	client            client.Client
	constructorFn     func() object
	timeout           time.Duration
	log               logging.Logger
	skipStatusUpdates bool
}

// Reconcile implements Reconciler.
func (r *objectReconcilerAdapter[object]) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	// Pre-amble

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	o := r.constructorFn()
	if err := r.client.Get(ctx, req.NamespacedName, o); err != nil {
		msg := fmt.Sprintf(ErrClientGet, o.GetObjectKind().GroupVersionKind().Kind)
		log.Debug(msg, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), msg)
	}
	original := o.DeepCopyObject()

	log = log.WithValues(
		"uid", o.GetUID(),
		"version", o.GetResourceVersion(),
	)
	if extName := meta.GetExternalName(o); extName != "" {
		log = log.WithValues("external-name", extName)
	}
	ctx = xlogging.WithLogger(ctx, log)

	// Do Reconcile

	result, resultErr := r.objReconciler.Reconcile(ctx, o)

	// Post-ample

	// Synchronize the status.
	switch {
	case r.skipStatusUpdates:
		// This reconciler implementation is configured to skip resource updates.
	case false: // TODO: remove this inplace of case equality...
		_ = original // TODO: implement some way to compare unstructured status.
		// case equality.Semantic.DeepEqual(original.Status, o.Status):
		// If we didn't change anything then don't call updateStatus.
	default:
		if err := errors.Wrap(r.client.Status().Update(ctx, o),
			fmt.Sprintf(ErrClientStatusUpdate, o.GetObjectKind().GroupVersionKind().Kind)); err != nil {
			// Join both the reconciler error and the status update error.
			resultErr = errors.Join(err, resultErr)
		}
	}

	return result, resultErr
}
