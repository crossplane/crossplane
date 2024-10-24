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
	internaldag "github.com/crossplane/crossplane/internal/dag"
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

	errGetLock                = "cannot get package lock"
	errAddFinalizer           = "cannot add lock finalizer"
	errRemoveFinalizer        = "cannot remove lock finalizer"
	errBuildDAG               = "cannot build DAG"
	errSortDAG                = "cannot sort DAG"
	errFmtMissingDependency   = "missing package (%s) is not a dependency"
	errInvalidConstraint      = "version constraint on dependency is invalid"
	errInvalidDependency      = "dependency package is not valid"
	errFindDependency         = "cannot find dependency version to install"
	errGetPullConfig          = "cannot get image pull secret from config"
	errFetchTags              = "cannot fetch dependency package tags"
	errFindDependencyUpgrade  = "cannot find dependency version to upgrade"
	errFmtNoValidVersion      = "dependency (%s) does not have a valid version to upgrade that satisfies all constraints. If there is a valid version that requires downgrade, manual intervention is required. Constraints: %v"
	errInvalidPackageType     = "cannot create invalid package dependency type"
	errGetDependency          = "cannot get dependency package"
	errCreateDependency       = "cannot create dependency package"
	errUpdateDependency       = "cannot update dependency package"
	errFmtSplit               = "package should have 2 segments after split but has %d"
	errFmtDiffConstraintTypes = "a dependency package has different types of parent constraints (%v)"
	errFmtDiffDigests         = "a dependency package has different digests in parent constraints (%v)"
)

