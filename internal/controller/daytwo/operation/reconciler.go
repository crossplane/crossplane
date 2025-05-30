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
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/apis/daytwo/v1alpha1"
	"github.com/crossplane/crossplane/internal/xfn"
)

// MaxRequirementsIterations is the maximum number of times a function should be
// called, limiting the number of times it can request for extra resources,
// capped for safety.
const MaxRequirementsIterations = 5

const reconcileTimeout = 1 * time.Minute

const (
	reasonRunPipelineStep    = "RunPipelineStep"
	reasonMaxFailures        = "MaxFailures"
	reasonFunctionInvocation = "FunctionInvocation"
	reasonExtraResources     = "ExtraResources"
	reasonInvalidOutput      = "InvalidOutput"
)

// A Reconciler reconciles Operations.
type Reconciler struct {
	client client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	pipeline  xfn.FunctionRunner
	resources xfn.ExtraResourcesFetcher
}

// Reconcile an Operation by running its function pipeline.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are typically complex.
	log := r.log.WithValues("request", req)
	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
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
	limit := ptr.Deref(op.Spec.FailureLimit, 5)
	if op.Status.Failures >= limit {
		log.Debug("Operation failure limit reached. Not running again.", "limit", limit)
		status.MarkConditions(v1alpha1.Failed("failure limit of %d reached", limit))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
	}

	// Updating this status condition ensures we're reconciling the latest
	// version of the Operation. This is important because it helps us make sure
	// the Operation really isn't complete.
	status.MarkConditions(v1alpha1.Running())
	if err := r.client.Status().Update(ctx, op); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update Operation status")
	}

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
				status.MarkConditions(v1alpha1.Failed("cannot unmarshal input for operation pipeline step %q", fn.Step))
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}
			req.Input = in
		}

		// Used to store the requirements returned at the previous iteration.
		var reqs *fnv1.Requirements

		// Used to store the response of the function at the previous iteration.
		var rsp *fnv1.RunFunctionResponse

		for i := int64(0); i <= MaxRequirementsIterations; i++ {
			if i == MaxRequirementsIterations {
				log.Debug("Function requirements didn't stabilize after the maximum number of iterations")
				op.Status.Failures++
				r.record.Event(op, event.Warning(reasonMaxFailures,
					errors.New("maximum number of iterations reached"),
					"failures", fmt.Sprintf("%d", op.Status.Failures),
					"step", fn.Step))
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}

			// TODO(negz): Generate a content-addressable tag for this request.
			// Perhaps using https://github.com/cerbos/protoc-gen-go-hashpb ?
			var err error
			rsp, err = r.pipeline.RunFunction(ctx, fn.FunctionRef.Name, req)
			if err != nil {
				log.Debug("Cannot run operation pipeline step", "error", err)
				op.Status.Failures++
				r.record.Event(op, event.Warning(reasonFunctionInvocation,
					errors.Wrap(err, "failed to invoke pipeline step"),
					"failures", fmt.Sprintf("%d", op.Status.Failures),
					"step", fn.Step))
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}

			// If the requirements stabilized, the function is done.
			if cmp.Equal(reqs, rsp.GetRequirements()) {
				break
			}

			// Store the requirements for the next iteration.
			reqs = rsp.GetRequirements()

			// Cleanup the extra resources from the previous iteration.
			req.ExtraResources = make(map[string]*fnv1.Resources)

			// Fetch the requested resources and add them to the desired state.
			for name, selector := range reqs.GetExtraResources() {
				rs, err := r.resources.Fetch(ctx, selector)
				if err != nil {
					log.Debug("Cannot fetch extra resources", "error", err, "resources", name)
					op.Status.Failures++
					r.record.Event(op, event.Warning(reasonExtraResources,
						errors.Wrap(err, "failed to fetch extra resources"),
						"failures", fmt.Sprintf("%d", op.Status.Failures),
						"step", fn.Step,
						"selector", selector.String(),
					))
					return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
				}

				// Resources would be nil in case of not found resources.
				req.ExtraResources[name] = rs
			}

			// Pass down the updated context across iterations.
			req.Context = rsp.GetContext()
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
				log.Debug("Pipeline step returned a fatal result", "error", rs.GetMessage())
				op.Status.Failures++
				r.record.Event(op, event.Warning(reasonFunctionInvocation,
					errors.New(rs.GetMessage()),
					"failures", fmt.Sprintf("%d", op.Status.Failures),
					"step", fn.Step))
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
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
				log.Debug("Cannot marshal pipeline step output to JSON", "error", err)
				op.Status.Failures++
				r.record.Event(op, event.Warning(reasonInvalidOutput,
					errors.Wrap(err, "failed to marshal pipeline step output to JSON"),
					"failures", fmt.Sprintf("%d", op.Status.Failures),
				))
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}
			op.Status.Pipeline = append(op.Status.Pipeline, v1alpha1.PipelineStepStatus{
				Step:   fn.Step,
				Output: &runtime.RawExtension{Raw: j},
			})
		}
	}

	status.MarkConditions(v1alpha1.Complete())
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
}
