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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	fnv1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/apis/daytwo/v1alpha1"
	daytwocontroller "github.com/crossplane/crossplane/internal/controller/daytwo/controller"
)

// TODO: DO NOT MERGE. Pending Tasks...
// - [ ] Update to use status.ObservedGeneration
// - [ ] Define sub-conditions for Operation.
// - [ ] Implement a Marker.
// - [ ] Add an e2e test.
// - [ ] Unit test needs to confirm status update from proto resp.Output, etc.
//

// MaxRequirementsIterations is the maximum number of times a function should be
// called, limiting the number of times it can request for extra resources,
// capped for safety.
const MaxRequirementsIterations = 5

const reconcileTimeout = 1 * time.Minute

// Setup adds a controller that reconciles Usages by
// defining a composite resource and starting a controller to reconcile it.
func Setup(mgr ctrl.Manager, o daytwocontroller.Options) error {
	name := "usage/" + strings.ToLower(v1alpha1.OperationGroupKind)
	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithFunctionRunner(o.FunctionRunner))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Operation{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// A FunctionRunner runs a single operation function.
type FunctionRunner interface {
	// RunFunction runs the named operation function.
	RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)
}

// A FunctionRunnerFn is a function that can run an operation function.
type FunctionRunnerFn func(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error)

// RunFunction runs the named Composition Function with the supplied request.
func (fn FunctionRunnerFn) RunFunction(ctx context.Context, name string, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	return fn(ctx, name, req)
}

// A ExtraResourcesFetcher gets extra resources matching a selector.
type ExtraResourcesFetcher interface {
	Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)
}

// An ExtraResourcesFetcherFn gets extra resources matching the selector.
type ExtraResourcesFetcherFn func(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error)

// Fetch gets extra resources matching the selector.
func (fn ExtraResourcesFetcherFn) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	return fn(ctx, rs)
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

// WithFunctionRunner specifies how the Reconciler should run functions.
func WithFunctionRunner(fr FunctionRunner) ReconcilerOption {
	return func(r *Reconciler) {
		r.pipeline = fr
	}
}

// NewReconciler returns a Reconciler of Usages.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles Operations.
type Reconciler struct {
	client client.Client

	log    logging.Logger
	record event.Recorder

	pipeline  FunctionRunner
	resources ExtraResourcesFetcher
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

	log = log.WithValues(
		"uid", op.GetUID(),
		"version", op.GetResourceVersion(),
		"name", op.GetName(),
	)

	// Don't reconcile the operation if it's being deleted.
	if meta.WasDeleted(op) {
		return reconcile.Result{Requeue: false}, nil
	}

	// We only want to run this Operation to completion once.
	if op.Status.GetCondition(v1alpha1.TypeComplete).Status == corev1.ConditionTrue {
		log.Debug("Operation is already complete. Nothing to do.")
		return reconcile.Result{Requeue: false}, nil
	}

	// Don't run if we're at the configured failure limit.
	limit := ptr.Deref(op.Spec.FailureLimit, 5)
	if op.Status.Failures >= limit {
		log.Debug("Operation failure limit reached. Not running again.", "limit", limit)
		op.Status.SetConditions(v1alpha1.Complete(), v1alpha1.Failed())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
	}

	// Updating this status condition ensures we're reconciling the latest
	// version of the Operation. This is important because it helps us make sure
	// the Operation really isn't complete.
	op.Status.SetConditions(v1alpha1.Running())
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
				op.Status.SetConditions(v1alpha1.Complete(), v1alpha1.Failed())
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
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}

			// TODO(negz): Generate a content-addressable tag for this request.
			// Perhaps using https://github.com/cerbos/protoc-gen-go-hashpb ?
			rsp, err := r.pipeline.RunFunction(ctx, fn.FunctionRef.Name, req)
			if err != nil {
				log.Debug("Cannot run operation pipeline step", "error", err)
				op.Status.Failures++
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
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			case fnv1.Severity_SEVERITY_WARNING:
				r.record.Event(op, event.Warning("RunPipelineStep", errors.Errorf("Pipeline step %q: %s", fn.Step, rs.GetMessage())))
			case fnv1.Severity_SEVERITY_NORMAL:
				r.record.Event(op, event.Normal("RunPipelineStep", fmt.Sprintf("Pipeline step %q: %s", fn.Step, rs.GetMessage())))
			case fnv1.Severity_SEVERITY_UNSPECIFIED:
				// We could hit this case if a Function was built against a newer
				// protobuf than this build of Crossplane, and the new protobuf
				// introduced a severity that we don't know about.
				r.record.Event(op, event.Warning("RunPipelineStep", errors.Errorf("Pipeline step %q returned a result of unknown severity (assuming warning): %s", fn.Step, rs.GetMessage())))
			}
		}

		if o := rsp.GetOutput(); o != nil {
			j, err := protojson.Marshal(o)
			if err != nil {
				log.Debug("Cannot marshal pipeline step output to JSON", "error", err)
				op.Status.Failures++
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
			}
			op.Status.Pipeline = append(op.Status.Pipeline, v1alpha1.PipelineStepStatus{
				Step:   fn.Step,
				Output: &runtime.RawExtension{Raw: j},
			})
		}
	}

	// TODO(negz): Process desired state (i.e. SSA resources).
	// TODO(negz): Emit events when we hit errors.
	// TODO(negz): Can we write errors to some status condition? Running?

	op.Status.SetConditions(v1alpha1.Complete())
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
}