// Event reasons.
const (
	reasonErrBuildDAG       event.Reason = "BuildDAGError"
	reasonCyclicDependency  event.Reason = "CyclicDependency"
	reasonInvalidDependency event.Reason = "InvalidDependency"
	reasonNoValidVersion    event.Reason = "NoValidVersion"
	reasonErrCreate         event.Reason = "CreateError"
	reasonErrGet            event.Reason = "GetError"
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
func WithNewDagFn(f internaldag.NewDAGFn) ReconcilerOption {
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

// WithConfigStore specifies how the Reconciler should access image config store.
func WithConfigStore(c xpkg.ConfigStore) ReconcilerOption {
	return func(r *Reconciler) {
		r.config = c
	}
}

// WithDefaultRegistry sets the default registry to use.
func WithDefaultRegistry(registry string) ReconcilerOption {
	return func(r *Reconciler) {
		r.registry = registry
	}
}

// WithUpgradesEnabled sets whether upgrades are enabled or not.
func WithUpgradesEnabled() ReconcilerOption {
	return func(r *Reconciler) {
		r.upgradesEnabled = true
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
	client   client.Client
	log      logging.Logger
	lock     resource.Finalizer
	newDag   internaldag.NewDAGFn
	fetcher  xpkg.Fetcher
	config   xpkg.ConfigStore
	registry string
	record   event.Recorder

	upgradesEnabled bool
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
	opts := []ReconcilerOption{
		WithLogger(o.Logger.WithValues("controller", name)),
		WithFetcher(f),
		WithDefaultRegistry(o.DefaultRegistry),
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient())),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	}

	if o.Features.Enabled(features.EnableAlphaDependencyVersionUpgrades) {
		opts = append(opts, WithUpgradesEnabled(), WithNewDagFn(internaldag.NewUpgradingMapDag))
	}

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
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, opts...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new package revision reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:  mgr.GetClient(),
		lock:    resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		log:     logging.NewNopLogger(),
		newDag:  internaldag.NewMapDag,
		fetcher: xpkg.NewNopFetcher(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // we need to handle both pkg installation and upgrade scenarios now
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

	dag := r.newDag()
	implied, err := dag.Init(v1beta1.ToNodes(lock.Packages...))
	if err != nil {
		log.Debug(errBuildDAG, "error", err)
		r.record.Event(lock, event.Warning(reasonErrBuildDAG, err))
		return reconcile.Result{}, errors.Wrap(err, errBuildDAG)
	}

	// Make sure we don't have any cyclical imports. If we do, refuse to
	// install additional packages.
	_, err = dag.Sort()
	if err != nil {
		log.Debug(errSortDAG, "error", err)
		r.record.Event(lock, event.Warning(reasonCyclicDependency, err))
		return reconcile.Result{}, errors.Wrap(err, errSortDAG)
	}

	if len(implied) == 0 {
		return reconcile.Result{}, nil
	}

	// If we are missing a node, we want to create it. The resolver never
	// modifies the Lock. We only create the first implied node as we will
	// be requeued when it adds itself to the Lock, at which point we will
	// check for missing nodes again.
	dep, ok := implied[0].(*v1beta1.Dependency)
	depID := dep.Identifier()
	if !ok {
		log.Debug(errInvalidDependency, "error", errors.Errorf(errFmtMissingDependency, depID))
		r.record.Event(lock, event.Warning(reasonInvalidDependency, err))
		return reconcile.Result{}, nil
	}

	ref, err := name.ParseReference(depID, name.WithDefaultRegistry(r.registry))
	if err != nil {
		log.Debug(errInvalidDependency, "error", err)
		r.record.Event(lock, event.Warning(reasonInvalidDependency, err))
		return reconcile.Result{}, nil
	}

	var pkg v1.Package
	if r.upgradesEnabled {
		pkg, err = r.getPackageWithID(ctx, depID, dep.Type)
		if err != nil {
			log.Debug("cannot get package", "error", err)
			r.record.Event(lock, event.Warning(reasonErrGet, err))
			return reconcile.Result{}, errors.Wrap(err, errGetDependency)
		}
	}

	if pkg == nil {
		// At this point, we know that the dependency is either missing or does not satisfy the constraints.
		// Package does not exist. We need to create it.
		var addVer string
		if addVer, err = r.findDependencyVersionToInstall(ctx, dep, log, ref); err != nil {
			log.Debug(errFindDependency, "error", errors.Wrapf(err, depID, dep.Constraints))
			r.record.Event(lock, event.Warning(reasonNoValidVersion, err))
			return reconcile.Result{Requeue: false}, errors.Wrap(err, errFindDependency)
		}

		// NOTE(hasheddan): consider creating event on package revision
		// dictating constraints.
		if addVer == "" {
			log.Debug(errFindDependencyUpgrade, "error", errors.Errorf(errFmtNoValidVersion, depID, dep.Constraints))
			r.record.Event(lock, event.Warning(reasonNoValidVersion, errors.Errorf(errFmtNoValidVersion, depID, dep.Constraints)))
			return reconcile.Result{Requeue: false}, nil
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
			return reconcile.Result{}, nil
		}

		pack.SetName(xpkg.ToDNSLabel(ref.Context().RepositoryStr()))

		format := packageTagFmt
		if strings.HasPrefix(addVer, "sha256:") {
			format = packageDigestFmt
		}

		pack.SetSource(fmt.Sprintf(format, ref.String(), addVer))

		// NOTE(hasheddan): consider making the lock the controller of packages
		// it creates.
		if err := r.client.Create(ctx, pack); err != nil && !kerrors.IsAlreadyExists(err) {
			log.Debug(errCreateDependency, "error", err)
			r.record.Event(lock, event.Warning(reasonErrCreate, err))
			return reconcile.Result{}, errors.Wrap(err, errCreateDependency)
		}

		return reconcile.Result{}, nil
	}

	if !r.upgradesEnabled {
		return reconcile.Result{}, nil
	}

	// The package is installed, but does not satisfy the constraints and upgrade flag is enabled.
	// We need to search for a new version that satisfies all the constraints.
	_, insVer, err := splitPackage(pkg.GetSource())
	if err != nil {
		log.Debug("cannot split package source", "error", err)
		return reconcile.Result{}, nil
	}

	n, err := dag.GetNode(depID)
	if err != nil {
		log.Debug(errInvalidDependency, "error", errors.Errorf(errFmtMissingDependency, depID))
		r.record.Event(lock, event.Warning(reasonInvalidDependency, err))
		return reconcile.Result{}, nil
	}

	newVer, err := r.findDependencyVersionToUpgrade(ctx, ref, insVer, n, log)
	if err != nil {
		log.Debug(errFindDependencyUpgrade, "error", errors.Wrapf(err, depID, dep.Constraints))
		r.record.Event(lock, event.Warning(reasonNoValidVersion, err))
		return reconcile.Result{}, errors.Wrap(err, errFindDependencyUpgrade)
	}

	// Update the package with the new version.
	format := packageTagFmt
	if strings.HasPrefix(newVer, "sha256:") {
		format = packageDigestFmt
	}

	pkg.SetSource(fmt.Sprintf(format, ref.String(), newVer))
	if err := r.client.Update(ctx, pkg); err != nil {
		log.Debug(errUpdateDependency, "error", err)
		r.record.Event(lock, event.Warning(reasonErrUpdate, err))
		return reconcile.Result{}, errors.Wrap(err, errUpdateDependency)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) findDependencyVersionToInstall(ctx context.Context, dep *v1beta1.Dependency, log logging.Logger, ref name.Reference) (string, error) {
	var addVer string

	if digest, err := conregv1.NewHash(dep.Constraints); err == nil {
		log.Debug("package is pinned to a specific digest, skipping resolution")
		return digest.String(), nil
	}

	c, err := semver.NewConstraint(dep.Constraints)
	if err != nil {
		log.Debug(errInvalidConstraint, "error", err)
		return "", errors.Wrap(err, errInvalidConstraint)
	}

	ic, ps, err := r.config.PullSecretFor(ctx, ref.String())
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
	tags, err := r.fetcher.Tags(ctx, ref, s...)
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

	return addVer, nil
}

// FindValidDependencyVersion finds a valid version with version upgrade capability considering parent constraints.
func (r *Reconciler) findDependencyVersionToUpgrade(ctx context.Context, ref name.Reference, insVer string, dep internaldag.Node, log logging.Logger) (string, error) {
	// If there is a digest in the parent constraints, we need to make sure that all other parent constraints are the same.
	digest, err := findDigestToUpdate(dep)
	if err != nil {
		log.Debug("cannot find digest to update", "error", err)
		return "", err
	}

	if digest != "" {
		log.Debug("package is pinned to a specific digest, skipping resolution")
		return digest, nil
	}

	ic, ps, err := r.config.PullSecretFor(ctx, ref.String())
	if err != nil {
		log.Info("cannot get pull secret from image config store", "error", err)
		return "", errors.Wrap(err, errGetPullConfig)
	}

	var s []string
	if ps != "" {
		log.Debug("Selected pull secret from image config store", "image", ref.String(), "imageConfig", ic, "pullSecret", ps)
		s = append(s, ps)
	}

	tags, err := r.fetcher.Tags(ctx, ref, s...)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return "", errors.New(errFetchTags)
	}

	availableVersions := make([]*semver.Version, 0, len(tags))
	for _, r := range tags {
		v, err := semver.NewVersion(r)
		if err != nil {
			// We skip any tags that are not valid semantic versions.
			continue
		}
		availableVersions = append(availableVersions, v)
	}

	parentConstraints := make([]*semver.Constraints, 0, len(dep.GetParentConstraints()))
	for _, c := range dep.GetParentConstraints() {
		constraint, err := semver.NewConstraint(c)
		if err != nil {
			log.Debug(errInvalidConstraint, "error", err)
			return "", errors.Wrap(err, errInvalidConstraint)
		}
		parentConstraints = append(parentConstraints, constraint)
	}

	sort.Sort(semver.Collection(availableVersions))
	currentVersion := semver.MustParse(insVer)

	// We aim to find the lowest version that satisfies all parent constraints and is greater than the current version.
	for _, v := range availableVersions {
		// Downgrades are not allowed, so we skip versions that are less than the current version.
		if v.LessThan(currentVersion) {
			continue
		}

		valid := true
		for _, c := range parentConstraints {
			if !c.Check(v) {
				valid = false
				break
			}
		}

		if valid {
			return v.Original(), nil
		}
	}

	log.Debug(errFindDependencyUpgrade, "error", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.GetParentConstraints()))
	return "", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.GetParentConstraints())
}

