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
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/dag"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	reconcileTimeout = 1 * time.Minute

	packageTagFmt    = "%s:%s"
	packageDigestFmt = "%s@%s"
)

const (
	lockName  = "lock"
	finalizer = "lock.pkg.crossplane.io"

	errGetLock              = "cannot get package lock"
	errAddFinalizer         = "cannot add lock finalizer"
	errRemoveFinalizer      = "cannot remove lock finalizer"
	errBuildDAG             = "cannot build DAG"
	errSortDAG              = "cannot sort DAG"
	errFmtMissingDependency = "missing package (%s) is not a dependency"
	errInvalidConstraint    = "version constraint on dependency is invalid"
	errDowngradeNotAllowed  = "version downgrade is required to satisfy constraints, manual intervention required"
	errInvalidDependency    = "dependency package is not valid"
	errGetPullConfig        = "cannot get image pull secret from config"
	errFetchTags            = "cannot fetch dependency package tags"
	errNoValidVersion       = "cannot find a valid version for package constraints"
	errFmtNoValidVersion    = "dependency (%s) does not have version in constraints (%s)"
	errInvalidPackageType   = "cannot create invalid package dependency type"
	errGetDependency        = "cannot get dependency package"
	errCreateDependency     = "cannot create dependency package"
	errUpdateDependency     = "cannot update dependency package"
)

