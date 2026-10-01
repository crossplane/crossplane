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
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/step"
	"github.com/crossplane/crossplane/v2/internal/xfn"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

const timeout = 2 * time.Minute

// DefaultRetryLimit before an Operation is marked failed.
const DefaultRetryLimit = 5

// Event reasons.
const (
	reasonRunPipelineStep       = "RunPipelineStep"
	reasonFunctionInvocation    = "FunctionInvocation"
	reasonInvalidOutput         = "InvalidOutput"
	reasonInvalidResource       = "InvalidResource"
	reasonInvalidPipeline       = "InvalidPipeline"
	reasonBootstrapRequirements = "BootstrapRequirements"
	reasonInstallFunctions      = "InstallFunctions"
	reasonCheckCapabilities     = "CheckCapabilities"
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
	resources xfn.RequiredResourcesFetcher
	schemas   xfn.RequiredSchemasFetcher
}

// Reconcile an Operation by running its function pipeline.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are typically complex.
	ctx = step.ForOperations(ctx)
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
		status.MarkConditions(xpv2.ReconcileSuccess(), v1alpha1.Failed(fmt.Sprintf("failure limit of %d reached", limit)))

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

	if err := r.ensureFunctions(ctx, op); err != nil {
		op.Status.Failures++

		log.Debug("Cannot ensure functions for pipeline", "error", err)
		r.record.Event(op, event.Warning(reasonInstallFunctions, err))
		status.MarkConditions(xpv2.ReconcileError(err))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	// Collect OCI refs for all functions in the pipeline so we can check their
	// capabilities. We'll re-use these refs for calling the functions later, so
	// it's important that their indices match the step indices.
	refs := make([]string, len(op.Spec.Pipeline))
	for i, step := range op.Spec.Pipeline {
		if step.Function != "" {
			refs[i] = step.Function
			continue
		}
		f := &pkgv1.Function{}
		if err := r.client.Get(ctx, client.ObjectKey{Name: step.FunctionRef.Name}, f); err != nil {
			log.Debug("Cannot get Function for pipeline step", "error", err)
			r.record.Event(op, event.Warning(reasonCheckCapabilities, err))
			status.MarkConditions(xpv2.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}
		refs[i] = f.Spec.Package
	}

	// This could need human intervention to fix. It could also be a new
	// function that hasn't written its capabilities to its FunctionRevision
	// status yet, so we retry.
	//
	// We don't watch FunctionRevisions because the watch would trigger
	// instant reconciles whenever the FunctionRevisions change. We always
	// want to retry Operations with a predictable exponential backoff, so
	// we just return an error and let controller-runtime requeue us.
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityOperation}, refs...); err != nil {
		op.Status.Failures++

		log.Debug("Function capability check failed", "error", err, "failures", op.Status.Failures)
		err = errors.Wrap(err, "function capability check failed")
		r.record.Event(op, event.Warning(reasonInvalidPipeline, err))
		status.MarkConditions(xpv2.ReconcileError(err), v1alpha1.MissingCapabilities(err.Error()))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	// All functions have the required operation capability
	status.MarkConditions(v1alpha1.ValidPipeline())

	// The function pipeline starts with empty desired state.
	d := &fnv1.State{}

	// The function context starts empty.
	fctx := &structpb.Struct{Fields: map[string]*structpb.Value{}}

	// Generate a trace ID for this pipeline execution. All steps in this
	// reconciliation will share this trace ID for correlation.
	traceID := uuid.NewString()

	// Run any operation functions in the pipeline. Each function may mutate
	// the desired state returned by the last, and each function may produce
	// results that will be emitted as events.
	for stepIndex, fn := range op.Spec.Pipeline {
		log = log.WithValues("step", fn.Step)

		req := &fnv1.RunFunctionRequest{Desired: d, Context: fctx}

		if fn.Input != nil {
			in := &structpb.Struct{}
			if err := in.UnmarshalJSON(fn.Input.Raw); err != nil {
				log.Debug("Cannot unmarshal input for operation pipeline step", "error", err)

				// An unmarshalable input requires human intervention to fix, so
				// we immediately fail this operation without retrying.
				status.MarkConditions(xpv2.ReconcileSuccess(), v1alpha1.Failed(fmt.Sprintf("cannot unmarshal input for operation pipeline step %q", fn.Step)))
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
				status.MarkConditions(xpv2.ReconcileError(err))
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

		// Pre-populate bootstrap requirements
		if fn.Requirements != nil {
			// Bootstrap requirements were introduced alongside the new field names,
			// so we only need to support the new required_resources field.
			req.RequiredResources = map[string]*fnv1.Resources{}
			for _, sel := range fn.Requirements.RequiredResources {
				resources, err := r.resources.Fetch(ctx, xfn.ToProtobufResourceSelector(&sel))
				if err != nil {
					op.Status.Failures++

					log.Debug("Cannot fetch bootstrap required resources", "error", err, "failures", op.Status.Failures, "requirement", sel.RequirementName)
					err = errors.Wrapf(err, "cannot fetch bootstrap required resources for requirement %q", sel.RequirementName)
					r.record.Event(op, event.Warning(reasonBootstrapRequirements, err))
					status.MarkConditions(xpv2.ReconcileError(err))
					_ = r.client.Status().Update(ctx, op)

					return reconcile.Result{}, err
				}

				// Add to request (resources could be nil if not found)
				req.RequiredResources[sel.RequirementName] = resources
			}

			req.RequiredSchemas = map[string]*fnv1.Schema{}
			for _, sel := range fn.Requirements.RequiredSchemas {
				schema, err := r.schemas.Fetch(ctx, xfn.ToProtobufSchemaSelector(&sel))
				if err != nil {
					op.Status.Failures++

					log.Debug("Cannot fetch bootstrap required schema", "error", err, "failures", op.Status.Failures, "requirement", sel.RequirementName)
					err = errors.Wrapf(err, "cannot fetch bootstrap required schema for requirement %q", sel.RequirementName)
					r.record.Event(op, event.Warning(reasonBootstrapRequirements, err))
					status.MarkConditions(xpv2.ReconcileError(err))
					_ = r.client.Status().Update(ctx, op)

					return reconcile.Result{}, err
				}

				req.RequiredSchemas[sel.RequirementName] = schema
			}
		}

		req.Meta = &fnv1.RequestMeta{Tag: xfn.Tag(req), Capabilities: xfn.SupportedCapabilities()}

		// Add step metadata to context for use by downstream components like InspectedRunner.
		stepCtx := step.ContextWithStepMetaForOperations(ctx, traceID, fn.Step, int32(stepIndex), op.GetName(), string(op.GetUID()))

		rsp, err := r.pipeline.RunFunction(stepCtx, refs[stepIndex], req)
		if err != nil {
			op.Status.Failures++

			log.Debug("Cannot run operation pipeline step", "error", err, "failures", op.Status.Failures)
			err = errors.Wrapf(err, "failed to invoke pipeline step %q", fn.Step)
			r.record.Event(op, event.Warning(reasonFunctionInvocation, err))
			status.MarkConditions(xpv2.ReconcileError(err))
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
				err = &xcomposite.PipelineFatalError{Step: fn.Step, Message: rs.GetMessage()}
				r.record.Event(op, event.Warning(reasonFunctionInvocation, err))
				status.MarkConditions(xpv2.ReconcileError(err))
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
				status.MarkConditions(xpv2.ReconcileError(err))
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
			status.MarkConditions(xpv2.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}

		// TODO(negz): Do we really want to force ownership? We'll
		// always be operating on a resource some other controller owns.
		// TODO(negz): Do we ever want to be an owner reference of these
		// resources?
		//
		//nolint:staticcheck // TODO(adamwg): Stop using client.Apply after the v2.2 release.
		if err := r.client.Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner(FieldOwnerPrefix+op.GetUID())); err != nil {
			op.Status.Failures++
			log.Debug("Cannot apply desired resource", "error", err, "failures", op.Status.Failures, "resource-name", name)

			err = errors.Wrap(err, "cannot apply desired resource")
			r.record.Event(op, event.Warning(reasonInvalidResource, err))
			status.MarkConditions(xpv2.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}

		// TODO(negz): A pipeline could overflow this if it returned
		// hundreds of desired resources. We could switch to a plain
		// count, but it's pretty useful to know what resources an
		// Operation applied...
		op.Status.AppliedResourceRefs = AddResourceRef(op.Status.AppliedResourceRefs, u)
	}

	status.MarkConditions(xpv2.ReconcileSuccess(), v1alpha1.Complete())

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, op), "cannot update Operation status")
}

// ensureFunctions ensures that a FunctionRevision and its parent Function exist
// for every pipeline step that references a function by package. Each object
// gains an owner reference to the supplied CompositionRevision, so that the
// objects are garbage collected once no CompositionRevision references them any
// longer.
func (r *Reconciler) ensureFunctions(ctx context.Context, op *v1alpha1.Operation) error {
	owner := meta.AsOwner(meta.TypedReferenceTo(op, v1alpha1.OperationGroupVersionKind))

	// Collect the set of external revisions per parent Function. Multiple steps
	// (or multiple composition revisions) may reference different versions of
	// the same function package, in which case the parent Function ends up with
	// more than one external revision.
	revisionsByFunction := map[string]map[string]bool{}

	for _, step := range op.Spec.Pipeline {
		if step.Function == "" {
			continue
		}

		ref, err := name.NewDigest(step.Function, name.StrictValidation)
		if err != nil {
			return errors.Wrapf(err, "invalid package reference in pipeline step %q; steps must reference functions by digest", step.Step)
		}

		fnName := xpkg.ToDNSLabel(ref.Context().RepositoryStr())
		revName := functionRevisionName(fnName, ref)

		if err := r.ensureFunctionRevision(ctx, revName, fnName, step.Function, owner); err != nil {
			return errors.Wrapf(err, "cannot ensure FunctionRevision for pipeline step %q", step.Step)
		}

		if revisionsByFunction[fnName] == nil {
			revisionsByFunction[fnName] = map[string]bool{}
		}
		revisionsByFunction[fnName][revName] = true
	}

	for fnName, revs := range revisionsByFunction {
		if err := r.ensureFunction(ctx, fnName, revs, owner); err != nil {
			return errors.Wrapf(err, "cannot ensure Function %q", fnName)
		}
	}

	return nil
}

// functionRevisionName derives a FunctionRevision name from a package
// reference. Names are based on the package repository and digest, so that
// multiple operations referencing the same package will share a function
// revision. Note, however, that we construct names differently from the
// package manager, so we will not share revisions with it.
func functionRevisionName(fnName string, ref name.Digest) string {
	return xpkg.FriendlyID(fnName, strings.TrimPrefix(ref.DigestStr(), "sha256:"))
}

// ensureFunctionRevision creates the named FunctionRevision if it does not
// exist, or adds the supplied owner reference to it if it does.
func (r *Reconciler) ensureFunctionRevision(ctx context.Context, revName, fnName, pkg string, owner metav1.OwnerReference) error {
	rev := &pkgv1.FunctionRevision{}
	err := r.client.Get(ctx, client.ObjectKey{Name: revName}, rev)
	if kerrors.IsNotFound(err) {
		rev = &pkgv1.FunctionRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: revName,
				Labels: map[string]string{
					pkgv1.LabelParentPackage: fnName,
					pkgv1.LabelFunction:      fnName,
					pkgv1.LabelRevision:      revName,
				},
				OwnerReferences: []metav1.OwnerReference{owner},
			},
			Spec: pkgv1.FunctionRevisionSpec{
				PackageRevisionSpec: pkgv1.PackageRevisionSpec{
					DesiredState: pkgv1.PackageRevisionActive,
					Package:      pkg,
					Revision:     1,
				},
				PackageRevisionRuntimeSpec: pkgv1.PackageRevisionRuntimeSpec{
					TLSServerSecretName: pkgv1.GetSecretNameWithSuffix(revName, pkgv1.TLSServerSecretNameSuffix),
				},
			},
		}

		return r.client.Create(ctx, rev)
	}
	if err != nil {
		return err
	}

	meta.AddOwnerReference(rev, owner)

	// TODO(adamwg): SSA?
	return r.client.Update(ctx, rev)
}

