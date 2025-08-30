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

// Package manager implements the Crossplane Package controllers.
package manager

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

const (
	reconcileTimeout = 1 * time.Minute

	// pullWait is the time after which the package manager will check for
	// updated content for the given package reference. This behavior is only
	// enabled when the packagePullPolicy is Always.
	pullWait = 1 * time.Minute

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

func pullBasedRequeue(p *corev1.PullPolicy) reconcile.Result {
	if p != nil && *p == corev1.PullAlways {
		return reconcile.Result{RequeueAfter: pullWait}
	}

	return reconcile.Result{Requeue: false}
}

const (
	errGetPackage           = "cannot get package"
	errListRevisions        = "cannot list revisions for package"
	errUnpack               = "cannot unpack package"
	errApplyPackageRevision = "cannot apply package revision"
	errGCPackageRevision    = "cannot garbage collect old package revision"
	errGetPullConfig        = "cannot get image pull secret from config"
	errRewriteImage         = "cannot rewrite image path using config"

	errUpdateStatus                  = "cannot update package status"
	errUpdateInactivePackageRevision = "cannot update inactive package revision"

	errCreateK8sClient = "failed to initialize clientset"
	errBuildFetcher    = "cannot build fetcher"
)

// Event reasons.
const (
	reasonList               event.Reason = "ListRevision"
	reasonUnpack             event.Reason = "UnpackPackage"
	reasonTransitionRevision event.Reason = "TransitionRevision"
	reasonGarbageCollect     event.Reason = "GarbageCollect"
	reasonInstall            event.Reason = "InstallPackageRevision"
	reasonPaused             event.Reason = "ReconciliationPaused"
	reasonImageConfig        event.Reason = "ImageConfigSelection"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithNewPackageFn determines the type of package being reconciled.
func WithNewPackageFn(f func() v1.Package) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackage = f
	}
}

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevision = f
	}
}

// WithNewPackageRevisionListFn determines the type of package being reconciled.
func WithNewPackageRevisionListFn(f func() v1.PackageRevisionList) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevisionList = f
	}
}

// WithRevisioner specifies how the Reconciler should acquire a package image's
// revision name.
func WithRevisioner(d Revisioner) ReconcilerOption {
	return func(r *Reconciler) {
		r.pkg = d
	}
}

// WithConfigStore specifies the image config store to use.
func WithConfigStore(c xpkg.ConfigStore) ReconcilerOption {
	return func(r *Reconciler) {
		r.config = c
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

// WithManagingRevisionRuntimeSpec will allow this reconciler to propagate the
// runtime spec fields to revisions.
func WithManagingRevisionRuntimeSpec() ReconcilerOption {
	return func(r *Reconciler) {
		r.setPackageRuntimeManagedFields = func(p v1.Package, pr v1.PackageRevision) {
			pwr, pwok := p.(v1.PackageWithRuntime)

			prwr, prok := pr.(v1.PackageRevisionWithRuntime)
			if pwok && prok {
				prwr.SetRuntimeConfigRef(pwr.GetRuntimeConfigRef())
				prwr.SetTLSServerSecretName(pwr.GetTLSServerSecretName())
				prwr.SetTLSClientSecretName(pwr.GetTLSClientSecretName())
			}
		}
	}
}

// Reconciler reconciles packages.
type Reconciler struct {
	client     resource.ClientApplicator
	pkg        Revisioner
	config     xpkg.ConfigStore
	log        logging.Logger
	record     event.Recorder
	conditions conditions.Manager

	setPackageRuntimeManagedFields func(p v1.Package, pr v1.PackageRevision)

	newPackage             func() v1.Package
	newPackageRevision     func() v1.PackageRevision
	newPackageRevisionList func() v1.PackageRevisionList
}

// SetupProvider adds a controller that reconciles Providers.
func SetupProvider(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ProviderGroupKind)
	np := func() v1.Package { return &v1.Provider{} }
	nr := func() v1.PackageRevision { return &v1.ProviderRevision{} }
	nrl := func() v1.PackageRevisionList { return &v1.ProviderRevisionList{} }

	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCreateK8sClient)
	}

	f, err := xpkg.NewK8sFetcher(cs, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errBuildFetcher)
	}

	log := o.Logger.WithValues("controller", name)
	opts := []ReconcilerOption{
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(f)),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.PackageRuntime.For(v1.ProviderKind) == controller.PackageRuntimeDeployment {
		opts = append(opts, WithManagingRevisionRuntimeSpec())
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Provider{}).
		Owns(&v1.ProviderRevision{}).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.ProviderList{}, log)).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, opts...)), o.GlobalRateLimiter))
}

