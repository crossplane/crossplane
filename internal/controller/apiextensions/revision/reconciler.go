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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/v2/internal/xfn"
)

const (
	timeout = 2 * time.Minute
)

// Event reasons.
const (
	reasonCheckCapabilities event.Reason = "CheckCapabilities"
	reasonInstallFunctions  event.Reason = "InstallFunctions"
)

// Setup adds a controller that reconciles CompositionRevisions by validating
// that all functions in the pipeline have the required composition capability.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "revision/" + strings.ToLower(v1.CompositionRevisionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)),
		WithCapabilityChecker(xfn.NewRevisionCapabilityChecker(mgr.GetClient())))

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

// NewReconciler returns a Reconciler of CompositionRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
		functions:  xfn.NewRevisionCapabilityChecker(mgr.GetClient()),
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

	// Ensure FunctionRevisions (and their parent Functions) exist for any
	// pipeline steps that reference a function by package. Steps that
	// reference a function by name are assumed to have been installed by the
	// user already.
	if err := r.ensureFunctions(ctx, rev); err != nil {
		log.Debug("Cannot ensure functions for pipeline", "error", err)
		r.record.Event(rev, event.Warning(reasonInstallFunctions, err))
		status.MarkConditions(xpv2.ReconcileError(err))
		_ = r.client.Status().Update(ctx, rev)

		return reconcile.Result{}, err
	}

	// Resolve each pipeline step to a function package OCI reference. A step may
	// reference a function either by the name of an installed Function, or
	// directly by package OCI reference. The capability checker looks functions
	// up by package, so resolve name-based references to their package here,
	// like the composite controller does.
	refs := make([]string, 0, len(rev.Spec.Pipeline))
	for _, fn := range rev.Spec.Pipeline {
		pkg := fn.Function
		if pkg == "" {
			f := &pkgv1.Function{}
			if err := r.client.Get(ctx, client.ObjectKey{Name: fn.FunctionRef.Name}, f); err != nil {
				log.Debug("Cannot get Function for pipeline step", "error", err)
				r.record.Event(rev, event.Warning(reasonCheckCapabilities, err))
				status.MarkConditions(xpv2.ReconcileError(err))
				_ = r.client.Status().Update(ctx, rev)

				return reconcile.Result{}, err
			}
			pkg = f.Spec.Package
		}

		refs = append(refs, pkg)
	}

	// Check that all functions have the composition capability
	if err := r.functions.CheckCapabilities(ctx, []string{pkgmetav1.FunctionCapabilityComposition}, refs...); err != nil {
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

// ensureFunctions ensures that a FunctionRevision and its parent Function exist
// for every pipeline step that references a function by package. Each object
// gains an owner reference to the supplied CompositionRevision, so that the
// objects are garbage collected once no CompositionRevision references them any
// longer.
func (r *Reconciler) ensureFunctions(ctx context.Context, rev *v1.CompositionRevision) error {
	owner := meta.AsOwner(meta.TypedReferenceTo(rev, v1.CompositionRevisionGroupVersionKind))

	// Collect the set of external revisions per parent Function. Multiple steps
	// (or multiple composition revisions) may reference different versions of
	// the same function package, in which case the parent Function ends up with
	// more than one external revision.
	revisionsByFunction := map[string]map[string]bool{}

	for _, step := range rev.Spec.Pipeline {
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
// multiple composition revisions referencing the same package will share a
// function revision. Note, however, that we construct names differently from
// the package manager, so we will not share revisions with it.
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
