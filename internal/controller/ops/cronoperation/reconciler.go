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

package cronoperation

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
	"github.com/crossplane/crossplane/internal/ops/lifecycle"
)

// Event reasons.
const (
	reasonInvalidSchedule = "InvalidSchedule"
	reasonListOperations  = "ListOperations"
	reasonDeleteOperation = "DeleteOperation"
	reasonCreateOperation = "CreateOperation"
)

// A Scheduler determines when the next Operation should run.
type Scheduler interface {
	Next(schedule string, last time.Time) (time.Time, error)
}

// A SchedulerFn is a function that satisfies Scheduler.
type SchedulerFn func(schedule string, last time.Time) (time.Time, error)

// Next returns the next time an Operation should run.
func (fn SchedulerFn) Next(schedule string, last time.Time) (time.Time, error) {
	return fn(schedule, last)
}

// A Reconciler reconciles CronOperations.
type Reconciler struct {
	client     client.Client
	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
	schedule   Scheduler
}

// Reconcile a CronOperation.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)

	co := &v1alpha1.CronOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, co); err != nil {
		log.Debug("cannot get CronOperation", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get CronOperation")
	}

	status := r.conditions.For(co)

	log = log.WithValues(
		"uid", co.GetUID(),
		"version", co.GetResourceVersion(),
		"name", co.GetName(),
	)

	// Don't reconcile if the CronOperation is being deleted.
	if meta.WasDeleted(co) {
		return reconcile.Result{Requeue: false}, nil
	}

	// Don't reconcile if the CronOperation is suspended.
	if ptr.Deref(co.Spec.Suspend, false) {
		log.Debug("CronOperation is suspended")
		return reconcile.Result{Requeue: false}, nil
	}

	ol := &v1alpha1.OperationList{}
	if err := r.client.List(ctx, ol, client.MatchingLabels{v1alpha1.LabelCronOperationName: co.GetName()}); err != nil {
		log.Debug("Cannot list Operations", "error", err)
		err = errors.Wrap(err, "cannot list Operations")
		r.record.Event(co, event.Warning(reasonListOperations, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, co)
		return reconcile.Result{}, err
	}

	// Derive our last scheduled time from the last time we created an
	// Operation.
	if t := lifecycle.LatestCreateTime(ol.Items...); !t.IsZero() {
		co.Status.LastScheduleTime = &metav1.Time{Time: t}
	}

	// If we recorded a last schedule time use it. If not, use the
	// CronOperation's creation timestamp.
	last := ptr.Deref(co.Status.LastScheduleTime, co.GetCreationTimestamp()).Time

	// Record the last time an Operation succeeded, if any.
	if t := lifecycle.LatestSucceededTransitionTime(lifecycle.WithReason(v1alpha1.ReasonPipelineSuccess, ol.Items...)...); !t.IsZero() {
		co.Status.LastSuccessfulTime = &metav1.Time{Time: t}
	}

	// Record all running Operations.
	running := make(map[string]bool)
	for _, op := range lifecycle.WithReason(v1alpha1.ReasonPipelineRunning, ol.Items...) {
		running[op.GetName()] = true
	}
	co.Status.RunningOperationRefs = lifecycle.RunningOperationRefs(slices.Sorted(maps.Keys(running)))

	// Garbage collect Operations older than the history limits.
	for _, op := range lifecycle.MarkGarbage(co.Spec.SuccessfulHistoryLimit, co.Spec.FailedHistoryLimit, ol.Items...) {
		if err := r.client.Delete(ctx, &op); resource.IgnoreNotFound(err) != nil {
			log.Debug("Cannot garbage collect Operation", "error", err, "operation", op.GetName())
			err = errors.Wrapf(err, "cannot garbage collect Operation %q", op.GetName())
			r.record.Event(co, event.Warning(reasonDeleteOperation, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, co)
			return reconcile.Result{}, err
		}
	}

	next, err := r.schedule.Next(co.Spec.Schedule, last)
	if err != nil {
		r.log.Info("Invalid cron schedule", "error", err, "schedule", co.Spec.Schedule)
		err = errors.Wrapf(err, "cannot parse cron schedule %q", co.Spec.Schedule)
		r.record.Event(co, event.Warning(reasonInvalidSchedule, err))
		status.MarkConditions(xpv1.ReconcileError(err))

		// We don't return the underlying error here because it's
		// terminal. There's no point requeuing until someone fixes it.
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, co), "cannot update CronOperation status")
	}

	// If the next scheduled operation is in the future, requeue for then.
	now := time.Now()
	if next.After(now) {
		r.log.Debug("Next scheduled Operation is in the future - doing nothing", "scheduled-time", next)
		return reconcile.Result{RequeueAfter: next.Sub(now)}, errors.Wrap(r.client.Status().Update(ctx, co), "cannot update CronOperation status")
	}

	// Figure out the next scheduled operation that's in the future. We know
	// we won't hit an error parsing the schedule because it worked above.
	future, _ := r.schedule.Next(co.Spec.Schedule, now)

	// If the next scheduled operation is in the past, but we missed its
	// deadline, requeue in time for the first one scheduled in the future.
	if deadline := co.Spec.StartingDeadlineSeconds; deadline != nil {
		grace := time.Duration(*deadline) * time.Second
		if next.Add(grace).Before(now) {
			r.log.Debug("Missed deadline for scheduled Operation - doing nothing", "scheduled-time", next, "deadline", next.Add(grace))
			return reconcile.Result{RequeueAfter: future.Sub(now)}, errors.Wrap(r.client.Status().Update(ctx, co), "cannot update CronOperation status")
		}
	}

	// At this point we know we're due to create an operation.

	if len(running) > 0 {
		switch p := ptr.Deref(co.Spec.ConcurrencyPolicy, v1alpha1.ConcurrencyPolicyAllow); p {
		case v1alpha1.ConcurrencyPolicyAllow:
			r.log.Debug("Concurrency policy allows creating scheduled Operation while other Operations are running", "policy", p, "running", len(running))
		case v1alpha1.ConcurrencyPolicyForbid:
			r.log.Debug("Concurrency policy forbids creating scheduled Operation while other Operations are running", "policy", p, "running", len(running))
			return reconcile.Result{RequeueAfter: future.Sub(now)}, errors.Wrap(r.client.Status().Update(ctx, co), "cannot update CronOperation status")
		case v1alpha1.ConcurrencyPolicyReplace:
			r.log.Debug("Concurrency policy requires deleting other running Operations", "policy", p, "running", len(running))
			for _, op := range ol.Items {
				if !running[op.GetName()] {
					continue
				}
				if err := r.client.Delete(ctx, &op); resource.IgnoreNotFound(err) != nil {
					log.Debug("Cannot delete running Operation", "error", err, "operation", op.GetName())
					err = errors.Wrapf(err, "cannot delete running Operation %q", op.GetName())
					r.record.Event(co, event.Warning(reasonDeleteOperation, err))
					status.MarkConditions(xpv1.ReconcileError(err))
					_ = r.client.Status().Update(ctx, co)
					return reconcile.Result{}, err
				}

				r.log.Debug("Deleted running operation due to concurrency policy", "policy", p, "operation", op.GetName())
			}
		}
	}

	op := NewOperation(co, next)
	if err := r.client.Create(ctx, op); err != nil {
		log.Debug("Cannot create scheduled Operation", "error", err, "operation", op.GetName())
		err = errors.Wrapf(err, "cannot create scheduled Operation %q", op.GetName())
		r.record.Event(co, event.Warning(reasonCreateOperation, err))
		status.MarkConditions(xpv1.ReconcileError(err))
		_ = r.client.Status().Update(ctx, co)
		return reconcile.Result{}, err
	}

	// We rely on our watch to add the new Operation to status at the top of
	// the Reconcile.

	status.MarkConditions(xpv1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: future.Sub(now)}, errors.Wrap(r.client.Status().Update(ctx, co), "cannot update CronOperation status")
}

// NewOperation creates a new operation given the CronOperation's template.
func NewOperation(co *v1alpha1.CronOperation, scheduled time.Time) *v1alpha1.Operation {
	op := &v1alpha1.Operation{
		ObjectMeta: co.Spec.OperationTemplate.ObjectMeta,
		Spec:       co.Spec.OperationTemplate.Spec,
	}

	op.SetName(fmt.Sprintf("%s-%d", co.GetName(), scheduled.Unix()))
	meta.AddLabels(op, map[string]string{v1alpha1.LabelCronOperationName: co.GetName()})

	av, k := v1alpha1.CronOperationGroupVersionKind.ToAPIVersionAndKind()
	meta.AddOwnerReference(op, meta.AsController(&xpv1.TypedReference{
		APIVersion: av,
		Kind:       k,
		Name:       co.GetName(),
		UID:        co.GetUID(),
	}))

	return op
}
