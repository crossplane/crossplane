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

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
	errGetPackage          = "cannot get package"
	errParseReference      = "cannot parse package image reference"
	errNewKubernetesClient = "cannot create new Kubernetes clientset"
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
		r.newPackage = f
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
	clientset      kubernetes.Interface
	config         xpkg.ConfigStore
	validator      Validator
	log            logging.Logger
	serviceAccount string
	namespace      string
	registry       string

	newPackage func() v1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.ProviderRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), o.Namespace)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.ProviderRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueueProviderRevisionsForImageConfig(mgr.GetClient(), log))

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
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), clientset, ro...)), o.GlobalRateLimiter))
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.ConfigurationRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.ConfigurationRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), o.Namespace)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.ConfigurationRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueueConfigurationRevisionsForImageConfig(mgr.GetClient(), log))

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
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), clientset, ro...)), o.GlobalRateLimiter))
}

// SetupFunctionRevision adds a controller that reconciles FunctionRevisions.
func SetupFunctionRevision(mgr ctrl.Manager, o controller.Options) error {
	n := "package-signature-verification/" + strings.ToLower(v1.FunctionRevisionGroupKind)
	np := func() v1.PackageRevision { return &v1.FunctionRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, errNewKubernetesClient)
	}

	cosignValidator, err := NewCosignValidator(mgr.GetClient(), o.Namespace)
	if err != nil {
		return errors.Wrap(err, "cannot create cosign validator")
	}

	log := o.Logger.WithValues("controller", n)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(n).
		For(&v1.FunctionRevision{}).
		Watches(&v1beta1.ImageConfig{}, enqueueFunctionRevisionsForImageConfig(mgr.GetClient(), log))

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
		Complete(ratelimiter.NewReconciler(n, errors.WithSilentRequeueOnConflict(NewReconciler(mgr.GetClient(), clientset, ro...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package reconciler for signature verification.
func NewReconciler(client client.Client, clientset kubernetes.Interface, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:    client,
		clientset: clientset,
		log:       logging.NewNopLogger(),
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

	p := r.newPackage()
	if err := r.client.Get(ctx, req.NamespacedName, p); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		log.Debug(errGetPackage, "error", err)
		p.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errGetPackage)))
		_ = r.client.Status().Update(ctx, p)
		return reconcile.Result{}, errors.Wrap(err, errGetPackage)
	}

	log = log.WithValues(
		"uid", p.GetUID(),
		"version", p.GetResourceVersion(),
		"name", p.GetName(),
	)

	// If signature verification is already complete, nothing to do here.
	// A package is deployed once signature verification is complete which means
	// either the verification skipped or succeeded. Once we have this condition,
	// it doesn't make sense to verify the signature again since the package is
	// already deployed.
	if cond := p.GetCondition(v1.TypeVerified); cond.Status == corev1.ConditionTrue {
		return reconcile.Result{}, nil
	}

	ic, vc, err := r.config.ImageVerificationConfigFor(ctx, p.GetSource())
	if err != nil {
		log.Debug("Cannot get image verification config", "error", err)
		p.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, "cannot get image verification config")))
		_ = r.client.Status().Update(ctx, p)
		return reconcile.Result{}, errors.Wrap(err, "cannot get image verification config")
	}
	if vc == nil || vc.Cosign == nil {
		// No verification config found for this image, so, we will skip
		// verification.
		log.Debug("No signature verification config found for image, skipping verification")
		p.SetConditions(v1.VerificationSkipped())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), "cannot update package status")
	}

	ref, err := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(r.registry))
	if err != nil {
		log.Debug("Cannot parse package image reference", "error", err)
		p.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, errParseReference)))
		_ = r.client.Status().Update(ctx, p)
		return reconcile.Result{}, errors.Wrap(err, errParseReference)
	}

	pullSecrets := make([]string, 0, 2)
	for _, s := range p.GetPackagePullSecrets() {
		pullSecrets = append(pullSecrets, s.Name)
	}

	_, s, err := r.config.PullSecretFor(ctx, p.GetSource())
	if err != nil {
		log.Debug("Cannot get image config pull secret for image", "error", err)
		p.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, "cannot get image config pull secret for image")))
		_ = r.client.Status().Update(ctx, p)
		return reconcile.Result{}, errors.Wrap(err, "cannot get image config pull secret for image")
	}
	if s != "" {
		pullSecrets = append(pullSecrets, s)
	}

	auth, err := k8schain.New(ctx, r.clientset, k8schain.Options{
		Namespace:          r.namespace,
		ServiceAccountName: r.serviceAccount,
		ImagePullSecrets:   pullSecrets,
	})
	if err != nil {
		log.Debug("Cannot create k8s auth chain", "error", err)
		p.SetConditions(v1.VerificationIncomplete(errors.Wrap(err, "cannot create k8s auth chain")))
		_ = r.client.Status().Update(ctx, p)
		return reconcile.Result{}, errors.Wrap(err, "cannot create k8s auth chain")
	}

	if err = r.validator.Validate(ctx, ref, vc, remote.WithAuthFromKeychain(auth)); err != nil {
		log.Debug("Signature verification failed", "error", err)
		p.SetConditions(v1.VerificationFailed(ic, err))
		if err = r.client.Status().Update(ctx, p); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot update status with failed verification")
		}
		return reconcile.Result{}, errors.Wrap(err, "signature verification failed")
	}

	p.SetConditions(v1.VerificationSucceeded(ic))
	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, p), "cannot update status with successful verification")
}

func enqueueProviderRevisionsForImageConfig(kube client.Client, log logging.Logger) handler.EventHandler {
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
		l := &v1.ProviderRevisionList{}
		if err := kube.List(ctx, l); err != nil {
			// Nothing we can do, except logging, if we can't list ProviderRevisions.
			log.Debug("Cannot list provider revisions while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, p := range l.Items {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(p.GetSource(), m.Prefix) {
					log.Debug("Enqueuing provider revisions for image config", "provider-revision", p.Name, "imageConfig", ic.Name)
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: p.Name}})
				}
			}
		}
		return matches
	})
}

func enqueueConfigurationRevisionsForImageConfig(kube client.Client, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			return nil
		}
		// We only care about ImageConfigs with Cosign verification configured.
		if ic.Spec.Verification == nil {
			return nil
		}
		// Enqueue all ConfigurationRevisions matching the prefixes in the ImageConfig.
		l := &v1.ConfigurationRevisionList{}
		if err := kube.List(ctx, l); err != nil {
			// Nothing we can do, except logging, if we can't list ConfigurationRevisions.
			log.Debug("Cannot list configuration revisions while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, c := range l.Items {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(c.GetSource(), m.Prefix) {
					log.Debug("Enqueuing configuration revisions for image config", "configuration-revision", c.Name, "imageConfig", ic.Name)
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: c.Name}})
				}
			}
		}
		return matches
	})
}

func enqueueFunctionRevisionsForImageConfig(kube client.Client, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			return nil
		}
		// We only care about ImageConfigs with Cosign verification configured.
		if ic.Spec.Verification == nil {
			return nil
		}
		// Enqueue all FunctionRevisions matching the prefixes in the ImageConfig.
		l := &v1.FunctionRevisionList{}
		if err := kube.List(ctx, l); err != nil {
			// Nothing we can do, except logging, if we can't list FunctionRevisions.
			log.Debug("Cannot list function revisions while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, fn := range l.Items {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(fn.GetSource(), m.Prefix) {
					log.Debug("Enqueuing function revisions for image config", "function-revision", fn.Name, "imageConfig", ic.Name)
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: fn.Name}})
				}
			}
		}
		return matches
	})
}
