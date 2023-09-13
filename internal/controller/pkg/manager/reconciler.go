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
	"math"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	reconcileTimeout = 1 * time.Minute

	// pullWait is the time after which the package manager will check for
	// updated content for the given package reference. This behavior is only
	// enabled when the packagePullPolicy is Always.
	pullWait = 1 * time.Minute
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

	errUpdateStatus                  = "cannot update package status"
	errUpdateInactivePackageRevision = "cannot update inactive package revision"

	errUnhealthyPackageRevision     = "current package revision is unhealthy"
	errUnknownPackageRevisionHealth = "current package revision health is unknown"

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
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithWebhookTLSSecretName configures the name of the webhook TLS Secret that
// Reconciler will add to PackageRevisions it creates.
func WithWebhookTLSSecretName(n string) ReconcilerOption {
	return func(r *Reconciler) {
		r.webhookTLSSecretName = &n
	}
}

// WithESSTLSSecretName configures the name of the ESS TLS certificate secret that
// Reconciler will add to PackageRevisions it creates.
func WithESSTLSSecretName(s *string) ReconcilerOption {
	return func(r *Reconciler) {
		r.essTLSSecretName = s
	}
}

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

// Reconciler reconciles packages.
type Reconciler struct {
	client               resource.ClientApplicator
	pkg                  Revisioner
	log                  logging.Logger
	record               event.Recorder
	webhookTLSSecretName *string
	essTLSSecretName     *string

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

	opts := []ReconcilerOption{
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(f, WithDefaultRegistry(o.DefaultRegistry))),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}
	if o.WebhookTLSSecretName != "" {
		opts = append(opts, WithWebhookTLSSecretName(o.WebhookTLSSecretName))
	}
	if o.ESSOptions != nil && o.ESSOptions.TLSSecretName != nil {
		opts = append(opts, WithESSTLSSecretName(o.ESSOptions.TLSSecretName))
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Provider{}).
		Owns(&v1.ProviderRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, NewReconciler(mgr, opts...), o.GlobalRateLimiter))
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

	r := NewReconciler(mgr,
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(fetcher, WithDefaultRegistry(o.DefaultRegistry))),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.Configuration{}).
		Owns(&v1.ConfigurationRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// SetupFunction adds a controller that reconciles Functions.
func SetupFunction(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1beta1.FunctionGroupKind)
	np := func() v1.Package { return &v1beta1.Function{} }
	nr := func() v1.PackageRevision { return &v1beta1.FunctionRevision{} }
	nrl := func() v1.PackageRevisionList { return &v1beta1.FunctionRevisionList{} }

	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errCreateK8sClient)
	}
	f, err := xpkg.NewK8sFetcher(cs, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, errBuildFetcher)
	}

	opts := []ReconcilerOption{
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithRevisioner(NewPackageRevisioner(f, WithDefaultRegistry(o.DefaultRegistry))),
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Function{}).
		Owns(&v1beta1.FunctionRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, NewReconciler(mgr, opts...), o.GlobalRateLimiter))
}