// Event reasons.
const (
	reasonErrBuildDAG       event.Reason = "BuildDAGError"
	reasonCyclicDependency  event.Reason = "CyclicDependency"
	reasonInvalidDependency event.Reason = "InvalidDependency"
	reasonNoValidVersion    event.Reason = "NoValidVersion"
	reasonErrCreate         event.Reason = "CreateError"
	reasonErrUpdate         event.Reason = "UpdateError"
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

// WithDefaultRegistry sets the default registry to use.
func WithDefaultRegistry(registry string) ReconcilerOption {
	return func(r *Reconciler) {
		r.registry = registry
	}
}

// WithVersionFinder sets the version finder to use.
func WithVersionFinder(vf versionFinder) ReconcilerOption {
	return func(r *Reconciler) {
		r.versionFinder = vf
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
	client        client.Client
	log           logging.Logger
	lock          resource.Finalizer
	newDag        dag.NewDAGFn
	registry      string
	versionFinder versionFinder
	record        event.Recorder
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
	cfg := xpkg.NewImageConfigStore(mgr.GetClient())

	opts := []ReconcilerOption{
		WithLogger(o.Logger.WithValues("controller", name)),
		WithDefaultRegistry(o.DefaultRegistry),
		WithVersionFinder(&DefaultVersionFinder{fetcher: f, config: cfg}),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(features.EnableAlphaDependencyVersionUpdate) {
		opts = append(opts, WithNewDagFn(dag.NewUpdatableMapDag),
			WithVersionFinder(&UpdatableVersionFinder{fetcher: f, config: cfg}))
	}

	r := NewReconciler(mgr, opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Lock{}).
		Owns(&v1.ConfigurationRevision{}).
		Owns(&v1.ProviderRevision{}).
		Watches(&v1beta1.ImageConfig{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
			ic, ok := o.(*v1beta1.ImageConfig)
			if !ok {
				return nil
			}
			// We only care about ImageConfigs that have a pull secret.
			if ic.Spec.Registry == nil || ic.Spec.Registry.Authentication == nil || ic.Spec.Registry.Authentication.PullSecretRef.Name == "" {
				return nil
			}
			// Ideally we should enqueue only if the ImageConfig applies to a
			// package in the Lock which would require getting/parsing the Lock
			// and checking the source of each package against the prefixes in
			// the ImageConfig. However, this is a bit more complex than needed,
			// and we don't expect to have many ImageConfigs so we just enqueue
			// for all ImageConfigs with a pull secret.
			return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: lockName}}}
		})).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: mgr.GetClient(),
		lock:   resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		log:    logging.NewNopLogger(),
		newDag: dag.NewMapDag,
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
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
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			return reconcile.Result{}, errors.Wrap(err, errRemoveFinalizer)
		}
		return reconcile.Result{}, nil
	}

	if err := r.lock.AddFinalizer(ctx, lock); err != nil {
		log.Debug(errAddFinalizer, "error", err)
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, errors.Wrap(err, errAddFinalizer)
	}

	log = log.WithValues(
		"uid", lock.GetUID(),
		"version", lock.GetResourceVersion(),
		"name", lock.GetName(),
	)

	ndag := r.newDag()
	implied, err := ndag.Init(v1beta1.ToNodes(lock.Packages...))
	if err != nil {
		log.Debug(errBuildDAG, "error", err)
		r.record.Event(lock, event.Warning(reasonErrBuildDAG, err))
		return reconcile.Result{}, errors.Wrap(err, errBuildDAG)
	}

	// Make sure we don't have any cyclical imports. If we do, refuse to
	// install additional packages.
	_, err = ndag.Sort()
	if err != nil {
		log.Debug(errSortDAG, "error", err)
		r.record.Event(lock, event.Warning(reasonCyclicDependency, err))
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
		r.record.Event(lock, event.Warning(reasonInvalidDependency, err))
		return reconcile.Result{Requeue: false}, nil
	}

	n, _ := ndag.GetNode(dep.Identifier()) // nolint: errcheck // we know the node exists since it was implied

	ref, err := name.ParseReference(dep.Package, name.WithDefaultRegistry(r.registry))
	if err != nil {
		log.Debug(errInvalidDependency, "error", err)
		r.record.Event(lock, event.Warning(reasonInvalidDependency, err))
		return reconcile.Result{Requeue: false}, nil
	}

	// NOTE(hasheddan): consider creating event on package revision
	// dictating constraints.
	addVer, err := r.versionFinder.FindValidDependencyVersion(ctx, dep, ref, n, log)
	if err != nil || addVer == "" {
		log.Debug(errNoValidVersion, "error", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.Constraints))
		r.record.Event(lock, event.Warning(reasonNoValidVersion, err))
		return reconcile.Result{Requeue: false}, errors.New(errNoValidVersion)
	}

	var pack v1.Package
	switch dep.Type {
	case v1beta1.ConfigurationPackageType:
		pack = &v1.Configuration{}
	case v1beta1.ProviderPackageType:
		pack = &v1.Provider{}
	case v1beta1.FunctionPackageType:
		pack = &v1.Function{}
	default:
		log.Debug(errInvalidPackageType)
		return reconcile.Result{Requeue: false}, nil
	}

	// NOTE(hasheddan): packages are currently created with default
	// settings. This means that a dependency must be publicly available as
	// no packagePullSecrets are set. Settings can be modified manually
	// after dependency creation to address this.
	pack.SetName(xpkg.ToDNSLabel(ref.Context().RepositoryStr()))

	format := packageTagFmt
	if strings.HasPrefix(addVer, "sha256:") {
		format = packageDigestFmt
	}

	tarSource := fmt.Sprintf(format, ref.String(), addVer)
	pack.SetSource(tarSource)

	if err := r.client.Get(ctx, client.ObjectKeyFromObject(pack), pack); err != nil {
		if !kerrors.IsNotFound(err) {
			log.Debug(errGetDependency, "error", err)
			r.record.Event(lock, event.Warning(reasonErrCreate, err))
			return reconcile.Result{}, errors.Wrap(err, errGetDependency)
		}

		// If the package does not exist, we create it.
		if err := r.client.Create(ctx, pack); err != nil {
			log.Debug(errCreateDependency, "error", err)
			r.record.Event(lock, event.Warning(reasonErrCreate, err))
			return reconcile.Result{}, errors.Wrap(err, errCreateDependency)
		}
	}

	// If the package exists and has a different version than the target we update it.
	if pack.GetSource() != tarSource {
		pack.SetSource(tarSource)
		if err := r.client.Update(ctx, pack); err != nil {
			r.record.Event(lock, event.Warning(reasonErrUpdate, err))
			return reconcile.Result{Requeue: true}, errors.Wrap(err, errUpdateDependency)
		}
	}

	return reconcile.Result{}, nil
}

type versionFinder interface {
	FindValidDependencyVersion(ctx context.Context, dep *v1beta1.Dependency, ref name.Reference, node dag.Node, log logging.Logger) (string, error)
}

// DefaultVersionFinder is the default implementation of versionFinder.
type DefaultVersionFinder struct {
	fetcher xpkg.Fetcher
	config  xpkg.ConfigStore
}

