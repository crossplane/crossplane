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

// Package operation implements day two operations.
package operation

import (
	"context"
	"fmt"
	"slices"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/proto/fn/v1"
)

const timeout = 2 * time.Minute

// DefaultRetryLimit before an Operation is marked failed.
const DefaultRetryLimit = 5

// Event reasons.
const (
	reasonRunPipelineStep    = "RunPipelineStep"
	reasonMaxFailures        = "MaxFailures"
	reasonFunctionInvocation = "FunctionInvocation"
	reasonInvalidOutput      = "InvalidOutput"
	reasonInvalidResource    = "InvalidResource"
	reasonInvalidPipeline    = "InvalidPipeline"
)

// FieldOwnerPrefix is used to form the server-side apply field owner
// that owns the fields this controller mutates on desired resources.
const FieldOwnerPrefix = "ops.crossplane.io/operation/"

// A Reconciler reconciles Operations.
type Reconciler struct {
	client client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	pipeline  xfn.FunctionRunner
	functions xfn.CapabilityChecker
}

// Reconcile an Operation by running its function pipeline.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are typically complex.
	log := r.log.WithValues("request", req)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	op := &v1alpha1.Operation{}
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug("cannot get Operation", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get Operation")
	}

	status := r.conditions.For(op)

	log = log.WithValues(
		"uid", op.GetUID(),
		"version", op.GetResourceVersion(),
		"name", op.GetName(),
		"namespace", op.GetNamespace(),
	)

	// Don't reconcile the operation if it's being deleted.
	if meta.WasDeleted(op) {
		return reconcile.Result{Requeue: false}, nil
	}

	// We only want to run this Operation to completion once.
	if op.IsComplete() {
		log.Debug("Operation is already complete. Nothing to do.")
		return reconcile.Result{Requeue: false}, nil
	}

	// Don't run if we're at the configured failure limit.
	limit := ptr.Deref(op.Spec.RetryLimit, DefaultRetryLimit)
	if op.Status.Failures >= limit {
		log.Debug("Operation failure limit reached. Not running again.", "limit", limit)
		status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.Failed(fmt.Sprintf("failure limit of %d reached", limit)))

		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
	}

	// Updating this status condition ensures we're reconciling the latest
	// version of the Operation. The update would be rejected if we were
	// reconciling a stale version. This is important because it helps us
	// make sure the Operation really isn't complete. That's why we do it
	// every time, instead of only if the Operation isn't already running.
	status.MarkConditions(v1alpha1.Running())
	if err := r.client.Status().Update(ctx, op); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Operation status")
	}

	// Check that all functions in the pipeline have the operation capability
	// before running any function.
	names := make([]string, 0, len(op.Spec.Pipeline))
	for _, fn := range op.Spec.Pipeline {
		names = append(names, fn.FunctionRef.Name)
	}

	// This could need human intervention to fix. It could also be a new
	// function that hasn't written its capabilities to its FunctionRevision
	// status yet, so we retry.
	//
	// We don't watch FunctionRevisions because the watch would trigger
	// instant reconciles whenever the FunctionRevisions change. We always
	// want to retry Operations with a predictable exponential backoff, so
	// we just return an error and let controller-runtime requeue us.
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityOperation}, names...); err != nil {
		op.Status.Failures++

		log.Debug("Function capability check failed", "error", err, "failures", op.Status.Failures)
		err = errors.Wrap(err, "function capability check failed")
		r.record.Event(op, event.Warning(reasonInvalidPipeline, err))
		status.MarkConditions(xpv1.ReconcileError(err), v1alpha1.MissingCapabilities(err.Error()))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	// All functions have the required operation capability
	status.MarkConditions(v1alpha1.ValidPipeline())

	// The function pipeline starts with empty desired state.
	d := &fnv1.State{}

	// The function context starts empty.
	fctx := &structpb.Struct{Fields: map[string]*structpb.Value{}}

	// Run any operation functions in the pipeline. Each function may mutate
	// the desired state returned by the last, and each function may produce
	// results that will be emitted as events.
	for _, fn := range op.Spec.Pipeline {
		log = log.WithValues("step", fn.Step)

		req := &fnv1.RunFunctionRequest{Desired: d, Context: fctx}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				log.Debug("Cannot unmarshal input for operation pipeline step", "error", err)

				// An unmarshalable input requires human intervention to fix, so
				// we immediately fail this operation without retrying.
				status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.Failed(fmt.Sprintf("cannot unmarshal input for operation pipeline step %q", fn.Step)))
				_ = r.client.Status().Update(ctx, op)

				return reconcile.Result{}, errors.Wrapf(err, "cannot unmarshal input for operation pipeline step %q", fn.Step)
			}

			req.Input = in
		}

		req.Credentials = map[string]*fnv1.Credentials{}
		for _, cs := range fn.Credentials {
			// For now we only support loading credentials from secrets.
			if cs.Source != v1alpha1.FunctionCredentialsSourceSecret || cs.SecretRef == nil {
				continue
			}

			s := &corev1.Secret{}
			if err := r.client.Get(ctx, client.ObjectKey{Namespace: cs.SecretRef.Namespace, Name: cs.SecretRef.Name}, s); err != nil {
				op.Status.Failures++

				log.Debug("Cannot get Operation pipeline step credential", "error", err, "failures", op.Status.Failures, "credential", cs.Name)
				err = errors.Wrapf(err, "cannot get operation pipeline step %q credential %q from Secret", fn.Step, cs.Name)
				r.record.Event(op, event.Warning(reasonFunctionInvocation, err))
				status.MarkConditions(xpv1.ReconcileError(err))
				_ = r.client.Status().Update(ctx, op)

				return reconcile.Result{}, err
			}

			req.Credentials[cs.Name] = &fnv1.Credentials{
				Source: &fnv1.Credentials_CredentialData{
					CredentialData: &fnv1.CredentialData{
						Data: s.Data,
					},
				},
			}
		}

		req.Meta = &fnv1.RequestMeta{Tag: xfn.Tag(req)}

		rsp, err := r.pipeline.RunFunction(ctx, fn.FunctionRef.Name, req)
		if err != nil {
			op.Status.Failures++

			log.Debug("Cannot run operation pipeline step", "error", err, "failures", op.Status.Failures)
			err = errors.Wrapf(err, "failed to invoke pipeline step %q", fn.Step)
			r.record.Event(op, event.Warning(reasonFunctionInvocation, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}

		// Pass the desired state returned by this Function to the next one.
		d = rsp.GetDesired()

		// Pass the Function context returned by this Function to the next one.
		// We intentionally discard/ignore this after the last Function runs.
		fctx = rsp.GetContext()

		// Results of fatal severity stop the Operation. Other results are
		// emitted as events.
		for _, rs := range rsp.GetResults() {
			switch rs.GetSeverity() {
			case fnv1.Severity_SEVERITY_FATAL:
				op.Status.Failures++

				log.Debug("Pipeline step returned a fatal result", "error", rs.GetMessage(), "failures", op.Status.Failures)
				err = errors.New(rs.GetMessage())
				r.record.Event(op, event.Warning(reasonFunctionInvocation, err))
				status.MarkConditions(xpv1.ReconcileError(err))
				_ = r.client.Status().Update(ctx, op)

				return reconcile.Result{}, err
			case fnv1.Severity_SEVERITY_WARNING:
				r.record.Event(op, event.Warning(reasonRunPipelineStep, errors.Errorf("Pipeline step %q: %s", fn.Step, rs.GetMessage())))
			case fnv1.Severity_SEVERITY_NORMAL:
				r.record.Event(op, event.Normal(reasonRunPipelineStep, fmt.Sprintf("Pipeline step %q: %s", fn.Step, rs.GetMessage())))
			case fnv1.Severity_SEVERITY_UNSPECIFIED:
				// We could hit this case if a Function was built against a newer
				// protobuf than this build of Crossplane, and the new protobuf
				// introduced a severity that we don't know about.
				r.record.Event(op, event.Warning(reasonRunPipelineStep, errors.Errorf("Pipeline step %q returned a result of unknown severity (assuming warning): %s", fn.Step, rs.GetMessage())))
			}
		}

		if o := rsp.GetOutput(); o != nil {
			j, err := protojson.Marshal(o)
			if err != nil {
				op.Status.Failures++

				log.Debug("Cannot marshal pipeline step output to JSON", "error", err, "failures", op.Status.Failures)
				err = errors.Wrapf(err, "cannot marshal pipeline step %q output to JSON", fn.Step)
				r.record.Event(op, event.Warning(reasonInvalidOutput, err))
				status.MarkConditions(xpv1.ReconcileError(err))
				_ = r.client.Status().Update(ctx, op)

				return reconcile.Result{}, err
			}

			op.Status.Pipeline = AddPipelineStepOutput(op.Status.Pipeline, fn.Step, &runtime.RawExtension{Raw: j})
		}
	}

	// Now that all functions have run, we want to apply any desired
	// resources the pipeline produced.
	for name, dr := range d.GetResources() {
		u := &kunstructured.Unstructured{}
		if err := xfn.FromStruct(u, dr.GetResource()); err != nil {
			op.Status.Failures++

			log.Debug("Cannot load desired resource from protobuf struct", "error", err, "failures", op.Status.Failures, "resource-name", name)
			err = errors.Wrapf(err, "cannot load desired resource %q from protobuf struct", name)
			r.record.Event(op, event.Warning(reasonInvalidResource, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}

		// TODO(negz): Do we really want to force ownership? We'll
		// always be operating on a resource some other controller owns.
		// TODO(negz): Do we ever want to be an owner reference of these
		// resources?
		if err := r.client.Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerPrefix+op.GetUID())); err != nil {
			op.Status.Failures++
			log.Debug("Cannot apply desired resource", "error", err, "failures", op.Status.Failures, "resource-name", name)

			err = errors.Wrap(err, "cannot apply desired resource")
			r.record.Event(op, event.Warning(reasonInvalidResource, err))
			status.MarkConditions(xpv1.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}

		// TODO(negz): A pipeline could overflow this if it returned
		// hundreds of desired resources. We could switch to a plain
		// count, but it's pretty useful to know what resources an
		// Operation applied...
		op.Status.AppliedResourceRefs = AddResourceRef(op.Status.AppliedResourceRefs, u)
	}

	status.MarkConditions(xpv1.ReconcileSuccess(), v1alpha1.Complete())

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
}

// AddResourceRef adds a reference to the supplied resource to supplied
// references. It only adds resources that aren't already referenced, and keeps
// the references sorted.
func AddResourceRef(refs []v1alpha1.AppliedResourceRef, u *kunstructured.Unstructured) []v1alpha1.AppliedResourceRef {
	ref := v1alpha1.AppliedResourceRef{
		APIVersion: u.GetAPIVersion(),
		Kind:       u.GetKind(),
		Name:       u.GetName(),
	}
	if u.GetNamespace() != "" {
		ref.Namespace = ptr.To(u.GetNamespace())
	}

	// Don't add the new ref if it's already there.
	for _, existing := range refs {
		if existing.Equals(ref) {
			return refs
		}
	}

	refs = append(refs, ref)

	slices.SortStableFunc(refs, func(a, b v1alpha1.AppliedResourceRef) int {
		sa := a.APIVersion + a.Kind + ptr.Deref(a.Namespace, "") + a.Name
		sb := b.APIVersion + b.Kind + ptr.Deref(b.Namespace, "") + b.Name

		if sa == sb {
			return 0
		}

		if sa > sb {
			return 1
		}

		return -1
	})

	return refs
}

// AddPipelineStepOutput updates the output for a pipeline step in the
// supplied pipeline status slice. If the step already exists, its output is
// updated in place. If it doesn't exist, it's appended to the slice. The input
// slice is assumed to be sorted by step name.
func AddPipelineStepOutput(pipeline []v1alpha1.PipelineStepStatus, step string, output *runtime.RawExtension) []v1alpha1.PipelineStepStatus {
	for i, ps := range pipeline {
		if ps.Step == step {
			pipeline[i].Output = output
			return pipeline
		}
	}

	return append(pipeline, v1alpha1.PipelineStepStatus{
		Step:   step,
		Output: output,
	})
}
