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
	"io"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/dag"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	reconcileTimeout = 3 * time.Minute
	// the max size of a package parsed by the parser
	maxPackageSize = 200 << 20 // 100 MB
)

const (
	finalizer = "revision.pkg.crossplane.io"

	errGetPackageRevision = "cannot get package revision"
	errUpdateStatus       = "cannot update package revision status"

	errDeleteCache = "cannot remove package contents from cache"
	errGetCache    = "cannot get package contents from cache"

	errPullPolicyNever = "failed to get pre-cached package with pull policy Never"

	errAddFinalizer    = "cannot add package revision finalizer"
	errRemoveFinalizer = "cannot remove package revision finalizer"

	errDeactivateRevision = "cannot deactivate package revision"

	errInitParserBackend = "cannot initialize parser backend"
	errParsePackage      = "cannot parse package contents"
	errLintPackage       = "linting package contents failed"
	errNotOneMeta        = "cannot install package with multiple meta types"
	errIncompatible      = "incompatible Crossplane version"

	errManifestBuilderOptions = "cannot prepare runtime manifest builder options"
	errPreHook                = "pre establish runtime hook failed for package"
	errPostHook               = "post establish runtime hook failed for package"
	errDeactivationHook       = "deactivation runtime hook failed for package"

	errEstablishControl = "cannot establish control of object"
	errReleaseObjects   = "cannot release objects"

	errUpdateMeta = "cannot update package revision object metadata"

	errRemoveLock  = "cannot remove package revision from Lock"
	errResolveDeps = "cannot resolve package dependencies"

	errConfResourceObject = "cannot convert to resource.Object"

	errCannotInitializeHostClientSet = "failed to initialize host clientset with in cluster config"
	errCannotBuildMetaSchema         = "cannot build meta scheme for package parser"
	errCannotBuildObjectSchema       = "cannot build object scheme for package parser"
	errCannotBuildFetcher            = "cannot build fetcher for package parser"

	errGetControllerConfig = "cannot get referenced controller config"
	errNoRuntimeConfig     = "no deployment runtime config set"
	errGetRuntimeConfig    = "cannot get referenced deployment runtime config"
	errGetServiceAccount   = "cannot get Crossplane service account"

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

// Event reasons.
const (
	reasonParse        event.Reason = "ParsePackage"
	reasonLint         event.Reason = "LintPackage"
	reasonDependencies event.Reason = "ResolveDependencies"
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
		r.client = ca
	}
}