func (r *Reconciler) getPackageWithID(ctx context.Context, id string, t v1beta1.PackageType) (v1.Package, error) { //nolint:gocognit // TODO(ezgidemirel): This function can be simplified by using a single lister for all package types.
	switch t {
	case v1beta1.ProviderPackageType:
		l := &v1.ProviderList{}
		if err := r.client.List(ctx, l); err != nil {
			return nil, err
		}

		for _, p := range l.Items {
			ref, _ := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(r.registry))
			s, _, err := splitPackage(ref.Name())
			if err != nil {
				return nil, err
			}
			if s == id {
				return &p, nil
			}
		}
	case v1beta1.ConfigurationPackageType:
		l := &v1.ConfigurationList{}
		if err := r.client.List(ctx, l); err != nil {
			return nil, err
		}

		for _, p := range l.Items {
			ref, _ := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(r.registry))
			s, _, err := splitPackage(ref.Name())
			if err != nil {
				return nil, err
			}
			if s == id {
				return &p, nil
			}
		}
	case v1beta1.FunctionPackageType:
		l := &v1.FunctionList{}
		if err := r.client.List(ctx, l); err != nil {
			return nil, err
		}

		for _, p := range l.Items {
			ref, _ := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(r.registry))
			s, _, err := splitPackage(ref.Name())
			if err != nil {
				return nil, err
			}
			if s == id {
				return &p, nil
			}
		}
	}

	return nil, nil
}

// splitPackage splits a package into a repository and a version.
func splitPackage(p string) (repo, version string, err error) {
	var a []string
	if strings.Contains(p, "@") {
		a = strings.Split(p, "@")
	} else {
		a = strings.Split(p, ":")
	}

	if len(a) != 2 {
		return "", "", errors.Errorf(errFmtSplit, len(a))
	}
	return a[0], a[1], nil
}

// findDigestToUpdate returns the digest to update if all parent constraints are the same digest.
// It returns an error, if there is at least one digest which is different from other constraints.
func findDigestToUpdate(node internaldag.Node) (string, error) {
	foundDigest := ""
	foundVersion := false
	for _, c := range node.GetParentConstraints() {
		if d, err := conregv1.NewHash(c); err == nil {
			if foundDigest != "" && foundDigest != d.String() {
				return "", errors.Errorf(errFmtDiffDigests, node.GetParentConstraints())
			}
			foundDigest = d.String()
		} else {
			foundVersion = true
		}

		if foundVersion && foundDigest != "" {
			return "", errors.Errorf(errFmtDiffConstraintTypes, node.GetParentConstraints())
		}
	}

	if foundDigest != "" {
		return foundDigest, nil
	}

	return "", nil
}
