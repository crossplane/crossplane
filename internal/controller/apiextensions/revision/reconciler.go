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

// Package revision implements the CompositionRevision controller.
package revision

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonCheckCapabilities event.Reason = "CheckCapabilities"
	reasonInstallFunctions  event.Reason = "InstallFunctions"
	reasonInvalidPipeline   event.Reason = "InvalidPipeline"
)

const (
	errFmtMissingFunctionRef = "pipeline step %q has no functionRef; referencing functions by OCI reference requires the --enable-pipeline-oci-references alpha feature flag"
	errFmtInvalidOCIRef      = "function for pipeline step %q is not a valid OCI reference"
)

// Setup adds a controller that reconciles CompositionRevisions by validating
// that all functions in the pipeline have the required composition capability.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "revision/" + strings.ToLower(v1.CompositionRevisionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)),
		WithCapabilityChecker(xfn.NewRevisionCapabilityChecker(mgr.GetClient())),
		WithFeatures(o.Features))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.CompositionRevision{}).
		Watches(&pkgv1.FunctionRevision{}, EnqueueCompositionRevisionsForFunctionRevision(mgr.GetClient(), o.Logger)).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
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

// WithCapabilityChecker specifies the CapabilityChecker the Reconciler should use.
func WithCapabilityChecker(cc xfn.CapabilityChecker) ReconcilerOption {
	return func(r *Reconciler) {
		r.functions = cc
	}
}

// WithFeatures specifies which feature flags are enabled.
func WithFeatures(f *feature.Flags) ReconcilerOption {
	return func(r *Reconciler) {
		r.features = f
	}
}

// NewReconciler returns a Reconciler of CompositionRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		functions:  xfn.NewRevisionCapabilityChecker(mgr.GetClient()),
		features:   &feature.Flags{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// A Reconciler reconciles CompositionRevisions by validating that all functions
// in the pipeline have the required composition capability.
type Reconciler struct {
	client client.Client

	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager
	functions  xfn.CapabilityChecker
	features   *feature.Flags
}

// Reconcile a CompositionRevision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rev := &v1.CompositionRevision{}
	if err := r.client.Get(ctx, req.NamespacedName, rev); err != nil {
		log.Debug("Cannot get CompositionRevision", "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), "cannot get CompositionRevision")
	}

	statusBefore := rev.Status.DeepCopy()
	status := r.conditions.For(rev)

	if meta.WasDeleted(rev) {
		return reconcile.Result{}, nil
	}

	log = log.WithValues(
		"uid", rev.GetUID(),
		"version", rev.GetResourceVersion(),
		"name", rev.GetName(),
		"revision", rev.Spec.Revision,
	)

	// Retrying won't help if the pipeline is invalid - the CompositionRevision
	// is immutable, so we just report the problem and wait.
	if err := r.validatePipeline(rev); err != nil {
		log.Debug("Invalid pipeline", "error", err)
		r.record.Event(rev, event.Warning(reasonInvalidPipeline, err))
		status.MarkConditions(xpv2.ReconcileSuccess(), v1.InvalidPipeline(err.Error()))

		if !cmp.Equal(statusBefore, &rev.Status) {
			return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rev), "cannot update CompositionRevision status")
		}
		return reconcile.Result{}, nil
	}

	// Ensure FunctionRevisions (and their parent Functions) exist for any
	// pipeline steps that reference a function by package. Steps that
	// reference a function by name are assumed to have been installed by the
	// user already.
	if err := r.ensureFunctions(ctx, rev); err != nil {
		// A conflict means another Operation or CompositionRevision updated a
		// shared Function or FunctionRevision. An AlreadyExists means we did a
		// stale cache read and something else created a function or revision
		// underneath us. Either way, it's not our failure and a retry should
		// work; don't emit any events.
		if kerrors.IsConflict(err) || kerrors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		}

		log.Debug("Cannot ensure functions for pipeline", "error", err)
		r.record.Event(rev, event.Warning(reasonInstallFunctions, err))
		status.MarkConditions(xpv2.ReconcileError(err))
		_ = r.client.Status().Update(ctx, rev)

		return reconcile.Result{}, err
	}

	// Resolve each pipeline step to the FunctionRevision that will run it, like
	// the composite controller does. A step that references a function by OCI
	// reference runs the revision we created for it above. A step that
	// references an installed Function by name runs that Function's active
	// revision.
	revs := make([]string, 0, len(rev.Spec.Pipeline))
	for _, fn := range rev.Spec.Pipeline {
		fr, err := composite.FunctionRevisionForStep(ctx, r.client, fn)
		if err != nil {
			err = errors.Wrapf(err, "cannot resolve FunctionRevision for pipeline step %q", fn.Step)
			log.Debug("Cannot resolve FunctionRevision for pipeline step", "error", err)
			r.record.Event(rev, event.Warning(reasonCheckCapabilities, err))
			status.MarkConditions(xpv2.ReconcileError(err))
			_ = r.client.Status().Update(ctx, rev)

			return reconcile.Result{}, err
		}

		revs = append(revs, fr)
	}

	// Check that all functions have the composition capability
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityComposition}, revs...); err != nil {
		log.Debug("Function capability check failed", "error", err)
		r.record.Event(rev, event.Warning(reasonCheckCapabilities, err))
		status.MarkConditions(xpv2.ReconcileSuccess(), v1.MissingCapabilities(err.Error()))

		// Update status but don't return the error - capability failures are informational
		if !cmp.Equal(statusBefore, &rev.Status) {
			return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rev), "cannot update CompositionRevision status")
		}
		return reconcile.Result{}, nil
	}

	log.Debug("All functions have required composition capability")
	r.record.Event(rev, event.Normal(reasonCheckCapabilities, "All functions have required composition capability"))
	status.MarkConditions(xpv2.ReconcileSuccess(), v1.ValidPipeline())

	if !cmp.Equal(statusBefore, &rev.Status) {
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, rev), "cannot update CompositionRevision status")
	}
	return reconcile.Result{}, nil
}