// FindValidDependencyVersion finds a valid version for the new dependency.
func (d *DefaultVersionFinder) FindValidDependencyVersion(ctx context.Context, dep *v1beta1.Dependency, ref name.Reference, _ dag.Node, log logging.Logger) (string, error) {
	var addVer string

	if digest, err := conregv1.NewHash(dep.Constraints); err == nil {
		log.Debug("package is pinned to a specific digest, skipping resolution")
		return digest.String(), nil
	}

	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		log.Debug(errInvalidConstraint, "error", err)
		return "", errors.New(errInvalidConstraint)
	}

	ic, ps, err := d.config.PullSecretFor(ctx, ref.String())
	if err != nil {
		log.Info("cannot get pull secret from image config store", "error", err)
		return "", errors.Wrap(err, errGetPullConfig)
	}

	var s []string
	if ps != "" {
		log.Debug("Selected pull secret from image config store", "image", ref.String(), "imageConfig", ic, "pullSecret", ps)
		s = append(s, ps)
	}
	// NOTE(hasheddan): we will be unable to fetch tags for private
	// dependencies because we do not attach any secrets. Consider copying
	// secrets from parent dependencies.
	tags, err := d.fetcher.Tags(ctx, ref, s...)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return "", errors.New(errFetchTags)
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
	for _, v := range vs {
		if c.Check(v) {
			addVer = v.Original()
		}
	}

	if addVer == "" {
		log.Debug(errNoValidVersion, "error", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.Constraints))
		return "", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.Constraints)
	}

	return addVer, nil
}

// UpdatableVersionFinder is an implementation of versionFinder that allows for version upgrade/downgrade capability considering parent constraints.
type UpdatableVersionFinder struct {
	fetcher xpkg.Fetcher
	config  xpkg.ConfigStore
}

// FindValidDependencyVersion finds a valid version with version upgrade/downgrade capability considering parent constraints.
func (u *UpdatableVersionFinder) FindValidDependencyVersion(ctx context.Context, dep *v1beta1.Dependency, ref name.Reference, n dag.Node, log logging.Logger) (string, error) {
	if digest, err := conregv1.NewHash(dep.Constraints); err == nil {
		log.Debug("package is pinned to a specific digest, skipping resolution")
		return digest.String(), nil
	}

	ic, ps, err := u.config.PullSecretFor(ctx, ref.String())
	if err != nil {
		log.Info("cannot get pull secret from image config store", "error", err)
		return "", errors.Wrap(err, errGetPullConfig)
	}

	var s []string
	if ps != "" {
		log.Debug("Selected pull secret from image config store", "image", ref.String(), "imageConfig", ic, "pullSecret", ps)
		s = append(s, ps)
	}
	// NOTE(hasheddan): we will be unable to fetch tags for private
	// dependencies because we do not attach any secrets. Consider copying
	// secrets from parent dependencies.
	tags, err := u.fetcher.Tags(ctx, ref, s...)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return "", errors.New(errFetchTags)
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

	var cv *semver.Version

	// If the node is a LockPackage, it means it's already installed.
	if lp, ok := n.(*v1beta1.LockPackage); ok {
		log.Debug("dependency found in lock, trying to find the minimum valid version which is greater than the current version")
		sort.Sort(semver.Collection(vs))
		cv = semver.MustParse(lp.Version)
	}

	// If the node is not a LockPackage, it means it's a new dependency that we need to install with the latest version.
	if cv == nil {
		log.Debug("dependency not found in lock, trying to find the maximum valid version")
		sort.Sort(sort.Reverse(semver.Collection(vs)))
	}

	var addVer string
	for _, v := range vs {
		valid := true
		for _, constraint := range n.GetParentConstraints() {
			c, err := semver.NewConstraint(constraint)
			if err != nil {
				log.Debug(errInvalidConstraint, "error", err)
				return "", errors.New(errInvalidConstraint)
			}

			if !c.Check(v) {
				valid = false
				break
			}

			// NOTE(ezgidemirel): If there is no valid version greater than the current version,
			// we should not allow downgrade.
			// Compare compares this version to another one. It returns -1, 0, or 1 if
			// the version smaller, equal, or larger than the other version.
			if cv != nil && v.Compare(cv) == -1 {
				log.Debug(errDowngradeNotAllowed, "currentVersion", cv.String(), "newVersion", v.String())
				return "", errors.New(errDowngradeNotAllowed)
			}
		}

		if valid {
			addVer = v.Original()
			break
		}
	}

	if addVer == "" {
		log.Debug(errNoValidVersion, "error", errors.Errorf(errFmtNoValidVersion, n.Identifier(), n.GetParentConstraints()))
		return "", errors.Errorf(errFmtNoValidVersion, n.Identifier(), n.GetParentConstraints())
	}

	return addVer, nil
}
