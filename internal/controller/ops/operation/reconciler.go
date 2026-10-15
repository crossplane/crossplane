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
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	xcomposite "github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/step"
	"github.com/crossplane/crossplane/v2/internal/features"
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

const (
	errFmtMissingFunctionRef = "pipeline step %q has no functionRef; referencing functions by OCI reference requires the --enable-pipeline-oci-references alpha feature flag"
	errFmtInvalidOCIRef      = "function for pipeline step %q is not a valid OCI reference"
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

	features *feature.Flags
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

	// Like a capability check failure, this could need human intervention to
	// fix, so we count it as a failure and retry with backoff.
	if err := r.validatePipeline(op); err != nil {
		op.Status.Failures++

		log.Debug("Invalid pipeline", "error", err, "failures", op.Status.Failures)
		r.record.Event(op, event.Warning(reasonInvalidPipeline, err))
		status.MarkConditions(xpv2.ReconcileError(err), v1alpha1.InvalidPipeline(err.Error()))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	if err := r.ensureFunctions(ctx, op); err != nil {
		// A conflict means another Operation or CompositionRevision updated a
		// shared Function or FunctionRevision. An AlreadyExists means we did a
		// stale cache read and something else created a function or revision
		// underneath us. Either way, it's not our failure and a retry should
		// work; don't count it toward the operation's failure limit or emit any
		// events.
		if kerrors.IsConflict(err) || kerrors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		}

		op.Status.Failures++
		log.Debug("Cannot ensure functions for pipeline", "error", err)
		r.record.Event(op, event.Warning(reasonInstallFunctions, err))
		status.MarkConditions(xpv2.ReconcileError(err))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	// Resolve each pipeline step to the FunctionRevision that will run it, so we
	// can check their capabilities. A step that references a function by OCI
	// reference runs the revision we created for it above. A step that
	// references an installed Function by name runs that Function's active
	// revision. We'll re-use these revisions for calling the functions later,
	// so it's important that their indices match the step indices.
	revs := make([]string, len(op.Spec.Pipeline))
	for i, step := range op.Spec.Pipeline {
		rev, err := functionRevisionForStep(ctx, r.client, step)
		if err != nil {
			if step.FunctionRef != nil {
				// Treat a function ref that doesn't resolve to a revision as a
				// failure, since it won't resolve itself (the function doesn't
				// exist or doesn't have an active revision, which is probably
				// something the user needs to fix).
				op.Status.Failures++
			}

			err = errors.Wrapf(err, "cannot resolve FunctionRevision for pipeline step %q", step.Step)
			log.Debug("Cannot resolve FunctionRevision for pipeline step", "error", err)
			r.record.Event(op, event.Warning(reasonCheckCapabilities, err))
			status.MarkConditions(xpv2.ReconcileError(err))
			_ = r.client.Status().Update(ctx, op)

			return reconcile.Result{}, err
		}
		revs[i] = rev
	}

	// Functions we install for this Operation take a while to become ready,
	// especially the first time their package is pulled. Waiting for them isn't
	// a failure, so we retry with backoff without counting it against the retry
	// limit.
	if err := r.functionsReady(ctx, op, revs); err != nil {
		log.Debug("Waiting for functions to become ready", "error", err)
		r.record.Event(op, event.Normal(reasonInstallFunctions, err.Error()))
		status.MarkConditions(xpv2.ReconcileError(err))
		_ = r.client.Status().Update(ctx, op)

		return reconcile.Result{}, err
	}

	// This could need human intervention to fix. It could also be a new
	// function that hasn't written its capabilities to its FunctionRevision
	// status yet, so we retry.
	//
	// We don't watch FunctionRevisions because the watch would trigger
	// instant reconciles whenever the FunctionRevisions change. We always
	// want to retry Operations with a predictable exponential backoff, so
	// we just return an error and let controller-runtime requeue us.
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityOperation}, revs...); err != nil {
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

		rsp, err := r.pipeline.RunFunction(stepCtx, revs[stepIndex], req)
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

// functionRevisionForStep returns the name of the FunctionRevision that should
// run the supplied Operation pipeline step.
func functionRevisionForStep(ctx context.Context, c client.Reader, s v1alpha1.PipelineStep) (string, error) {
	if s.Function != "" {
		return xcomposite.ExternalFunctionRevision(ctx, c, s.Function)
	}

	if s.FunctionRef == nil {
		return "", errors.Errorf("pipeline step %s is invalid: missing both function and functionRef", s.Step)
	}

	return xcomposite.ActiveFunctionRevision(ctx, c, s.FunctionRef.Name)
}

// validatePipeline validates the steps in the operation's pipeline and returns
// an error if any step is invalid.
func (r *Reconciler) validatePipeline(op *v1alpha1.Operation) error {
	if r.features.Enabled(features.EnableAlphaPipelineOCIReferences) {
		for _, s := range op.Spec.Pipeline {
			if s.Function == "" {
				if s.FunctionRef == nil {
					return errors.Errorf("pipeline step %s specifies neither function nor functionRef", s.Step)
				}
				continue
			}
			if _, err := name.NewDigest(s.Function, name.StrictValidation); err != nil {
				return errors.Wrapf(err, errFmtInvalidOCIRef, s.Step)
			}
		}
	} else {
		for _, s := range op.Spec.Pipeline {
			if s.FunctionRef == nil {
				return errors.Errorf(errFmtMissingFunctionRef, s.Step)
			}
		}
	}

	return nil
}

// ensureFunctions ensures that a FunctionRevision and its parent Function exist
// for every pipeline step that references a function by package.
func (r *Reconciler) ensureFunctions(ctx context.Context, op *v1alpha1.Operation) error {
	owner := meta.AsOwner(meta.TypedReferenceTo(op, v1alpha1.OperationGroupVersionKind))

	// Multiple steps (or multiple operations) may reference different versions
	// of the same function package, in which case the parent Function ends up
	// with more than one external revision. The package manager discovers a
	// Function's external revisions by their parent package label, so we only
	// need to ensure each Function once.
	fns := map[string]bool{}

	for _, step := range op.Spec.Pipeline {
		if step.Function == "" {
			continue
		}

		ref, err := xcomposite.NormalizeFunctionOCIRef(step.Function)
		if err != nil {
			return errors.Wrapf(err, "invalid package reference in pipeline step %q", step.Step)
		}

		fnName := xcomposite.FunctionName(ref)
		revName := xcomposite.FunctionRevisionName(ref)

		if err := r.ensureFunctionRevision(ctx, revName, fnName, ref.Name(), owner); err != nil {
			return errors.Wrapf(err, "cannot ensure FunctionRevision for pipeline step %q", step.Step)
		}

		fns[fnName] = true
	}

	for _, fnName := range slices.Sorted(maps.Keys(fns)) {
		if err := r.ensureFunction(ctx, fnName, owner); err != nil {
			return errors.Wrapf(err, "cannot ensure Function %q", fnName)
		}
	}

	return nil
}

func (r *Reconciler) functionsReady(ctx context.Context, op *v1alpha1.Operation, revs []string) error {
	for i, step := range op.Spec.Pipeline {
		if step.Function == "" {
			continue
		}

		fr := &pkgv1.FunctionRevision{}
		if err := r.client.Get(ctx, client.ObjectKey{Name: revs[i]}, fr); err != nil {
			return errors.Wrapf(err, "cannot get FunctionRevision %q for pipeline step %q", revs[i], step.Step)
		}

		if pkgv1.PackageHealth(fr).Status != corev1.ConditionTrue || fr.Status.Endpoint == "" {
			return errors.Errorf("waiting for FunctionRevision %q for pipeline step %q to become healthy", revs[i], step.Step)
		}
	}

	return nil
}

// ensureFunctionRevision creates the named FunctionRevision if it does not
// exist and validates that it references the correct package if it does exist.
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

	// The revision name is derived from the normalized package, so a revision
	// with our name but a different package wasn't created for this package.
	// Refuse to use it.
	if existing, err := xcomposite.NormalizeFunctionOCIRef(rev.Spec.Package); err != nil || existing.Name() != pkg {
		return errors.Errorf("FunctionRevision %q already exists for package %q, not %q", revName, rev.Spec.Package, pkg)
	}

	// Don't add an owner reference to an otherwise unowned revision, to avoid
	// GCing someone else's revisions. However, do add an owner if there is any
	// existing owner, to avoid the revision getting GCed from under us.
	if len(rev.OwnerReferences) > 0 {
		meta.AddOwnerReference(rev, owner)
	}

	// The package manager doesn't manage this revision, so nothing else will
	// activate it if it's somehow deactivated.
	rev.Spec.DesiredState = pkgv1.PackageRevisionActive

	// TODO(adamwg): SSA?
	return r.client.Update(ctx, rev)
}

// ensureFunction creates the named parent Function if it does not exist. The
// package manager records the Function's external revisions in its status.
func (r *Reconciler) ensureFunction(ctx context.Context, fnName string, owner metav1.OwnerReference) error {
	fn := &pkgv1.Function{}
	err := r.client.Get(ctx, client.ObjectKey{Name: fnName}, fn)
	if kerrors.IsNotFound(err) {
		fn = &pkgv1.Function{
			ObjectMeta: metav1.ObjectMeta{
				Name:            fnName,
				OwnerReferences: []metav1.OwnerReference{owner},
			},
		}

		return r.client.Create(ctx, fn)
	}

	if err != nil {
		return err
	}

	// Don't add an owner reference to an otherwise unowned function, to avoid
	// GCing someone else's functions. However, do add an owner if there is any
	// existing owner, to avoid the function getting GCed from under us.
	if len(fn.OwnerReferences) == 0 {
		return nil
	}

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