// NewReconciler creates a new package reconciler.
func NewReconciler(mgr ctrl.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
		},
		pkg:    NewNopRevisioner(),
		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcilers are complex. Be wary of adding more.
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

	log = log.WithValues(
		"uid", p.GetUID(),
		"version", p.GetResourceVersion(),
		"name", p.GetName(),
	)

	// Get existing package revisions.
	prs := r.newPackageRevisionList()
	if err := r.client.List(ctx, prs, client.MatchingLabels(map[string]string{v1.LabelParentPackage: p.GetName()})); resource.IgnoreNotFound(err) != nil {
		log.Debug(errListRevisions, "error", err)
		err = errors.Wrap(err, errListRevisions)
		r.record.Event(p, event.Warning(reasonList, err))
		return reconcile.Result{}, err
	}

	revisionName, err := r.pkg.Revision(ctx, p)
	if err != nil {
		log.Debug(errUnpack, "error", err)
		err = errors.Wrap(err, errUnpack)
		p.SetConditions(v1.Unpacking().WithMessage(err.Error()))
		r.record.Event(p, event.Warning(reasonUnpack, err))

		if updateErr := r.client.Status().Update(ctx, p); updateErr != nil {
			return reconcile.Result{}, errors.Wrap(updateErr, errUpdateStatus)
		}

		return reconcile.Result{}, err
	}

	if revisionName == "" {
		p.SetConditions(v1.Unpacking())
		r.record.Event(p, event.Normal(reasonUnpack, "Waiting for unpack to complete"))
		return reconcile.Result{Requeue: true}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Set the current revision and identifier.
	p.SetCurrentRevision(revisionName)
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
				log.Debug(errUpdateInactivePackageRevision, "error", err)
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
			log.Debug(errGCPackageRevision, "error", err)
			err = errors.Wrap(err, errGCPackageRevision)
			r.record.Event(p, event.Warning(reasonGarbageCollect, err))
			return reconcile.Result{}, err
		}
	}

	if pr.GetCondition(v1.TypeHealthy).Status == corev1.ConditionTrue {
		p.SetConditions(v1.Healthy())
		r.record.Event(p, event.Normal(reasonInstall, "Successfully installed package revision"))
	}
	if pr.GetCondition(v1.TypeHealthy).Status == corev1.ConditionFalse {
		p.SetConditions(v1.Unhealthy())
		r.record.Event(p, event.Warning(reasonInstall, errors.New(errUnhealthyPackageRevision)))
	}
	if pr.GetCondition(v1.TypeHealthy).Status == corev1.ConditionUnknown {
		p.SetConditions(v1.UnknownHealth())
		r.record.Event(p, event.Warning(reasonInstall, errors.New(errUnknownPackageRevisionHealth)))
	}

	// Create the non-existent package revision.
	pr.SetName(revisionName)
	pr.SetLabels(map[string]string{v1.LabelParentPackage: p.GetName()})
	pr.SetSource(p.GetSource())
	pr.SetPackagePullPolicy(p.GetPackagePullPolicy())
	pr.SetPackagePullSecrets(p.GetPackagePullSecrets())
	pr.SetIgnoreCrossplaneConstraints(p.GetIgnoreCrossplaneConstraints())
	pr.SetSkipDependencyResolution(p.GetSkipDependencyResolution())
	pr.SetControllerConfigRef(p.GetControllerConfigRef())
	pr.SetWebhookTLSSecretName(r.webhookTLSSecretName)
	pr.SetESSTLSSecretName(r.essTLSSecretName)
	pr.SetTLSServerSecretName(p.GetTLSServerSecretName())
	pr.SetTLSClientSecretName(p.GetTLSClientSecretName())
	pr.SetCommonLabels(p.GetCommonLabels())

	// If current revision is not active and we have an automatic or
	// undefined activation policy, always activate.
	if pr.GetDesiredState() != v1.PackageRevisionActive && (p.GetActivationPolicy() == nil || *p.GetActivationPolicy() == v1.AutomaticActivation) {
		pr.SetDesiredState(v1.PackageRevisionActive)
	}

	controlRef := meta.AsController(meta.TypedReferenceTo(p, p.GetObjectKind().GroupVersionKind()))
	controlRef.BlockOwnerDeletion = pointer.Bool(true)
	meta.AddOwnerReference(pr, controlRef)
	if err := r.client.Apply(ctx, pr, resource.MustBeControllableBy(p.GetUID())); err != nil {
		log.Debug(errApplyPackageRevision, "error", err)
		err = errors.Wrap(err, errApplyPackageRevision)
		r.record.Event(p, event.Warning(reasonInstall, err))
		return reconcile.Result{}, err
	}

	// Handle changes in labels
	same := reflect.DeepEqual(pr.GetCommonLabels(), p.GetCommonLabels())
	if !same {
		pr.SetCommonLabels(p.GetCommonLabels())
		if err := r.client.Update(ctx, pr); err != nil {
			log.Debug(errApplyPackageRevision, "error", err)
			err = errors.Wrap(err, errApplyPackageRevision)
			r.record.Event(p, event.Warning(reasonInstall, err))
			return reconcile.Result{}, err
		}
	}

	p.SetConditions(v1.Active())

	// If current revision is still not active, the package is inactive.
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		p.SetConditions(v1.Inactive())
	}

	// NOTE(hasheddan): when the first package revision is created for a
	// package, the health of the package is not set until the revision reports
	// its health. If updating from an existing revision, the package health
	// will match the health of the old revision until the next reconcile.
	return pullBasedRequeue(p.GetPackagePullPolicy()), errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
}
