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

package manager

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	parentLabel      = "pkg.crossplane.io/package"
	reconcileTimeout = 1 * time.Minute

	shortWait     = 30 * time.Second
	veryShortWait = 5 * time.Second
)

const (
	errGetPackage            = "cannot get package"
	errListRevisions         = "cannot list revisions for package"
	errUnpack                = "cannot unpack package"
	errCreatePackageRevision = "cannot create package revision"
	errGCPackageRevision     = "cannot garbage collect old package revision"
	errGCInstallPod          = "cannot garbage collect old package revision install pod"

	errUpdateStatus                  = "cannot update package status"
	errUpdateInactivePackageRevision = "cannot update inactive package revision"
	errUpdateActivePackageRevision   = "cannot update active package revision"

	errUnhealthyPackageRevision = "current package revision is unhealthy"
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

// WithNewPackageFn determines the type of package being reconciled.
func WithNewPackageFn(f func() v1alpha1.Package) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackage = f
	}
}

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1alpha1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevision = f
	}
}

// WithNewPackageRevisionListFn determines the type of package being reconciled.
func WithNewPackageRevisionListFn(f func() v1alpha1.PackageRevisionList) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevisionList = f
	}
}

// WithPodManager specifies how the Reconciler should manage install pods.
func WithPodManager(m PodManager) ReconcilerOption {
	return func(r *Reconciler) {
		r.podManager = m
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
	client     resource.ClientApplicator
	podManager PodManager
	log        logging.Logger
	record     event.Recorder

	newPackage             func() v1alpha1.Package
	newPackageRevision     func() v1alpha1.PackageRevision
	newPackageRevisionList func() v1alpha1.PackageRevisionList
}

// SetupProvider adds a controller that reconciles Providers.
func SetupProvider(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ProviderGroupKind)
	np := func() v1alpha1.Package { return &v1alpha1.Provider{} }
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }
	nrl := func() v1alpha1.PackageRevisionList { return &v1alpha1.ProviderRevisionList{} }

	r := NewReconciler(mgr,
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithPodManager(NewPackagePodManager(mgr.GetClient(), namespace)),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Provider{}).
		Owns(&v1alpha1.ProviderRevision{}).
		Complete(r)
}

// SetupConfiguration adds a controller that reconciles Configurations.
func SetupConfiguration(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ConfigurationGroupKind)
	np := func() v1alpha1.Package { return &v1alpha1.Configuration{} }
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }
	nrl := func() v1alpha1.PackageRevisionList { return &v1alpha1.ConfigurationRevisionList{} }

	r := NewReconciler(mgr,
		WithNewPackageFn(np),
		WithNewPackageRevisionFn(nr),
		WithNewPackageRevisionListFn(nrl),
		WithPodManager(NewPackagePodManager(mgr.GetClient(), namespace)),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Configuration{}).
		Owns(&v1alpha1.ConfigurationRevision{}).
		Complete(r)
}