// An ExistingExtraResourcesFetcher fetches extra resources requested by
// functions using the provided client.Reader.
type ExistingExtraResourcesFetcher struct {
	client client.Reader
}

// NewExistingExtraResourcesFetcher returns a new ExistingExtraResourcesFetcher.
func NewExistingExtraResourcesFetcher(c client.Reader) *ExistingExtraResourcesFetcher {
	return &ExistingExtraResourcesFetcher{client: c}
}

// Fetch fetches resources requested by functions using the provided client.Reader.
func (e *ExistingExtraResourcesFetcher) Fetch(ctx context.Context, rs *fnv1.ResourceSelector) (*fnv1.Resources, error) {
	if rs == nil {
		return nil, errors.New("resource selector must not be nil")
	}

	switch match := rs.GetMatch().(type) {
	case *fnv1.ResourceSelector_MatchName:
		// Fetch a single resource.
		r := &unstructured.Unstructured{}
		r.SetAPIVersion(rs.GetApiVersion())
		r.SetKind(rs.GetKind())

		err := e.client.Get(ctx, types.NamespacedName{Name: rs.GetMatchName()}, r)
		if kerrors.IsNotFound(err) {
			// The resource doesn't exist. We'll return nil, which the Functions
			// know means that the resource was not found.
			return nil, nil
		}
		if err != nil {
			return nil, errors.Wrap(err, "cannot get extra resource by name")
		}

		o, err := AsStruct(r)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert extra resource to protobuf Struct")
		}
		return &fnv1.Resources{Items: []*fnv1.Resource{{Resource: o}}}, nil

	case *fnv1.ResourceSelector_MatchLabels:
		// Fetch a list of resources.
		list := &unstructured.UnstructuredList{}
		list.SetAPIVersion(rs.GetApiVersion())
		list.SetKind(rs.GetKind())

		if err := e.client.List(ctx, list, client.MatchingLabels(match.MatchLabels.GetLabels())); err != nil {
			return nil, errors.Wrap(err, "cannot list extra resources")
		}

		rs := make([]*fnv1.Resource, len(list.Items))
		for i := range list.Items {
			o, err := AsStruct(&list.Items[i])
			if err != nil {
				return nil, errors.Wrap(err, "cannot convert extra resource to protobuf Struct")
			}
			rs[i] = &fnv1.Resource{Resource: o}
		}

		return &fnv1.Resources{Items: rs}, nil
	}

	return nil, errors.New("unsupported resource selector type")
}

// AsStruct converts the supplied object to a protocol buffer Struct well-known
// type.
func AsStruct(o runtime.Object) (*structpb.Struct, error) {
	// If the supplied object is *Unstructured we don't need to round-trip.
	if u, ok := o.(interface {
		UnstructuredContent() map[string]interface{}
	}); ok {
		s, err := structpb.NewStruct(u.UnstructuredContent())
		return s, errors.Wrap(err, "cannot create protobuf Struct from Unstructured resource")
	}

	// Fall back to a JSON round-trip.
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal object to JSON")
	}

	s := &structpb.Struct{}
	return s, errors.Wrap(s.UnmarshalJSON(b), "cannot unmarshal JSON to protobuf Struct")
}
