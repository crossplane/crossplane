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
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/xpkg"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"
)

const (
	reconcileTimeout = 3 * time.Minute
)

const (
	errGetPackageRevision = "cannot get package revision"
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
		r.newPackageRevision = f
	}
}

// WithConfigStore specifies the ConfigStore to use for fetching image
// configurations.
func WithConfigStore(c xpkg.ConfigStore) ReconcilerOption {
	return func(r *Reconciler) {
		r.config = c
	}
}

// Reconciler reconciles package revision.
type Reconciler struct {
	client client.Client
	config xpkg.ConfigStore
	log    logging.Logger

	newPackageRevision func() v1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, o controller.Options) error {
	name := "package-signature-verification/" + strings.ToLower(v1.ProviderRevisionGroupKind)
	nr := func() v1.PackageRevision { return &v1.ProviderRevision{} }

	log := o.Logger.WithValues("controller", name)
	cb := ctrl.NewControllerManagedBy(mgr).
		Named(name).
		// TODO: Watch for changes in ImageConfig and trigger reconciliation of ProviderRevisions
		For(&v1.ProviderRevision{})

	ro := []ReconcilerOption{
		WithNewPackageRevisionFn(nr),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient())),
		WithLogger(log),
	}

	return cb.WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, ro...)), o.GlobalRateLimiter))
}

// TODO: Setup ConfigurationRevision controller
// TODO: Setup FunctionRevision controller

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),
		log:    logging.NewNopLogger(),
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
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		log.Debug(errGetPackageRevision, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackageRevision)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	log.Info("Reconciled")

	return reconcile.Result{}, nil
}
