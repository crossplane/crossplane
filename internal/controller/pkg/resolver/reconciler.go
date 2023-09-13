/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing perimpliedions and
limitations under the License.
*/

// Package resolver implements the Crossplane Package Lock controller.
package resolver

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/dag"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	reconcileTimeout = 1 * time.Minute

	packageTagFmt = "%s:%s"
)

const (
	finalizer = "lock.pkg.crossplane.io"

	errGetLock              = "cannot get package lock"
	errAddFinalizer         = "cannot add lock finalizer"
	errRemoveFinalizer      = "cannot remove lock finalizer"
	errBuildDAG             = "cannot build DAG"
	errSortDAG              = "cannot sort DAG"
	errFmtMissingDependency = "missing package (%s) is not a dependency"
	errInvalidConstraint    = "version constraint on dependency is invalid"
	errInvalidDependency    = "dependency package is not valid"
	errFetchTags            = "cannot fetch dependency package tags"
	errNoValidVersion       = "cannot find a valid version for package constraints"
	errFmtNoValidVersion    = "dependency (%s) does not have version in constraints (%s)"
	errInvalidPackageType   = "cannot create invalid package dependency type"
	errCreateDependency     = "cannot create dependency package"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithFinalizer specifies how the Reconciler should finalize package revisions.
func WithFinalizer(f resource.Finalizer) ReconcilerOption {
	return func(r *Reconciler) {
		r.lock = f
	}
}

// WithNewDagFn specifies how the Reconciler should build its dependency graph.
func WithNewDagFn(f dag.NewDAGFn) ReconcilerOption {
	return func(r *Reconciler) {
		r.newDag = f
	}
}

// WithFetcher specifies how the Reconciler should fetch package tags.
func WithFetcher(f xpkg.Fetcher) ReconcilerOption {
	return func(r *Reconciler) {
		r.fetcher = f
	}
}

// Reconciler reconciles packages.
type Reconciler struct {
	client  client.Client
	log     logging.Logger
	lock    resource.Finalizer
	newDag  dag.NewDAGFn
	fetcher xpkg.Fetcher
}

// Setup adds a controller that reconciles the Lock.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "packages/" + strings.ToLower(v1beta1.LockGroupKind)

	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize clientset")
	}
	f, err := xpkg.NewK8sFetcher(cs, append(o.FetcherOptions, xpkg.WithNamespace(o.Namespace), xpkg.WithServiceAccount(o.ServiceAccount))...)
	if err != nil {
		return errors.Wrap(err, "cannot build fetcher")
	}

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithFetcher(f),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Lock{}).
		Owns(&v1.ConfigurationRevision{}).
		Owns(&v1.ProviderRevision{}).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:  mgr.GetClient(),
		lock:    resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		log:     logging.NewNopLogger(),
		newDag:  dag.NewMapDag,
		fetcher: xpkg.NewNopFetcher(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo // Reconcilers are complex. Be wary of adding more.
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, reconcileTimeout)
	defer cancel()

	lock := &v1beta1.Lock{}
	if err := r.client.Get(ctx, req.NamespacedName, lock); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise
		// we'll be requeued implicitly because we return an error.
		log.Debug(errGetLock, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetLock)
	}

	// If no packages exist in Lock then we remove finalizer and wait until
	// a package is added to reconcile again. This allows for cleanup of the
	// Lock when uninstalling Crossplane after all packages have already
	// been uninstalled.
	if len(lock.Packages) == 0 {
		if err := r.lock.RemoveFinalizer(ctx, lock); err != nil {
			log.Debug(errRemoveFinalizer, "error", err)
			return reconcile.Result{}, errors.Wrap(err, errRemoveFinalizer)
		}
		return reconcile.Result{}, nil
	}

	if err := r.lock.AddFinalizer(ctx, lock); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		return reconcile.Result{}, errors.Wrap(err, errAddFinalizer)
	}

	log = log.WithValues(
		"uid", lock.GetUID(),
		"version", lock.GetResourceVersion(),
		"name", lock.GetName(),
	)

	dag := r.newDag()
	implied, err := dag.Init(v1beta1.ToNodes(lock.Packages...))
	if err != nil {
		log.Debug(errBuildDAG, "error", err)
		return reconcile.Result{}, errors.Wrap(err, errBuildDAG)
	}

	// Make sure we don't have any cyclical imports. If we do, refuse to
	// install additional packages.
	_, err = dag.Sort()
	if err != nil {
		log.Debug(errSortDAG, "error", err)
		return reconcile.Result{}, errors.Wrap(err, errSortDAG)
	}

	if len(implied) == 0 {
		return reconcile.Result{Requeue: false}, nil
	}

	// If we are missing a node, we want to create it. The resolver never
	// modifies the Lock. We only create the first implied node as we will
	// be requeued when it adds itself to the Lock, at which point we will
	// check for missing nodes again.
	dep, ok := implied[0].(*v1beta1.Dependency)
	if !ok {
		log.Debug(errInvalidDependency, "error", errors.Errorf(errFmtMissingDependency, dep.Identifier()))
		return reconcile.Result{Requeue: false}, nil
	}
	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		log.Debug(errInvalidConstraint, "error", err)
		return reconcile.Result{Requeue: false}, nil
	}
	ref, err := name.ParseReference(dep.Package)
	if err != nil {
		log.Debug(errInvalidDependency, "error", err)
		return reconcile.Result{Requeue: false}, nil
	}

	// NOTE(hasheddan): we will be unable to fetch tags for private
	// dependencies because we do not attach any secrets. Consider copying
	// secrets from parent dependencies.
	tags, err := r.fetcher.Tags(ctx, ref)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return reconcile.Result{}, errors.Wrap(err, errFetchTags)
	}

	vs := []*semver.Version{}
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions.
			continue
		}
		vs = append(vs, v)
	}

	sort.Sort(semver.Collection(vs))
	var addVer string
	for _, v := range vs {
		if c.Check(v) {
			addVer = v.Original()
		}
	}

	// NOTE(hasheddan): consider creating event on package revision
	// dictating constraints.
	if addVer == "" {
		log.Debug(errNoValidVersion, "error", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.Constraints))
		return reconcile.Result{Requeue: false}, nil
	}

	var pack v1.Package
	switch dep.Type {
	case v1beta1.ConfigurationPackageType:
		pack = &v1.Configuration{}
	case v1beta1.ProviderPackageType:
		pack = &v1.Provider{}
	case v1beta1.FunctionPackageType:
		pack = &v1beta1.Function{}
	default:
		log.Debug(errInvalidPackageType)
		return reconcile.Result{Requeue: false}, nil
	}

	// NOTE(hasheddan): packages are currently created with default
	// settings. This means that a dependency must be publicly available as
	// no packagePullSecrets are set. Settings can be modified manually
	// after dependency creation to address this.
	pack.SetName(xpkg.ToDNSLabel(ref.Context().RepositoryStr()))
	pack.SetSource(fmt.Sprintf(packageTagFmt, ref.String(), addVer))

	// NOTE(hasheddan): consider making the lock the controller of packages
	// it creates.
	if err := r.client.Create(ctx, pack); err != nil {
		log.Debug(errCreateDependency, "error", err)
		return reconcile.Result{}, errors.Wrap(err, errCreateDependency)
	}

	return reconcile.Result{Requeue: false}, nil
}