// ensureFunction creates the named parent Function if it does not exist, or
// updates it to include the supplied owner reference and external revisions if
// it does. The Function has an empty spec.package, which signals the package
// manager not to manage its revisions.
func (r *Reconciler) ensureFunction(ctx context.Context, fnName string, revNames map[string]bool, owner metav1.OwnerReference) error {
	revs := make([]corev1.LocalObjectReference, 0, len(revNames))
	for _, n := range slices.Sorted(maps.Keys(revNames)) {
		revs = append(revs, corev1.LocalObjectReference{Name: n})
	}

	fn := &pkgv1.Function{}
	err := r.client.Get(ctx, client.ObjectKey{Name: fnName}, fn)
	if kerrors.IsNotFound(err) {
		fn = &pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name:            fnName,
				OwnerReferences: []metav1.OwnerReference{owner},
			},
			Spec: pkgv1.FunctionSpec{
				ExternalRevisionRefs: revs,
			},
		}

		return r.client.Create(ctx, fn)
	}

	if err != nil {
		return err
	}

	// Function already exists, but we need to make sure it has all our external
	// revisions.
	for _, rev := range revs {
		if slices.Contains(fn.Spec.ExternalRevisionRefs, rev) {
			continue
		}

		fn.Spec.ExternalRevisionRefs = append(fn.Spec.ExternalRevisionRefs, rev)
	}
	// Ensure the composition revision is an owner of the function, for GC
	// purposes.
	meta.AddOwnerReference(fn, owner)

	// TODO(adamwg): SSA?
	return r.client.Update(ctx, fn)
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
		ref.Namespace = new(u.GetNamespace())
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