// SetupConfiguration adds a controller that reconciles Configurations.
func SetupConfiguration(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.ConfigurationGroupKind)
	np := func() v1.Package { return &v1.Configuration{} }
	nr := func() v1.PackageRevision { return &v1.ConfigurationRevision{} }
	nrl := func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize clientset")
	}

	fetcher, err := xpkg.NewK8sFetcher(clientset, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, "cannot build fetcher")
	}

	log := o.Logger.WithValues("controller", name)
	r := NewReconciler(mgr,
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(fetcher)),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Configuration{}).
		Owns(&v1.ConfigurationRevision{}).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.ConfigurationList{}, log)).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// SetupFunction adds a controller that reconciles Functions.
func SetupFunction(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1.FunctionGroupKind)
	np := func() v1.Package { return &v1.Function{} }
	nr := func() v1.PackageRevision { return &v1.FunctionRevision{} }
	nrl := func() v1.PackageRevisionList { return &v1.FunctionRevisionList{} }

	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCreateK8sClient)
	}

	f, err := xpkg.NewK8sFetcher(cs, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errBuildFetcher)
	}

	log := o.Logger.WithValues("controller", name)
	opts := []ReconcilerOption{
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(f)),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.PackageRuntime.For(v1.FunctionKind) == controller.PackageRuntimeDeployment {
		opts = append(opts, WithManagingRevisionRuntimeSpec())
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Function{}).
		Owns(&v1.FunctionRevision{}).
		Watches(&v1beta1.ImageConfig{}, EnqueuePackagesForImageConfig(mgr.GetClient(), &v1.FunctionList{}, log)).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, opts...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package reconciler.
