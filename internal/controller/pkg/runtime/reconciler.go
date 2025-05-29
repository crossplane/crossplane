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

// Package runtime implements "Deployment" runtime for Crossplane packages.
package runtime

import (
	"context"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/controller/pkg/revision"
	"github.com/crossplane/crossplane/internal/features"
)

const (
	reconcileTimeout = 3 * time.Minute
)

const (
	errGetPackageRevision = "cannot get package revision"
	errUpdateStatus       = "cannot update package revision status"

	errGetPullConfig = "cannot get image pull secret from config"
	errRewriteImage  = "cannot rewrite image path using config"

	errManifestBuilderOptions = "cannot prepare runtime manifest builder options"
	errPreHook                = "pre establish runtime hook failed for package"
	errPostHook               = "post establish runtime hook failed for package"

	errNoRuntimeConfig   = "no deployment runtime config set"
	errGetRuntimeConfig  = "cannot get referenced deployment runtime config"
	errGetServiceAccount = "cannot get Crossplane service account"

	reconcilePausedMsg = "Reconciliation (including deletion) is paused via the pause annotation"
)

// Event reasons.
const (
	reasonImageConfig event.Reason = "FetchResolvedImageConfig"
	reasonSync        event.Reason = "SyncPackage"
	reasonPaused      event.Reason = "ReconciliationPaused"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithNewPackageRevisionWithRuntimeFn determines the type of package being reconciled.
func WithNewPackageRevisionWithRuntimeFn(f func() v1.PackageRevisionWithRuntime) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevisionWithRuntime = f
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

// WithRuntimeHooks specifies how the Reconciler should perform preparations
// (pre- and post-establishment) and cleanup (deactivate) for package runtime.
// The hooks are only used when the package has a runtime and the runtime is
// configured as Deployment.
func WithRuntimeHooks(h Hooks) ReconcilerOption {
	return func(r *Reconciler) {
		r.runtimeHook = h
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
	client         client.Client
	log            logging.Logger
	runtimeHook    Hooks
	record         event.Recorder
	conditions     conditions.Manager
	features       *feature.Flags
	namespace      string
	serviceAccount string

	newPackageRevisionWithRuntime func() v1.PackageRevisionWithRuntime
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "package-runtime/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	nr := func() v1.PackageRevisionWithRuntime { return &v1.ProviderRevision{} }

	log := o.Logger.WithValues("controller", name)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&v1beta1.ImageConfig{}, revision.EnqueuePackageRevisionsForImageConfig(mgr.GetClient(), &v1.ProviderRevisionList{}, log))

	ro := []ReconcilerOption{
		WithNewPackageRevisionWithRuntimeFn(nr),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithRuntimeHooks(NewProviderHooks(mgr.GetClient(), o.DefaultRegistry)),
		WithFeatureFlags(o.Features),
	}

	if o.Features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
		cb = cb.Watches(&v1beta1.DeploymentRuntimeConfig{}, EnqueuePackageRevisionsForRuntimeConfig(mgr.GetClient(), &v1.ProviderRevisionList{}, log))
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, ro...)), o.GlobalRateLimiter))
}

