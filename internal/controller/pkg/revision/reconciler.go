/*
Copyright 2020 The Crossplane Authors.

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

// Package revision implements the Crossplane Package Revision controllers.
package revision

import (
	"context"
	"sort"
	"strings"
	"time"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	k8sextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	extv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	extv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	extv2 "github.com/crossplane/crossplane/apis/v2/apiextensions/v2"
	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/v2/internal/converter"
	"github.com/crossplane/crossplane/v2/internal/dag"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/version"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

const (
	reconcileTimeout   = 3 * time.Minute
	finalizer          = "revision.pkg.crossplane.io"
	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

const (
	errGetPackageRevision = "cannot get package revision"
	errUpdateStatus       = "cannot update package revision status"
	errAddFinalizer       = "cannot add package revision finalizer"
	errRemoveFinalizer    = "cannot remove package revision finalizer"
	errDeactivateRevision = "cannot deactivate package revision"
	errGetPackage         = "cannot get package"
	errValidatePackage    = "validating package contents failed"
	errLintPackage        = "linting package contents failed"
	errNotOneMeta         = "cannot install package with multiple meta types"
	errIncompatible       = "incompatible Crossplane version"
	errEstablishControl   = "cannot establish control of object"
	errReleaseObjects     = "cannot release objects"
	errUpdateMeta         = "cannot update package revision object metadata"
	errRemoveLock         = "cannot remove package revision from Lock"
	errResolveDeps        = "cannot resolve package dependencies"
	errConfResourceObject = "cannot convert to resource.Object"
)

// Event reasons.
const (
	reasonParse        event.Reason = "ParsePackage"
	reasonLint         event.Reason = "LintPackage"
	reasonValidate     event.Reason = "ValidatePackage"
	reasonDependencies event.Reason = "ResolveDependencies"
	reasonConvertCRD   event.Reason = "ConvertCRDToMRD"
	reasonSync         event.Reason = "SyncPackage"
	reasonDeactivate   event.Reason = "DeactivateRevision"
	reasonPaused       event.Reason = "ReconciliationPaused"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.kube = ca.Client
	}
}

// WithClient specifies the package client to use for fetching and parsing packages.
func WithClient(c xpkg.Client) ReconcilerOption {
	return func(r *Reconciler) {
		r.pkg = c
	}
}

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevision = f
	}
}

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

// WithFinalizer specifies how the Reconciler should finalize package revisions.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.revision = f
	}
}

// WithDependencyManager specifies how the Reconciler should manage dependencies.
func WithDependencyManager(m DependencyManager) ReconcilerOption {
	return func(r *Reconciler) {
		r.lock = m
	}
}

// WithEstablisher specifies how the Reconciler should establish package resources.
func WithEstablisher(e Establisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.objects = e
	}
}

// WithLinter specifies how the Reconciler should lint a package.
func WithLinter(l parser.Linter) ReconcilerOption {
	return func(r *Reconciler) {
		r.linter = l
	}
}

// WithValidator specifies how the Reconciler should validate a package.
func WithValidator(v xpkg.Validator) ReconcilerOption {
	return func(r *Reconciler) {
		r.validator = v
	}
}

// WithVersioner specifies how the Reconciler should fetch the current
// Crossplane version.
func WithVersioner(v version.Operations) ReconcilerOption {
	return func(r *Reconciler) {
		r.versioner = v
	}
}

// WithNamespace specifies the namespace in which the Reconciler should create
// runtime resources.
func WithNamespace(n string) ReconcilerOption {
	return func(r *Reconciler) {
		r.namespace = n
	}
}

// WithServiceAccount specifies the core Crossplane ServiceAccount name.
func WithServiceAccount(sa string) ReconcilerOption {
	return func(r *Reconciler) {
		r.serviceAccount = sa
	}
}

// WithFeatureFlags specifies the feature flags to inject into the Reconciler.
func WithFeatureFlags(f *feature.Flags) ReconcilerOption {
	return func(r *Reconciler) {
		r.features = f
	}
}

// Reconciler reconciles packages.
type Reconciler struct {
	kube           client.Client
	pkg            xpkg.Client
	revision       resource.Finalizer
	lock           DependencyManager
	objects        Establisher
	linter         parser.Linter
	validator      xpkg.Validator
	versioner      version.Operations
	log            logging.Logger
	record         event.Recorder
	conditions     conditions.Manager
	features       *feature.Flags
	namespace      string
	serviceAccount string

	newPackageRevision func() v1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.ProviderRevision{} }

	log := o.Logger.WithValues("controller", name)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&corev1.Secret{}). // Watch secret changes to react if pull or cert secrets change.
		Watches(&v1beta1.Lock{}, EnqueuePackageRevisionsForLock(mgr.GetClient(), &v1.ProviderRevisionList{}, log)).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackageRevisionsForImageConfig(mgr.GetClient(), &v1.ProviderRevisionList{}, log))

	est := NewFilteringEstablisher(
		NewAPIEstablisher(mgr.GetClient(), o.Namespace, o.MaxConcurrentPackageEstablishers),
		extv1alpha1.ManagedResourceDefinitionGroupVersionKind.GroupKind(),
		schema.GroupKind{Group: k8sextv1.SchemeGroupVersion.Group, Kind: "CustomResourceDefinition"},
		schema.GroupKind{Group: admv1.SchemeGroupVersion.Group, Kind: "ValidatingWebhookConfiguration"},
		schema.GroupKind{Group: admv1.SchemeGroupVersion.Group, Kind: "MutatingWebhookConfiguration"},
	)

	r := NewReconciler(mgr,
		WithClient(o.Client),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1.ProviderGroupVersionKind, log)),
		WithEstablisher(est),
		WithNewPackageRevisionFn(nr),
		WithLinter(xpkg.NewProviderLinter()),
		WithValidator(xpkg.NewProviderValidator()),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	)

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ConfigurationRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.ConfigurationRevision{} }

	log := o.Logger.WithValues("controller", name)

	est := NewFilteringEstablisher(
		NewAPIEstablisher(mgr.GetClient(), o.Namespace, o.MaxConcurrentPackageEstablishers),
		extv2.CompositeResourceDefinitionGroupVersionKind.GroupKind(),
		extv1.CompositionGroupVersionKind.GroupKind(),
		extv1alpha1.ManagedResourceActivationPolicyGroupVersionKind.GroupKind(),
		opsv1alpha1.OperationGroupVersionKind.GroupKind(),
		opsv1alpha1.CronOperationGroupVersionKind.GroupKind(),
		opsv1alpha1.WatchOperationGroupVersionKind.GroupKind(),
	)

	r := NewReconciler(mgr,
		WithClient(o.Client),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1.ConfigurationGroupVersionKind, log)),
		WithNewPackageRevisionFn(nr),
		WithEstablisher(est),
		WithLinter(xpkg.NewConfigurationLinter()),
		WithValidator(xpkg.NewConfigurationValidator()),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ConfigurationRevision{}).
		Watches(&v1beta1.Lock{}, EnqueuePackageRevisionsForLock(mgr.GetClient(), &v1.ConfigurationRevisionList{}, log)).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackageRevisionsForImageConfig(mgr.GetClient(), &v1.ConfigurationRevisionList{}, log)).
		WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// SetupFunctionRevision adds a controller that reconciles FunctionRevisions.
func SetupFunctionRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.FunctionRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.FunctionRevision{} }

	log := o.Logger.WithValues("controller", name)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.FunctionRevision{}).
		Owns(&corev1.Secret{}). // Watch secret changes to react if pull or cert secrets change.
		Watches(&v1beta1.Lock{}, EnqueuePackageRevisionsForLock(mgr.GetClient(), &v1.FunctionRevisionList{}, log)).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackageRevisionsForImageConfig(mgr.GetClient(), &v1.FunctionRevisionList{}, log))

	// The xpkg spec allows for CRDs to be included in function packages, but
	// states that they will not be installed. This means we shouldn't install
	// any objects from function packages. Create an empty filtering establisher
	// to filter them all out.
	est := NewFilteringEstablisher(
		NewAPIEstablisher(mgr.GetClient(), o.Namespace, o.MaxConcurrentPackageEstablishers),
	)

	r := NewReconciler(mgr,
		WithClient(o.Client),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1.FunctionGroupVersionKind, log)),
		WithEstablisher(est),
		WithNewPackageRevisionFn(nr),
		WithLinter(xpkg.NewFunctionLinter()),
		WithValidator(xpkg.NewFunctionValidator()),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name), o.EventFilterFunctions...)), //nolint:staticcheck // TODO(adamwg) Update crossplane-runtime to the new events API.
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	)

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(errors.WithSilentRequeueOnConflict(r))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		kube:       mgr.GetClient(),
		revision:   resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		objects:    NewNopEstablisher(),
		linter:     parser.NewPackageLinter(nil, nil, nil),
		validator:  parser.NewPackageLinter(nil, nil, nil),
		versioner:  version.New(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are often very complex.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	pr := r.newPackageRevision()
	if err := r.kube.Get(ctx, req.NamespacedName, pr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetPackageRevision, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackageRevision)
	}

	status := r.conditions.For(pr)

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(pr) {
		r.record.Event(pr, event.Normal(reasonPaused, reconcilePausedMsg))
		status.MarkConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, pr), errUpdateStatus)
	}

	if meta.WasDeleted(pr) {
		// NOTE(hasheddan): if we were previously marked as inactive, we
		// likely already removed self. If we skipped dependency
		// resolution, we will not be present in the lock.
		if err := r.lock.RemoveSelf(ctx, pr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errRemoveLock)
			r.record.Event(pr, event.Warning(reasonSync, err))

			return reconcile.Result{}, err
		}
		// Note(turkenh): During the deletion of an active package revision,
		// we don't need to run relinquish step since when the parent objects
		// (i.e. Package Revision) is gone, the controller reference on the
		// child objects (i.e. CRD) will be garbage collected.
		// We don't need to run the deactivate runtimeHook either since the owned
		// Deployment or similar objects will be garbage collected as well.
		if err := r.revision.RemoveFinalizer(ctx, pr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errRemoveFinalizer)
			r.record.Event(pr, event.Warning(reasonSync, err))

			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: false}, nil
	}

	if c := pr.GetCondition(xpv1.ReconcilePaused().Type); c.Reason == xpv1.ReconcilePaused().Reason {
		pr.CleanConditions()
		// Persist the removal of conditions and return. We'll be requeued
		// with the updated status and resume reconciliation.
		return reconcile.Result{}, errors.Wrap(r.kube.Status().Update(ctx, pr), errUpdateStatus)
	}

	if err := r.revision.AddFinalizer(ctx, pr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	// Deactivate revision if it is inactive.
	if pr.GetDesiredState() == v1.PackageRevisionInactive {
		if err := r.deactivateRevision(ctx, pr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errDeactivateRevision)
			r.record.Event(pr, event.Warning(reasonDeactivate, err))

			return reconcile.Result{}, err
		}

		if len(pr.GetObjects()) > 0 {
			// Note(turkenh): If the revision is inactive we don't need to
			// fetch/parse the package again, so we can report success and return
			// here. The only exception is that revision NOT having references
			// to the objects that it owns, i.e. status.objectRefs is empty.
			// This could happen in one of the following two ways:
			// 1. The revision created as inactive, i.e. was never active before
			//    which could be possible if package installed with
			//    `revisionActivationPolicy` as `Manual`.
			// 2. The status of the revision got lost, e.g. the status subresource
			//    is not properly restored after a backup/restore operation.
			// So, we report success and return here iff the revision is inactive
			// and it has references to the objects that it owns.
			// Otherwise, we continue with fetching/parsing the package which
			// would trigger another reconcile after setting object references
			// in the status where we finalize the deactivation by transitioning
			// from "controller" to "owner" on owned resources.
			// We still want to call r.deactivateRevision() above, i.e. even
			// status.objectRefs is empty, to make sure that the revision is
			// removed from the lock which could otherwise block a successful
			// reconciliation.
			if pr.GetCondition(v1.TypeRevisionHealthy).Status != corev1.ConditionTrue {
				// NOTE(phisco): We don't want to spam the user with events if the
				// package revision is already healthy.
				r.record.Event(pr, event.Normal(reasonSync, "Successfully reconciled package revision"))
			}

			status.MarkConditions(v1.RevisionHealthy())

			return reconcile.Result{Requeue: false}, errors.Wrap(r.kube.Status().Update(ctx, pr), errUpdateStatus)
		}
	}

	// NOTE(negz): There are a bunch of cases below where we ignore errors
	// returned while updating our status to reflect that our revision is
	// unhealthy (or of unknown health). This is because:
	//
	// 1. We prefer to return the 'original' underlying error.
	// 2. We'll requeue and try the status update again if needed.
	// 3. There's little else we could do about it apart from log.

	// Fetch and parse the package.
	pkg, err := r.pkg.Get(ctx, pr.GetSource(),
		xpkg.WithPullSecrets(v1.RefNames(pr.GetPackagePullSecrets())...),
		xpkg.WithPullPolicy(ptr.Deref(pr.GetPackagePullPolicy(), corev1.PullIfNotPresent)),
	)
	if err != nil {
		err = errors.Wrap(err, errGetPackage)
		status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

		_ = r.kube.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonParse, err))

		return reconcile.Result{}, err
	}

	// Update status with resolved source and applied image configs.
	// NOTE: We set these early so error paths below can report them. We'll set
	// them again after r.kube.Update() because Update() overwrites pr with the
	// server's response, which would wipe these in-memory status changes.
	pr.SetResolvedSource(pkg.ResolvedRef())

	for _, reason := range xpkg.SupportedImageConfigs() {
		pr.ClearAppliedImageConfigRef(v1.ImageConfigRefReason(reason))
	}
	for _, cfg := range pkg.AppliedImageConfigs {
		pr.SetAppliedImageConfigRefs(v1.ImageConfigRef{
			Name:   cfg.Name,
			Reason: v1.ImageConfigRefReason(cfg.Reason),
		})
	}

	// Validate the package using a package-specific validator. If validation
	// fails, we won't try to install the package.
	if err := r.validator.Lint(pkg.Package); err != nil {
		err = errors.Wrap(err, errValidatePackage)
		status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

		_ = r.kube.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonValidate, err))

		return reconcile.Result{}, err
	}

	// Lint package using package-specific linter. We can proceed with
	// installation even there are lint errors; we just record them in an event
	// for the user's information since they may cause unexpected behavior.
	if err := r.linter.Lint(pkg.Package); err != nil {
		err = errors.Wrap(err, errLintPackage)
		r.record.Event(pr, event.Warning(reasonLint, err))
		// TODO(adamwg): Should we also record lint errors in the status for
		// posterity? Events are ephemeral.
	}

	// NOTE(hasheddan): the linter should check this property already, but
	// if a consumer forgets to pass an option to guarantee one meta object,
	// we check here to avoid a potential panic on 0 index below.
	if len(pkg.Package.GetMeta()) != 1 {
		err = errors.New(errNotOneMeta)
		status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

		_ = r.kube.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonLint, err))

		return reconcile.Result{}, err
	}

	pkgMeta, _ := xpkg.TryConvertToPkg(pkg.Package.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{}, &pkgmetav1.Function{})

	meta.AddLabels(pr, pkgMeta.GetLabels())
	meta.AddAnnotations(pr, pkgMeta.GetAnnotations())

	if err := r.kube.Update(ctx, pr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errUpdateMeta)
		status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

		_ = r.kube.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	// Re-apply status changes that were wiped by r.kube.Update() above. Update()
	// overwrites pr with the server's response, which doesn't include status.
	pr.SetResolvedSource(pkg.ResolvedRef())

	for _, reason := range xpkg.SupportedImageConfigs() {
		pr.ClearAppliedImageConfigRef(v1.ImageConfigRefReason(reason))
	}
	for _, cfg := range pkg.AppliedImageConfigs {
		pr.SetAppliedImageConfigRefs(v1.ImageConfigRef{
			Name:   cfg.Name,
			Reason: v1.ImageConfigRefReason(cfg.Reason),
		})
	}

	// Copy the capabilities from the metadata to the revision.
	pr.SetCapabilities(pkgMeta.GetCapabilities())

	// Check Crossplane constraints if they exist.
	if pr.GetIgnoreCrossplaneConstraints() == nil || !*pr.GetIgnoreCrossplaneConstraints() {
		if err := xpkg.PackageCrossplaneCompatible(r.versioner)(pkgMeta); err != nil {
			err = errors.Wrap(err, errIncompatible)
			status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

			r.record.Event(pr, event.Warning(reasonLint, err))

			// No need to requeue if outside version constraints.
			// Package will either need to be updated or ignore
			// crossplane constraints will need to be specified,
			// both of which will trigger a new reconcile.
			return reconcile.Result{Requeue: false}, errors.Wrap(r.kube.Status().Update(ctx, pr), errUpdateStatus)
		}
	}

	// Check status of package dependencies unless package specifies to skip
	// resolution.
	if pr.GetSkipDependencyResolution() != nil && !*pr.GetSkipDependencyResolution() {
		found, installed, invalid, err := r.lock.Resolve(ctx, pkgMeta, pr)
		pr.SetDependencyStatus(int64(found), int64(installed), int64(invalid))

		if err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errResolveDeps)
			status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

			_ = r.kube.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonDependencies, err))

			return reconcile.Result{}, err
		}
	}

	objects := pkg.GetObjects()
	// The CustomToManagedResourceConversion feature selectively converts CRDs
	// to MRDs from a provider package. Only managed resource CRDs are converted;
	// provider configuration CRDs (ProviderConfig, etc.) remain as regular CRDs.
	if _, ok := pkgMeta.(*pkgmetav1.Provider); ok && r.features.Enabled(features.EnableBetaCustomToManagedResourceConversion) {
		// Convert managed resource CRDs to MRDs, leaving other CRDs unchanged
		// If SafeStart is not in capabilities, we default mrd state to Active.
		activationState := !pkgmetav1.CapabilitiesContainFuzzyMatch(pr.GetCapabilities(), pkgmetav1.ProviderCapabilitySafeStart)
		if converted, err := converter.CustomToManagedResourceDefinitions(activationState, objects...); err != nil {
			log.Debug("failed to convert CRDs to MRDs for provider, skipping conversion", "error", err)
			r.record.Event(pr, event.Warning(reasonConvertCRD, err))
		} else {
			objects = converted
		}
	}

	// Establish control or ownership of objects.
	refs, err := r.objects.Establish(ctx, objects, pr, pr.GetDesiredState() == v1.PackageRevisionActive)
	if err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errEstablishControl)
		status.MarkConditions(v1.RevisionUnhealthy().WithMessage(err.Error()))

		_ = r.kube.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	// Update object list in package revision status with objects for which
	// ownership or control has been established.
	// NOTE(hasheddan): we avoid the overhead of performing a stable sort here
	// as we are not concerned with preserving the existing ordering of the
	// slice, but rather the existing references in the status of the package
	// revision. We should also not have equivalent references in the slice, but
	// a poorly formed, but still valid package could contain duplicates.
	// However, in that case the references would be identical (including UUID),
	// so unstable sort order would not cause a diff in the package revision
	// status.
	// See https://github.com/crossplane/crossplane/issues/3466 for tracking
	// restricting duplicate resources in packages.
	sort.Slice(refs, func(i, j int) bool {
		return uniqueResourceIdentifier(refs[i]) > uniqueResourceIdentifier(refs[j])
	})
	pr.SetObjects(refs)

	if pr.GetCondition(v1.TypeRevisionHealthy).Status != corev1.ConditionTrue {
		// NOTE(phisco): We don't want to spam the user with events if the
		// package revision is already healthy.
		r.record.Event(pr, event.Normal(reasonSync, "Successfully reconciled package revision"))
	}

	status.MarkConditions(v1.RevisionHealthy())

	return reconcile.Result{Requeue: false}, errors.Wrap(r.kube.Status().Update(ctx, pr), errUpdateStatus)
}

func (r *Reconciler) deactivateRevision(ctx context.Context, pr v1.PackageRevision) error {
	// Remove self from the lock if we are present.
	if err := r.lock.RemoveSelf(ctx, pr); err != nil {
		return errors.Wrap(err, errRemoveLock)
	}

	// ReleaseObjects control of objects.
	if err := r.objects.ReleaseObjects(ctx, pr); err != nil {
		return errors.Wrap(err, errReleaseObjects)
	}

	return nil
}

// uniqueResourceIdentifier returns a unique identifier for a resource in a
// package, consisting of the group, version, kind, and name.
func uniqueResourceIdentifier(ref xpv1.TypedReference) string {
	return strings.Join([]string{ref.GroupVersionKind().String(), ref.Name}, "/")
}