func NewReconciler(mgr ctrl.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
		},
		pkg:        NewNopRevisioner(),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Reconcilers are complex. Be wary of adding more.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	p := r.newPackage()
	if err := r.client.Get(ctx, req.NamespacedName, p); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetPackage, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackage)
	}

	status := r.conditions.For(p)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(p) {
		r.record.Event(p, event.Normal(reasonPaused, reconcilePausedMsg))
		status.MarkConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
		// If the pause annotation is removed, we will have a chance to reconcile again and resume
		// and if status update fails, we will reconcile again to retry to update the status
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	if c := p.GetCondition(xpv1.ReconcilePaused().Type); c.Reason == xpv1.ReconcilePaused().Reason {
		p.CleanConditions()
		// Persist the removal of conditions and return. We'll be requeued
		// with the updated status and resume reconciliation.
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Get existing package revisions.
	prs := r.newPackageRevisionList()
	if err := r.client.List(ctx, prs, client.MatchingLabels(map[string]string{v1.LabelParentPackage: p.GetName()})); resource.IgnoreNotFound(err) != nil {
		err = errors.Wrap(err, errListRevisions)
		r.record.Event(p, event.Warning(reasonList, err))

		return reconcile.Result{}, err
	}

	// Rewrite the image path if necessary. We need to do this before looking
	// for pull secrets, since the rewritten path may use different secrets than
	// the original.
	imagePath := p.GetSource()

	rewriteConfigName, newPath, err := r.config.RewritePath(ctx, imagePath)
	if err != nil {
		err = errors.Wrap(err, errRewriteImage)
		p.SetConditions(v1.Unpacking().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, p)

		r.record.Event(p, event.Warning(reasonImageConfig, err))

		return reconcile.Result{}, err
	}

	if newPath != "" {
		imagePath = newPath

		p.SetAppliedImageConfigRefs(v1.ImageConfigRef{
			Name:   rewriteConfigName,
			Reason: v1.ImageConfigReasonRewrite,
		})
	} else {
		p.ClearAppliedImageConfigRef(v1.ImageConfigReasonRewrite)
	}

	p.SetResolvedSource(imagePath)

	pullSecretConfig, pullSecretFromConfig, err := r.config.PullSecretFor(ctx, p.GetResolvedSource())
	if err != nil {
		err = errors.Wrap(err, errGetPullConfig)
		status.MarkConditions(v1.Unpacking().WithMessage(err.Error()))

		_ = r.client.Status().Update(ctx, p)

		r.record.Event(p, event.Warning(reasonImageConfig, err))

		return reconcile.Result{}, err
	}

	var secrets []string
	if pullSecretFromConfig != "" {
		secrets = append(secrets, pullSecretFromConfig)

		p.SetAppliedImageConfigRefs(v1.ImageConfigRef{
			Name:   pullSecretConfig,
			Reason: v1.ImageConfigReasonSetPullSecret,
		})
	} else {
		p.ClearAppliedImageConfigRef(v1.ImageConfigReasonSetPullSecret)
	}

	revisionName, err := r.pkg.Revision(ctx, p, secrets...)
	if err != nil {
		err = errors.Wrap(err, errUnpack)
		status.MarkConditions(v1.Unpacking().WithMessage(err.Error()))
		r.record.Event(p, event.Warning(reasonUnpack, err))

		if updateErr := r.client.Status().Update(ctx, p); updateErr != nil {
			return reconcile.Result{}, errors.Wrap(updateErr, errUpdateStatus)
		}

		return reconcile.Result{}, err
	}

	if revisionName == "" {
		status.MarkConditions(v1.Unpacking().WithMessage("Waiting for unpack to complete"))
		r.record.Event(p, event.Normal(reasonUnpack, "Waiting for unpack to complete"))

		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Set the current revision and identifier.
	p.SetCurrentRevision(revisionName)
	// Use the original source as the identifier, even if it was rewritten by
	// ImageConfig. The revisioning and dependency resolution logic are all
	// based on the original package sources, so it's important that we preserve
	// the original until it's time to actually pull an image.
	p.SetCurrentIdentifier(p.GetSource())

	pr := r.newPackageRevision()
	maxRevision := int64(0)
	oldestRevision := int64(math.MaxInt64)
	oldestRevisionIndex := -1
	revisions := prs.GetRevisions()

	// Check to see if revision already exists.
	for index, rev := range revisions {
		revisionNum := rev.GetRevision()

		// Set max revision to the highest numbered existing revision.
		if revisionNum > maxRevision {
			maxRevision = revisionNum
		}

		// Set oldest revision to the lowest numbered revision and
		// record its index.
		if revisionNum < oldestRevision {
			oldestRevision = revisionNum
			oldestRevisionIndex = index
		}
		// If revision name is same as current revision, then revision
		// already exists.
		if rev.GetName() == p.GetCurrentRevision() {
			pr = rev
			// Finish iterating through all revisions to make sure
			// all non-current revisions are inactive.
			continue
		}

		if rev.GetDesiredState() == v1.PackageRevisionActive {
			// If revision is not the current revision, set to
			// inactive. This should always be done, regardless of
			// the package's revision activation policy.
			rev.SetDesiredState(v1.PackageRevisionInactive)

			if err := r.client.Apply(ctx, rev, resource.MustBeControllableBy(p.GetUID())); err != nil {
				if kerrors.IsConflict(err) {
					return reconcile.Result{Requeue: true}, nil
				}

				err = errors.Wrap(err, errUpdateInactivePackageRevision)
				r.record.Event(p, event.Warning(reasonTransitionRevision, err))

				return reconcile.Result{}, err
			}
		}
	}

	// The current revision should always be the highest numbered revision.
	if pr.GetRevision() < maxRevision || maxRevision == 0 {
		pr.SetRevision(maxRevision + 1)
	}

	// Check to see if there are revisions eligible for garbage collection.
	if p.GetRevisionHistoryLimit() != nil &&
		*p.GetRevisionHistoryLimit() != 0 &&
		len(revisions) > (int(*p.GetRevisionHistoryLimit())+1) {
		gcRev := revisions[oldestRevisionIndex]
		// Find the oldest revision and delete it.
		if err := r.client.Delete(ctx, gcRev); err != nil {
			err = errors.Wrap(err, errGCPackageRevision)
			r.record.Event(p, event.Warning(reasonGarbageCollect, err))

			return reconcile.Result{}, err
		}
	}

	health := v1.PackageHealth(pr)
	if health.Status == corev1.ConditionTrue && p.GetCondition(v1.TypeHealthy).Status != corev1.ConditionTrue {
		// NOTE(phisco): We don't want to spam the user with events if the
		// package is already healthy.
		r.record.Event(p, event.Normal(reasonInstall, "Successfully installed package revision"))
	}

	status.MarkConditions(health)

	if pr.GetUID() == "" && pullSecretConfig != "" {
		// We only record this event if the revision is new, as we don't want to
		// spam the user with events if the revision already exists.
		log.Debug("Selected pull secret from image config store", "image", p.GetResolvedSource(), "pullSecretConfig", pullSecretConfig, "pullSecret", pullSecretFromConfig, "rewriteConfig", rewriteConfigName)
		r.record.Event(p, event.Normal(reasonImageConfig, fmt.Sprintf("Selected pullSecret %q from ImageConfig %q for registry authentication", pullSecretFromConfig, pullSecretConfig)))
	}

	// Create the non-existent package revision.
	pr.SetName(revisionName)
	pr.SetLabels(map[string]string{v1.LabelParentPackage: p.GetName()})
	// Use the original source; the revision reconciler will rewrite it if
	// needed. The revision reconciler also inserts packages into the dependency
	// manager's lock, which must use the original source to ensure dependency
	// packages have the expected names even when rewritten.
	pr.SetSource(p.GetSource())
	pr.SetPackagePullPolicy(p.GetPackagePullPolicy())
	pr.SetPackagePullSecrets(p.GetPackagePullSecrets())
	pr.SetIgnoreCrossplaneConstraints(p.GetIgnoreCrossplaneConstraints())
	pr.SetSkipDependencyResolution(p.GetSkipDependencyResolution())
	pr.SetCommonLabels(p.GetCommonLabels())

	if r.setPackageRuntimeManagedFields != nil {
		r.setPackageRuntimeManagedFields(p, pr)
	}

	// If the current revision is not active, and we have an automatic or
	// undefined activation policy, always activate.
	if pr.GetDesiredState() != v1.PackageRevisionActive && (p.GetActivationPolicy() == nil || *p.GetActivationPolicy() == v1.AutomaticActivation) {
		pr.SetDesiredState(v1.PackageRevisionActive)
	}

	controlRef := meta.AsController(meta.TypedReferenceTo(p, p.GetObjectKind().GroupVersionKind()))
	controlRef.BlockOwnerDeletion = ptr.To(true)
	meta.AddOwnerReference(pr, controlRef)

	if err := r.client.Apply(ctx, pr, resource.MustBeControllableBy(p.GetUID())); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errApplyPackageRevision)
		r.record.Event(p, event.Warning(reasonInstall, err))

		return reconcile.Result{}, err
	}

	// Handle changes in labels
	same := reflect.DeepEqual(pr.GetCommonLabels(), p.GetCommonLabels())
	if !same {
		pr.SetCommonLabels(p.GetCommonLabels())

		if err := r.client.Update(ctx, pr); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}

			err = errors.Wrap(err, errApplyPackageRevision)
			r.record.Event(p, event.Warning(reasonInstall, err))

			return reconcile.Result{}, err
		}
	}

	status.MarkConditions(v1.Active())

	// If current revision is still not active, the package is inactive.
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		status.MarkConditions(v1.Inactive().WithMessage("Package is inactive"))
	}

	// NOTE(hasheddan): when the first package revision is created for a
	// package, the health of the package is not set until the revision reports
	// its health. If updating from an existing revision, the package health
	// will match the health of the old revision until the next reconcile.
	return pullBasedRequeue(p.GetPackagePullPolicy()), errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
}