// SetupFunctionRevision adds a controller that reconciles FunctionRevisions.
func SetupFunctionRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "package-runtime/" + strings.ToLower(v1.FunctionRevisionGroupKind)
	nr := func() v1.PackageRevisionWithRuntime { return &v1.FunctionRevision{} }

	log := o.Logger.WithValues("controller", name)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.FunctionRevision{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(&v1beta1.ImageConfig{}, revision.EnqueuePackageRevisionsForImageConfig(mgr.GetClient(), &v1.FunctionRevisionList{}, log))

	ro := []ReconcilerOption{
		WithNewPackageRevisionWithRuntimeFn(nr),
		WithLogger(log),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithRuntimeHooks(NewFunctionHooks(mgr.GetClient(), o.DefaultRegistry)),
		WithFeatureFlags(o.Features),
	}

	if o.Features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
		cb = cb.Watches(&v1beta1.DeploymentRuntimeConfig{}, EnqueuePackageRevisionsForRuntimeConfig(mgr.GetClient(), &v1.FunctionRevisionList{}, log))
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, ro...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
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

	pr := r.newPackageRevisionWithRuntime()
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
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
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	if c := pr.GetCondition(xpv1.ReconcilePaused().Type); c.Reason == xpv1.ReconcilePaused().Reason {
		pr.CleanConditions()
		// Persist the removal of conditions and return. We'll be requeued
		// with the updated status and resume reconciliation.
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	if r.features.Enabled(features.EnableAlphaSignatureVerification) {
		// Wait for signature verification to complete before proceeding.
		if cond := pr.GetCondition(v1.TypeVerified); cond.Status != corev1.ConditionTrue {
			log.Debug("Waiting for signature verification controller to complete verification.", "condition", cond)
			// Initialize the installed condition if they are not already set to
			// communicate the status of the package.
			if pr.GetCondition(v1.TypeHealthy).Status == corev1.ConditionUnknown {
				status.MarkConditions(v1.AwaitingVerification())
				return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), "cannot update status with awaiting verification")
			}
			return reconcile.Result{}, nil
		}
	}

	var pullSecretFromConfig string
	// Read applied image config for SetImagePullSecret from the package
	// revision status, so that we can use the same pull secret without having
	// to resolve it again.
	for _, icr := range pr.GetAppliedImageConfigRefs() {
		if icr.Reason == v1.ImageConfigReasonSetPullSecret {
			// Get applied image config to find the pull secret.
			ic := &v1beta1.ImageConfig{}
			if err := r.client.Get(ctx, types.NamespacedName{Name: icr.Name}, ic); err != nil {
				err = errors.Wrap(err, errGetPullConfig)
				status.MarkConditions(v1.Unhealthy().WithMessage(err.Error()))
				_ = r.client.Status().Update(ctx, pr)

				r.record.Event(pr, event.Warning(reasonImageConfig, err))

				return reconcile.Result{}, err
			}
			pullSecretFromConfig = ic.Spec.Registry.Authentication.PullSecretRef.Name
			break
		}
	}

	// Initialize the runtime manifest builder with the package revision
	opts, err := r.builderOptions(ctx, pr)
	if err != nil {
		log.Debug(errManifestBuilderOptions, "error", err)

		err = errors.Wrap(err, errManifestBuilderOptions)
		status.MarkConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}
	if pullSecretFromConfig != "" {
		opts = append(opts, BuilderWithPullSecrets(pullSecretFromConfig))
	}
	builder := NewDeploymentRuntimeBuilder(pr, r.namespace, opts...)

	// Deactivate revision if it is inactive.
	if pr.GetDesiredState() == v1.PackageRevisionInactive {
		if err := r.runtimeHook.Deactivate(ctx, pr, builder); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err := errors.Wrap(err, "failed to run deactivation hook")
			r.log.Info("Error", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: false}, nil
	}

	// Run pre-establish hooks
	if err := r.runtimeHook.Pre(ctx, pr, builder); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errPreHook)
		status.MarkConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	// Wait for the package revision to be established.
	if pr.GetCondition(v1.TypeInstalled).Status != corev1.ConditionTrue {
		r.log.Debug("Waiting for the package revision to be established")
		status.MarkConditions(v1.Unhealthy().WithMessage("Package revision is not established yet"))
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Run post-establish hooks
	if err := r.runtimeHook.Post(ctx, pr, builder); err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}

		err = errors.Wrap(err, errPostHook)
		status.MarkConditions(v1.Unhealthy().WithMessage(err.Error()))
		_ = r.client.Status().Update(ctx, pr)

		r.record.Event(pr, event.Warning(reasonSync, err))

		return reconcile.Result{}, err
	}

	if pr.GetCondition(v1.TypeHealthy).Status != corev1.ConditionTrue {
		// We don't want to spam the user with events if the package revision is
		// already healthy.
		r.record.Event(pr, event.Normal(reasonSync, "Successfully configured package revision"))
	}

	status.MarkConditions(v1.Healthy())
	return reconcile.Result{Requeue: false}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
}

func (r *Reconciler) builderOptions(ctx context.Context, pwr v1.PackageRevisionWithRuntime) ([]BuilderOption, error) {
	var opts []BuilderOption

	if r.features.Enabled(features.EnableBetaDeploymentRuntimeConfigs) {
		rcRef := pwr.GetRuntimeConfigRef()
		if rcRef == nil {
			return nil, errors.New(errNoRuntimeConfig)
		}

		rc := &v1beta1.DeploymentRuntimeConfig{}
		if err := r.client.Get(ctx, types.NamespacedName{Name: rcRef.Name}, rc); err != nil {
			return nil, errors.Wrap(err, errGetRuntimeConfig)
		}
		opts = append(opts, BuilderWithRuntimeConfig(rc))
	}

	sa := &corev1.ServiceAccount{}
	// Fetch XP ServiceAccount to get the ImagePullSecrets defined there.
	// We will append them to the list of ImagePullSecrets for the runtime
	// ServiceAccount.
	if err := r.client.Get(ctx, types.NamespacedName{Namespace: r.namespace, Name: r.serviceAccount}, sa); err != nil {
		return nil, errors.Wrap(err, errGetServiceAccount)
	}
	if len(sa.ImagePullSecrets) > 0 {
		opts = append(opts, BuilderWithServiceAccountPullSecrets(sa.ImagePullSecrets))
	}

	return opts, nil
}
