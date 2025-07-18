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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	opscontroller "github.com/crossplane/crossplane/internal/controller/ops/controller"
	"github.com/crossplane/crossplane/internal/controller/ops/watched"
	"github.com/crossplane/crossplane/internal/engine"
	"github.com/crossplane/crossplane/internal/ops/lifecycle"
)

const (
	timeout   = 2 * time.Minute
	finalizer = "watchoperation.ops.crossplane.io"
)

// Event reasons.
const (
	reasonEstablishWatched event.Reason = "EstablishWatched"
	reasonTerminateWatched event.Reason = "TerminateWatched"
	reasonGarbageCollect   event.Reason = "GarbageCollectOperations"
)

// WatchTypeWatchOperation is the watch type used when a WatchOperation
// controller watches resources specified in the WatchOperation spec.
const WatchTypeWatchOperation engine.WatchType = "WatchOperation"

// A Reconciler reconciles WatchOperations.
type Reconciler struct {
	client client.Client
	log    logging.Logger
	record event.Recorder

	finalizer  resource.Finalizer
	conditions conditions.Manager

	engine  ControllerEngine
	options opscontroller.Options
}

// Reconcile a WatchOperation by starting a controller to watch the specified
// resource type and create Operations when those resources change.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	wo := &v1alpha1.WatchOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, wo); err != nil {
		log.Debug("Cannot get WatchOperation", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get WatchOperation")
	}

	status := r.conditions.For(wo)

	log = log.WithValues(
		"uid", wo.GetUID(),
		"version", wo.GetResourceVersion(),
		"name", wo.GetName(),
		"watched-gvk", schema.FromAPIVersionAndKind(wo.Spec.Watch.APIVersion, wo.Spec.Watch.Kind),
	)

	if meta.WasDeleted(wo) {
		if err := r.engine.Stop(ctx, WatchedControllerName(wo.GetName())); err != nil {
			log.Debug("Cannot stop watched resource controller", "error", err)
			err = errors.Wrap(err, "cannot stop watched resource controller")
			r.record.Event(wo, event.Warning(reasonTerminateWatched, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, wo)
			return reconcile.Result{}, err
		}

		log.Debug("Stopped watched resource controller")

		if err := r.finalizer.RemoveFinalizer(ctx, wo); err != nil {
			log.Debug("Cannot remove watched resource finalizer", "error", err)
			err = errors.Wrap(err, "cannot remove watched resource finalizer")
			r.record.Event(wo, event.Warning(reasonTerminateWatched, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, wo)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: false}, nil
	}

	if err := r.finalizer.AddFinalizer(ctx, wo); err != nil {
		log.Debug("Cannot add watched resource finalizer", "error", err)
		err = errors.Wrap(err, "cannot add watched resource finalizer")
		r.record.Event(wo, event.Warning(reasonEstablishWatched, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, wo)
		return reconcile.Result{}, err
	}

	// Don't reconcile if the WatchOperation is paused.
	if meta.IsPaused(wo) {
		log.Debug("WatchOperation is paused")
		status.MarkConditions(v1alpha1.WatchPaused(), xpv1.ReconcilePaused())
		return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, wo), "cannot update status of WatchOperation")
	}

	// List Operations to update status.
	ol := &v1alpha1.OperationList{}
	if err := r.client.List(ctx, ol, client.MatchingLabels{v1alpha1.LabelWatchOperationName: wo.GetName()}); err != nil {
		log.Debug("Cannot list Operations", "error", err)
		err = errors.Wrap(err, "cannot list Operations")
		r.record.Event(wo, event.Warning(reasonEstablishWatched, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, wo)
		return reconcile.Result{}, err
	}

	// Update LastScheduleTime from the latest Operation creation time.
	if t := lifecycle.LatestCreateTime(ol.Items...); !t.IsZero() {
		wo.Status.LastScheduleTime = &metav1.Time{Time: t}
	}

	// Update LastSuccessfulTime from the latest successful Operation.
	if t := lifecycle.LatestSucceededTransitionTime(lifecycle.WithReason(v1alpha1.ReasonPipelineSuccess, ol.Items...)...); !t.IsZero() {
		wo.Status.LastSuccessfulTime = &metav1.Time{Time: t}
	}

	// Count resources being watched. This is best effort. If we hit an
	// error we just don't update it this time around.
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.FromAPIVersionAndKind(wo.Spec.Watch.APIVersion, wo.Spec.Watch.Kind))
	if err := r.client.List(ctx, ul, client.InNamespace(wo.Spec.Watch.Namespace), client.MatchingLabels(wo.Spec.Watch.MatchLabels)); err == nil {
		wo.Status.WatchingResources = int64(len(ul.Items))
	}

	// Garbage collect Operations older than the history limits.
	for _, op := range lifecycle.MarkGarbage(ptr.Deref(wo.Spec.SuccessfulHistoryLimit, 3), ptr.Deref(wo.Spec.FailedHistoryLimit, 1), ol.Items...) {
		if err := r.client.Delete(ctx, &op); resource.IgnoreNotFound(err) != nil {
			log.Debug("Cannot garbage collect Operation", "error", err, "operation", op.GetName())
			err = errors.Wrapf(err, "cannot garbage collect Operation %q", op.GetName())
			r.record.Event(wo, event.Warning(reasonGarbageCollect, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, wo)
			return reconcile.Result{}, err
		}
	}

	// Update status with running operations.
	running := make([]string, 0)
	for _, op := range ol.Items {
		if op.GetCondition(v1alpha1.TypeSucceeded).Reason == v1alpha1.ReasonPipelineRunning {
			running = append(running, op.GetName())
		}
	}
	wo.Status.RunningOperationRefs = lifecycle.RunningOperationRefs(running)

	// Start the Watched controller.
	wr := watched.NewReconciler(r.engine.GetCached(), wo,
		watched.WithLogger(r.log.WithValues("controller", WatchedControllerName(wo.GetName()))),
		watched.WithRecorder(r.record.WithAnnotations("controller", WatchedControllerName(wo.GetName()))))

	ko := r.options.ForControllerRuntime()
	ko.Reconciler = ratelimiter.NewReconciler(WatchedControllerName(wo.GetName()), errors.WithSilentRequeueOnConflict(wr), r.options.GlobalRateLimiter)

	name := WatchedControllerName(wo.GetName())
	co := []engine.ControllerOption{engine.WithRuntimeOptions(ko)}

	if err := r.engine.Start(name, co...); err != nil {
		log.Debug("Cannot start watched resource controller", "error", err)
		err = errors.Wrap(err, "cannot start watched resource controller")
		r.record.Event(wo, event.Warning(reasonEstablishWatched, err))
		status.MarkConditions(v1alpha1.WatchFailed(err.Error()), xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, wo)
		return reconcile.Result{}, err
	}

	// Start watching the specified kind of resource.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.FromAPIVersionAndKind(wo.Spec.Watch.APIVersion, wo.Spec.Watch.Kind))

	if err := r.engine.StartWatches(ctx, name, engine.WatchFor(u, WatchTypeWatchOperation, NewWatchedResourceHandler(wo))); err != nil {
		log.Debug("Cannot start watched resource controller watches", "error", err)
		err = errors.Wrap(err, "cannot start watched resource controller watches")
		r.record.Event(wo, event.Warning(reasonEstablishWatched, err))
		status.MarkConditions(v1alpha1.WatchFailed(err.Error()), xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, wo)
		return reconcile.Result{}, err
	}

	log.Debug("Started watched resource controller")

	status.MarkConditions(v1alpha1.WatchActive(), xpv1.ReconcileSuccess())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, wo), "cannot update status of WatchOperation")
}

// WatchedControllerName returns the recommended name for controllers that watch
// resources on behalf of a WatchOperation.
func WatchedControllerName(name string) string {
	return "watched/" + name
}