// WithCache specifies how the Reconcile should cache package contents.
func WithCache(c xpkg.PackageCache) ReconcilerOption {
	return func(r *Reconciler) {
		r.cache = c
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

// WithRuntimeHooks specifies how the Reconciler should perform preparations
// (pre- and post-establishment) and cleanup (deactivate) for package runtime.
// The hooks are only used when the package has a runtime and the runtime is
// configured as Deployment.
func WithRuntimeHooks(h RuntimeHooks) ReconcilerOption {
	return func(r *Reconciler) {
		r.runtimeHook = h
	}
}

// WithEstablisher specifies how the Reconciler should establish package resources.
func WithEstablisher(e Establisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.objects = e
	}
}

// WithParser specifies how the Reconciler should parse a package.
func WithParser(p parser.Parser) ReconcilerOption {
	return func(r *Reconciler) {
		r.parser = p
	}
}

// WithParserBackend specifies how the Reconciler should parse a package.
func WithParserBackend(p parser.Backend) ReconcilerOption {
	return func(r *Reconciler) {
		r.backend = p
	}
}

// WithLinter specifies how the Reconciler should lint a package.
func WithLinter(l parser.Linter) ReconcilerOption {
	return func(r *Reconciler) {
		r.linter = l
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

// uniqueResourceIdentifier returns a unique identifier for a resource in a
// package, consisting of the group, version, kind, and name.
func uniqueResourceIdentifier(ref xpv1.TypedReference) string {
	return strings.Join([]string{ref.GroupVersionKind().String(), ref.Name}, "/")
}

// Reconciler reconciles packages.
type Reconciler struct {
	client         client.Client
	cache          xpkg.PackageCache
	revision       resource.Finalizer
	lock           DependencyManager
	runtimeHook    RuntimeHooks
	objects        Establisher
	parser         parser.Parser
	linter         parser.Linter
	versioner      version.Operations
	backend        parser.Backend
	log            logging.Logger
	record         event.Recorder
	features       *feature.Flags
	namespace      string
	serviceAccount string

	newPackageRevision func() v1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.ProviderRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCannotInitializeHostClientSet)
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return errors.New(errCannotBuildMetaSchema)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return errors.New(errCannotBuildObjectSchema)
	}
	fetcher, err := xpkg.NewK8sFetcher(clientset, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errCannotBuildFetcher)
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&v1alpha1.ControllerConfig{}, &EnqueueRequestForReferencingProviderRevisions{
			client: mgr.GetClient(),
		})

	ro := []ReconcilerOption{
		WithCache(o.Cache),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1beta1.ProviderPackageType)),
		WithEstablisher(NewAPIEstablisher(mgr.GetClient(), o.Namespace)),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(NewImageBackend(fetcher, WithDefaultRegistry(o.DefaultRegistry))),
		WithLinter(xpkg.NewProviderLinter()),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	}

	if o.PackageRuntime == controller.PackageRuntimeDeployment {
		ro = append(ro, WithRuntimeHooks(NewProviderHooks(mgr.GetClient())))

		if o.Features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
			cb = cb.Watches(&v1beta1.DeploymentRuntimeConfig{}, &EnqueueRequestForReferencingProviderRevisions{
				client: mgr.GetClient(),
			})
		}
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, ro...)), o.GlobalRateLimiter))
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ConfigurationRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.ConfigurationRevision{} }

	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCannotInitializeHostClientSet)
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return errors.New(errCannotBuildMetaSchema)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return errors.New(errCannotBuildObjectSchema)
	}
	f, err := xpkg.NewK8sFetcher(cs, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errCannotBuildFetcher)
	}

	r := NewReconciler(mgr,
		WithCache(o.Cache),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1beta1.ConfigurationPackageType)),
		WithNewPackageRevisionFn(nr),
		WithEstablisher(NewAPIEstablisher(mgr.GetClient(), o.Namespace)),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(NewImageBackend(f, WithDefaultRegistry(o.DefaultRegistry))),
		WithLinter(xpkg.NewConfigurationLinter()),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ConfigurationRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// SetupFunctionRevision adds a controller that reconciles FunctionRevisions.
func SetupFunctionRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1beta1.FunctionRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1beta1.FunctionRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCannotInitializeHostClientSet)
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return errors.New(errCannotBuildMetaSchema)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return errors.New(errCannotBuildObjectSchema)
	}
	fetcher, err := xpkg.NewK8sFetcher(clientset, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errCannotBuildFetcher)
	}

	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.FunctionRevision{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&v1alpha1.ControllerConfig{}, &EnqueueRequestForReferencingFunctionRevisions{
			client: mgr.GetClient(),
		})

	ro := []ReconcilerOption{
		WithCache(o.Cache),
		WithDependencyManager(NewPackageDependencyManager(mgr.GetClient(), dag.NewMapDag, v1beta1.FunctionPackageType)),
		WithEstablisher(NewAPIEstablisher(mgr.GetClient(), o.Namespace)),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(NewImageBackend(fetcher, WithDefaultRegistry(o.DefaultRegistry))),
		WithLinter(xpkg.NewFunctionLinter()),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithFeatureFlags(o.Features),
	}

	if o.PackageRuntime == controller.PackageRuntimeDeployment {
		ro = append(ro, WithRuntimeHooks(NewFunctionHooks(mgr.GetClient())))

		if o.Features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
			cb = cb.Watches(&v1beta1.DeploymentRuntimeConfig{}, &EnqueueRequestForReferencingFunctionRevisions{
				client: mgr.GetClient(),
			})
		}
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, ro...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {

	r := &Reconciler{
		client:    mgr.GetClient(),
		cache:     xpkg.NewNopCache(),
		revision:  resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		objects:   NewNopEstablisher(),
		parser:    parser.New(nil, nil),
		linter:    parser.NewPackageLinter(nil, nil, nil),
		versioner: version.New(),
		log:       logging.NewNopLogger(),
		record:    event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcilers are often very complex.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	pr := r.newPackageRevision()
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetPackageRevision, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackageRevision)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(pr) {
		r.record.Event(pr, event.Normal(reasonPaused, reconcilePausedMsg))
		pr.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	if meta.WasDeleted(pr) {
		// NOTE(hasheddan): In the event that a pre-cached package was
		// used for this revision, delete will not remove the pre-cached
		// package image from the cache unless it has the same name as
		// the provider revision. Delete will not return an error so we
		// will remove finalizer and leave the image in the cache.
		if err := r.cache.Delete(pr.GetName()); err != nil {
			err = errors.Wrap(err, errDeleteCache)
			r.record.Event(pr, event.Warning(reasonSync, err))
			return reconcile.Result{}, err
		}
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
	}

	if err := r.revision.AddFinalizer(ctx, pr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errAddFinalizer)
		r.record.Event(pr, event.Warning(reasonSync, err))
		return reconcile.Result{}, err
	}

	var runtimeManifestBuilder ManifestBuilder
	if r.runtimeHook != nil {
		pwr := pr.(v1.PackageRevisionWithRuntime)

		opts, err := r.runtimeManifestBuilderOptions(ctx, pwr)
		if err != nil {
			log.Debug(errManifestBuilderOptions, "error", err)

			err = errors.Wrap(err, errManifestBuilderOptions)
			pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
			_ = r.client.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonSync, err))

			return reconcile.Result{}, err
		}

		runtimeManifestBuilder = NewRuntimeManifestBuilder(pwr, r.namespace, opts...)
	}

	// Deactivate revision if it is inactive.
	if pr.GetDesiredState() == v1.PackageRevisionInactive {
		if err := r.deactivateRevision(ctx, pr, runtimeManifestBuilder); err != nil {
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
			r.record.Event(pr, event.Normal(reasonSync, "Successfully configured package revision"))
			pr.SetConditions(v1.Healthy())
			return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
		}
	}

	// NOTE(negz): There are a bunch of cases below where we ignore errors
	// returned while updating our status to reflect that our revision is
	// unhealthy (or of unknown health). This is because:
	//
	// 1. We prefer to return the 'original' underlying error.
	// 2. We'll requeue and try the status update again if needed.
	// 3. There's little else we could do about it apart from log.

	pullPolicyNever := false
	id := pr.GetName()
	// If packagePullPolicy is Never, the identifier is the package source and
	// contents must be in the cache.
	if pr.GetPackagePullPolicy() != nil && *pr.GetPackagePullPolicy() == corev1.PullNever {
		pullPolicyNever = true
		id = pr.GetSource()
	}

	var rc io.ReadCloser
	cacheWrite := make(chan error)

	if r.cache.Has(id) {
		var err error
		rc, err = r.cache.Get(id)
		if err != nil {
			// If package contents are in the cache, but we cannot access them,
			// we clear them and try again.
			_ = r.cache.Delete(id)
			err = errors.Wrap(err, errGetCache)
			r.record.Event(pr, event.Warning(reasonParse, err))
			return reconcile.Result{}, err
		}
		// If we got content from cache we don't need to wait for it to be
		// written.
		close(cacheWrite)
	}

	// packagePullPolicy is Never and contents are not in the cache so we return
	// an error.
	if rc == nil && pullPolicyNever {
		err := errors.New(errPullPolicyNever)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonParse, err))

		return reconcile.Result{}, err
	}

	// If we didn't get a ReadCloser from cache, we need to get it from image.
	if rc == nil {
		// Initialize parser backend to obtain package contents.
		imgrc, err := r.backend.Init(ctx, PackageRevision(pr))
		if err != nil {
			err = errors.Wrap(err, errInitParserBackend)
			pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
			_ = r.client.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonParse, err))

			// Requeue because we may be waiting for parent package
			// controller to recreate Pod.
			return reconcile.Result{}, err
		}

		// Package is not in cache, so we write it to the cache while parsing.
		pipeR, pipeW := io.Pipe()
		rc = xpkg.TeeReadCloser(imgrc, pipeW)
		go func() {
			defer pipeR.Close() //nolint:errcheck // Not much we can do if this fails.
			if err := r.cache.Store(pr.GetName(), pipeR); err != nil {
				_ = pipeR.CloseWithError(err)
				cacheWrite <- err
				return
			}
			close(cacheWrite)
		}()
	}

	// Parse package contents.
	pkg, err := r.parser.Parse(ctx, struct {
		io.Reader
		io.Closer
	}{
		Reader: io.LimitReader(rc, maxPackageSize),
		Closer: rc,
	})
	// Wait until we finish writing to cache. Parser closes the reader.
	if err := <-cacheWrite; err != nil {
		// If we failed to cache we want to cleanup, but we don't abort unless
		// parsing also failed. Subsequent reconciles will pull image again and
		// attempt to cache.
		if err := r.cache.Delete(id); err != nil {
			log.Debug(errDeleteCache, "error", err)
		}
	}
	if err != nil {
		err = errors.Wrap(err, errParsePackage)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonParse, err))
		return reconcile.Result{}, err
	}

	// Lint package using package-specific linter.
	if err := r.linter.Lint(pkg); err != nil {
		err = errors.Wrap(err, errLintPackage)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonLint, err))

		// NOTE(hasheddan): a failed lint typically will require manual
		// intervention, but on the off chance that we read pod logs
		// early, which caused a linting failure, we will requeue by
		// returning an error.
		return reconcile.Result{}, err
	}

	// NOTE(hasheddan): the linter should check this property already, but
	// if a consumer forgets to pass an option to guarantee one meta object,
	// we check here to avoid a potential panic on 0 index below.
	if len(pkg.GetMeta()) != 1 {
		err = errors.New(errNotOneMeta)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonLint, err))

		return reconcile.Result{}, err
	}

	pkgMeta, _ := xpkg.TryConvert(pkg.GetMeta()[0], &pkgmetav1.Provider{}, &pkgmetav1.Configuration{}, &pkgmetav1beta1.Function{})

	pmo := pkgMeta.(metav1.Object)
	meta.AddLabels(pr, pmo.GetLabels())
	meta.AddAnnotations(pr, pmo.GetAnnotations())
	if err := r.client.Update(ctx, pr); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errUpdateMeta)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	// Check Crossplane constraints if they exist.
	if pr.GetIgnoreCrossplaneConstraints() == nil || !*pr.GetIgnoreCrossplaneConstraints() {
		if err := xpkg.PackageCrossplaneCompatible(r.versioner)(pkgMeta); err != nil {
			err = errors.Wrap(err, errIncompatible)
			pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))

			r.record.Event(pr, event.Warning(reasonLint, err))

			// No need to requeue if outside version constraints.
			// Package will either need to be updated or ignore
			// crossplane constraints will need to be specified,
			// both of which will trigger a new reconcile.
			return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
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
			pr.SetConditions(v1.UnknownHealth().WithMessage(err.Error()))
			_ = r.client.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonDependencies, err))

			return reconcile.Result{}, err
		}
	}

	if r.runtimeHook != nil {
		pwr := pr.(v1.PackageRevisionWithRuntime)
		if err := r.runtimeHook.Pre(ctx, pkgMeta, pwr, runtimeManifestBuilder); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errPreHook)
			pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
			_ = r.client.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonSync, err))

			return reconcile.Result{}, err
		}
	}

	// Establish control or ownership of objects.
	refs, err := r.objects.Establish(ctx, pkg.GetObjects(), pr, pr.GetDesiredState() == v1.PackageRevisionActive)
	if err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errEstablishControl)
		pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

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

	if r.runtimeHook != nil {
		if err := r.runtimeHook.Post(ctx, pkgMeta, pr.(v1.PackageRevisionWithRuntime), runtimeManifestBuilder); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errPostHook)
			pr.SetConditions(v1.Unhealthy().WithMessage(err.Error()))
			_ = r.client.Status().Update(ctx, pr)

			r.record.Event(pr, event.Warning(reasonSync, err))

			return reconcile.Result{}, err
		}
	}

	if pr.GetCondition(v1.TypeHealthy).Status != corev1.ConditionTrue {
		// NOTE(phisco): We don't want to spam the user with events if the
		// package revision is already healthy.
		r.record.Event(pr, event.Normal(reasonSync, "Successfully configured package revision"))
	}
	pr.SetConditions(v1.Healthy())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
}