// validatePipeline validates the steps in the revision's pipeline and returns
// an error if any step is invalid.
func (r *Reconciler) validatePipeline(rev *v1.CompositionRevision) error {
	if r.features.Enabled(features.EnableAlphaPipelineOCIReferences) {
		for _, s := range rev.Spec.Pipeline {
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
		for _, s := range rev.Spec.Pipeline {
			if s.FunctionRef == nil {
				return errors.Errorf(errFmtMissingFunctionRef, s.Step)
			}
		}
	}

	return nil
}

// ensureFunctions ensures that a FunctionRevision and its parent Function exist
// for every pipeline step that references a function by package.
func (r *Reconciler) ensureFunctions(ctx context.Context, rev *v1.CompositionRevision) error {
	owner := meta.AsOwner(meta.TypedReferenceTo(rev, v1.CompositionRevisionGroupVersionKind))

	// Multiple steps (or multiple composition revisions) may reference
	// different versions of the same function package, in which case the
	// parent Function ends up with more than one external revision. The
	// package manager discovers a Function's external revisions by their
	// parent package label, so we only need to ensure each Function once.
	fns := map[string]bool{}

	for _, step := range rev.Spec.Pipeline {
		if step.Function == "" {
			continue
		}

		ref, err := composite.NormalizeFunctionOCIRef(step.Function)
		if err != nil {
			return errors.Wrapf(err, "invalid package reference in pipeline step %q", step.Step)
		}

		fnName := composite.FunctionName(ref)
		revName := composite.FunctionRevisionName(ref)

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

	// Ensure the revision points to the right package.
	if existing, err := composite.NormalizeFunctionOCIRef(rev.Spec.Package); err != nil || existing.Name() != pkg {
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
