/*
Copyright 2024 The Crossplane Authors.

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

// Package signature implements the controller verifying package signatures.
package signature

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	reconcileTimeout = 3 * time.Minute
)

const (
	errGetRevision           = "cannot get package revision"
	errParseReference        = "cannot parse package image reference"
	errNewKubernetesClient   = "cannot create new Kubernetes clientset"
	errGetVerificationConfig = "cannot get image verification config"
	errGetConfigPullSecret   = "cannot get image config pull secret for image"
	errFailedVerification    = "signature verification failed"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newRevision = f
	}
}

// WithConfigStore specifies the ConfigStore to use for fetching image
// configurations.
func WithConfigStore(c xpkg.ConfigStore) ReconcilerOption {
	return func(r *Reconciler) {
		r.config = c
	}
}

// WithNamespace specifies the namespace in which the Reconciler should create
// runtime resources.
func WithNamespace(n string) ReconcilerOption {
	return func(r *Reconciler) {
		r.namespace = n
	}
}

// WithDefaultRegistry specifies the registry to use for fetching images.
func WithDefaultRegistry(registry string) ReconcilerOption {
	return func(r *Reconciler) {
		r.registry = registry
	}
}

// WithServiceAccount specifies the service account to use for fetching images.
func WithServiceAccount(sa string) ReconcilerOption {
	return func(r *Reconciler) {
		r.serviceAccount = sa
	}
}

// WithValidator specifies the Validator to use for verifying signatures.
func WithValidator(v Validator) ReconcilerOption {
	return func(r *Reconciler) {
		r.validator = v
	}
}

// Reconciler reconciles package for signature verification.
type Reconciler struct {
	client         client.Client
	config         xpkg.ConfigStore
	validator      Validator
	log            logging.Logger
	serviceAccount string
	namespace      string
	registry       string

	newRevision func() v1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.ProviderRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), clientset, o.Namespace, o.ServiceAccount)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.ProviderRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueuePackageRevisionsForImageConfig(mgr.GetClient(), log, &v1.ProviderRevisionList{}))

	ro := []ReconcilerOption{
		WithNewPackageRevisionFn(np),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithDefaultRegistry(o.DefaultRegistry),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithValidator(cosignValidator),
		WithLogger(log),
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), ro...)), o.GlobalRateLimiter))
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.ConfigurationRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.ConfigurationRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), clientset, o.Namespace, o.ServiceAccount)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.ConfigurationRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueuePackageRevisionsForImageConfig(mgr.GetClient(), log, &v1.ConfigurationRevisionList{}))

	ro := []ReconcilerOption{
		WithNewPackageRevisionFn(np),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithDefaultRegistry(o.DefaultRegistry),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithValidator(cosignValidator),
		WithLogger(log),
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), ro...)), o.GlobalRateLimiter))
}

// SetupFunctionRevision adds a controller that reconciles FunctionRevisions.
func SetupFunctionRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.FunctionRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.FunctionRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), clientset, o.Namespace, o.ServiceAccount)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.FunctionRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueuePackageRevisionsForImageConfig(mgr.GetClient(), log, &v1.FunctionRevisionList{}))

	ro := []ReconcilerOption{
		WithNewPackageRevisionFn(np),
		WithNamespace(o.Namespace),
		WithServiceAccount(o.ServiceAccount),
		WithDefaultRegistry(o.DefaultRegistry),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithValidator(cosignValidator),
		WithLogger(log),
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), ro...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package reconciler for signature verification.
func NewReconciler(client client.Client, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: client,
		log:    logging.NewNopLogger(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile packages and verify signatures if configured.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	pr := r.newRevision()
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		log.Debug(errGetRevision, "error", err)
		pr.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errGetRevision)))
		_ = r.client.Status().Update(ctx, pr)
		return reconcile.Result{}, errors.Wrap(err, errGetRevision)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Only verify signatures for active revisions.
	if pr.GetDesiredState() != v1.PackageRevisionActive {
		log.Debug("Skipping signature verification for inactive package revision")
		return reconcile.Result{}, nil
	}

	// If signature verification is already complete, nothing to do here.
	// A package is deployed once signature verification is complete which means
	// either the verification skipped or succeeded. Once we have this condition,
	// it doesn't make sense to verify the signature again since the package is
	// already deployed.
	if cond := pr.GetCondition(v1.TypeVerified); cond.Status == corev1.ConditionTrue {
		return reconcile.Result{}, nil
	}

	ic, vc, err := r.config.ImageVerificationConfigFor(ctx, pr.GetSource())
	if err != nil {
		log.Debug("Cannot get image verification config", "error", err)
		pr.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errGetVerificationConfig)))
		_ = r.client.Status().Update(ctx, pr)
		return reconcile.Result{}, errors.Wrap(err, errGetVerificationConfig)
	}
	if vc == nil || vc.Cosign == nil {
		// No verification config found for this image, so, we will skip
		// verification.
		log.Debug("No signature verification config found for image, skipping verification")
		pr.SetConditions(v1.VerificationSkipped())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), "cannot update package status")
	}

	ref, err := name.ParseReference(pr.GetSource(), name.WithDefaultRegistry(r.registry))
	if err != nil {
		log.Debug("Cannot parse package image reference", "error", err)
		pr.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errParseReference)))
		_ = r.client.Status().Update(ctx, pr)
		return reconcile.Result{}, errors.Wrap(err, errParseReference)
	}

	pullSecrets := make([]string, 0, 2)
	for _, s := range pr.GetPackagePullSecrets() {
		pullSecrets = append(pullSecrets, s.Name)
	}

	_, s, err := r.config.PullSecretFor(ctx, pr.GetSource())
	if err != nil {
		log.Debug("Cannot get image config pull secret for image", "error", err)
		pr.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errGetConfigPullSecret)))
		_ = r.client.Status().Update(ctx, pr)
		return reconcile.Result{}, errors.Wrap(err, errGetConfigPullSecret)
	}
	if s != "" {
		pullSecrets = append(pullSecrets, s)
	}

	if err = r.validator.Validate(ctx, ref, vc, pullSecrets...); err != nil {
		log.Debug("Signature verification failed", "error", err)
		pr.SetConditions(v1.VerificationFailed(ic, err))
		if sErr := r.client.Status().Update(ctx, pr); sErr != nil {
			return reconcile.Result{}, errors.Wrap(sErr, "cannot update status with failed verification")
		}
		return reconcile.Result{}, errors.Wrap(err, errFailedVerification)
	}

	pr.SetConditions(v1.VerificationSucceeded(ic))
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, pr), "cannot update status with successful verification")
}

func enqueuePackageRevisionsForImageConfig(kube client.Client, log logging.Logger, list v1.PackageRevisionList) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			return nil
		}
		// We only care about ImageConfigs with Cosign verification configured.
		if ic.Spec.Verification == nil {
			return nil
		}
		// Enqueue all ProviderRevisions matching the prefixes in the ImageConfig.
		l := list.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Guaranteed to be this type.
		if err := kube.List(ctx, l); err != nil {
			// Nothing we can do, except logging, if we can't list ProviderRevisions.
			log.Debug("Cannot list provider revisions while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, p := range l.GetRevisions() {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(p.GetSource(), m.Prefix) {
					log.Debug("Enqueuing provider revisions for image config", "provider-revision", p.GetName(), "imageConfig", ic.Name)
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: p.GetName()}})
				}
			}
		}
		return matches
	})
}