func (r *Reconciler) deactivateRevision(ctx context.Context, pr v1.PackageRevision, runtimeManifestBuilder ManifestBuilder) error {
	// Remove self from the lock if we are present.
	if err := r.lock.RemoveSelf(ctx, pr); err != nil {
		return errors.Wrap(err, errRemoveLock)
	}

	// ReleaseObjects control of objects.
	if err := r.objects.ReleaseObjects(ctx, pr); err != nil {
		return errors.Wrap(err, errReleaseObjects)
	}

	if r.runtimeHook == nil {
		return nil
	}

	// Call deactivation hook.
	if err := r.runtimeHook.Deactivate(ctx, pr.(v1.PackageRevisionWithRuntime), runtimeManifestBuilder); err != nil {
		return errors.Wrap(err, errDeactivationHook)
	}

	return nil
}

func (r *Reconciler) runtimeManifestBuilderOptions(ctx context.Context, pwr v1.PackageRevisionWithRuntime) ([]RuntimeManifestBuilderOption, error) {
	var opts []RuntimeManifestBuilderOption

	if r.features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
		rcRef := pwr.GetRuntimeConfigRef()
		if rcRef == nil {
			return nil, errors.New(errNoRuntimeConfig)
		}

		rc := &v1beta1.DeploymentRuntimeConfig{}
		if err := r.client.Get(ctx, types.NamespacedName{Name: rcRef.Name}, rc); err != nil {
			return nil, errors.Wrap(err, errGetRuntimeConfig)
		}
		opts = append(opts, RuntimeManifestBuilderWithRuntimeConfig(rc))
	}

	// Note(turkenh): Until we completely remove the old controller config
	// reference, we support both the old and the new way with DeploymentRuntimeConfig.
	// If both are specified, we will start with DeploymentRuntimeConfig as the
	// base, apply optional and mandatory overrides and finally apply the
	// ControllerConfig on top. While it sounds like we are giving precedence
	// to the ControllerConfig, this is to make sure that we keep the old
	// behavior of the controller config reference for existing users.
	cc := &v1alpha1.ControllerConfig{}
	if ccRef := pwr.GetControllerConfigRef(); ccRef != nil {
		if err := r.client.Get(ctx, types.NamespacedName{Name: ccRef.Name}, cc); err != nil {
			return nil, errors.Wrap(err, errGetControllerConfig)
		}
		opts = append(opts, RuntimeManifestBuilderWithControllerConfig(cc))
	}

	sa := &corev1.ServiceAccount{}
	// Fetch XP ServiceAccount to get the ImagePullSecrets defined there.
	// We will append them to the list of ImagePullSecrets for the runtime
	// ServiceAccount.
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: r.serviceAccount}, sa); err != nil {
		return nil, errors.Wrap(err, errGetServiceAccount)
	}
	if len(sa.ImagePullSecrets) > 0 {
		opts = append(opts, RuntimeManifestBuilderWithServiceAccountPullSecrets(sa.ImagePullSecrets))
	}

	return opts, nil
}