// NewReconciler creates a new package reconciler.
func NewReconciler(mgr ctrl.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIPatchingApplicator(mgr.GetClient()),
		},
		podManager: NewPackagePodManager(mgr.GetClient(), "crossplane-system"),
		log:        logging.NewNopLogger(),
		record:     event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	p := r.newPackage()
	if err := r.client.Get(ctx, req.NamespacedName, p); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
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
	if err := r.client.List(ctx, prs, client.MatchingLabels(map[string]string{parentLabel: p.GetName()})); resource.IgnoreNotFound(err) != nil {
		log.Debug(errListRevisions, "error", err)
		r.record.Event(p, event.Warning(reasonList, errors.Wrap(err, errListRevisions)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	p.SetConditions(v1alpha1.Unpacking())

	hash, err := r.podManager.Sync(ctx, p)
	if err != nil {
		log.Debug(errUnpack, "error", err)
		r.record.Event(p, event.Warning(reasonUnpack, errors.Wrap(err, errUnpack)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	if hash == "" {
		r.record.Event(p, event.Normal(reasonUnpack, "Waiting for unpack to complete"))
		return reconcile.Result{RequeueAfter: veryShortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Set the current revision to match the install job for image.
	p.SetCurrentRevision(packNHash(p.GetName(), hash))

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

		// Set oldest revision to the lowest numbered revision and record its
		// index.
		if revisionNum < oldestRevision {
			oldestRevision = revisionNum
			oldestRevisionIndex = index
		}
		// If revision name is same as current revision, then revision already exists.
		if rev.GetName() == p.GetCurrentRevision() {
			pr = rev
			// If current revision exists, is not active, and package has an
			// automatic activation policy, we should set the revision to
			// active.
			p.SetConditions(v1alpha1.Active())
			if p.GetActivationPolicy() == v1alpha1.AutomaticActivation {
				pr.SetDesiredState(v1alpha1.PackageRevisionActive)
			}
			// Finish iterating through all revisions to make sure all
			// non-current revisions are inactive.
			continue
		}
		if rev.GetDesiredState() == v1alpha1.PackageRevisionActive {
			// If revision is not the current revision, set to inactive. This
			// should always be done, regardless of the package's revision
			// activation policy.
			rev.SetDesiredState(v1alpha1.PackageRevisionInactive)
			if err := r.client.Apply(ctx, rev, resource.MustBeControllableBy(p.GetUID())); err != nil {
				log.Debug(errUpdateInactivePackageRevision, "error", err)
				r.record.Event(p, event.Warning(reasonTransitionRevision, errors.Wrap(err, errUpdateInactivePackageRevision)))
				return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
			}
		}
	}

	// The current revision should always be the highest numbered revision.
	if pr.GetRevision() < maxRevision || maxRevision == 0 {
		pr.SetRevision(maxRevision + 1)
	}

	// Check to see if there are revisions eligible for garbage collection.
	if len(revisions) > (int(p.GetRevisionHistoryLimit())+1) && p.GetRevisionHistoryLimit() != 0 {
		gcRev := revisions[oldestRevisionIndex]
		// Find the oldest revision and delete it.
		if err := r.client.Delete(ctx, gcRev); err != nil {
			log.Debug(errGCPackageRevision, "error", err)
			r.record.Event(p, event.Warning(reasonGarbageCollect, errors.Wrap(err, errGCPackageRevision)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}

		// Clean up the oldest revision's Pod as well.
		if err := r.podManager.GarbageCollect(ctx, gcRev.GetSource(), p); err != nil {
			log.Debug(errGCInstallPod, "error", err)
			r.record.Event(p, event.Warning(reasonGarbageCollect, errors.Wrap(err, errGCInstallPod)))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
		}
	}

	// If the name of the package revision matches the current revision of the
	// package, update and return.
	if pr.GetName() == p.GetCurrentRevision() {
		// Make sure install pod matches.
		pr.SetInstallPod(runtimev1alpha1.Reference{Name: imageToPod(p.GetSource())})
		if err := r.client.Apply(ctx, pr); err != nil {
			log.Debug(errUpdateActivePackageRevision, "error", err)
			r.record.Event(p, event.Warning(reasonInstall, errors.Wrap(err, errUpdateActivePackageRevision)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		// Update Package status to match that of revision.
		if pr.GetCondition(v1alpha1.TypeHealthy).Status == corev1.ConditionTrue {
			p.SetConditions(v1alpha1.Healthy())
			r.record.Event(p, event.Normal(reasonInstall, "Successfully installed package revision"))
		} else {
			p.SetConditions(v1alpha1.Unhealthy())
			r.record.Event(p, event.Warning(reasonInstall, errors.New(errUnhealthyPackageRevision)))
		}
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// Create the non-existent package revision.
	pr.SetName(packNHash(p.GetName(), hash))
	pr.SetLabels(map[string]string{parentLabel: p.GetName()})
	pr.SetInstallPod(runtimev1alpha1.Reference{Name: imageToPod(p.GetSource())})
	pr.SetSource(p.GetSource())

	pr.SetDesiredState(v1alpha1.PackageRevisionInactive)
	p.SetConditions(v1alpha1.Inactive())
	if p.GetActivationPolicy() == v1alpha1.AutomaticActivation {
		pr.SetDesiredState(v1alpha1.PackageRevisionActive)
		p.SetConditions(v1alpha1.Active())
	}

	meta.AddOwnerReference(pr, meta.AsController(meta.TypedReferenceTo(p, p.GetObjectKind().GroupVersionKind())))
	if err := r.client.Apply(ctx, pr, resource.MustBeControllableBy(p.GetUID())); err != nil {
		log.Debug(errCreatePackageRevision, "error", err)
		r.record.Event(p, event.Warning(reasonInstall, errors.Wrap(err, errCreatePackageRevision)))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
	}

	// NOTE(hasheddan): when the first package revision is created for a
	// package, the health of the package is not set until the revision reports
	// its health. If updating from an existing revision, the package health
	// will match the health of the old revision until the next reconcile.
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), errUpdateStatus)
}
