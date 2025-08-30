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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/controller/pkg/controller"
	internaldag "github.com/crossplane/crossplane/v2/internal/dag"
	"github.com/crossplane/crossplane/v2/internal/features"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
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
	errRewriteImage           = "cannot rewrite image path using config"
	errInvalidRewrite         = "rewritten image path is invalid"
	errFetchTags              = "cannot fetch dependency package tags"
	errFindDependencyUpgrade  = "cannot find dependency version to upgrade"
	errFmtNoValidVersion      = "dependency (%s) does not have a valid version to upgrade that satisfies all constraints. If there is a valid version that requires downgrade, manual intervention is required. Constraints: %v"
	errGetDependency          = "cannot get dependency package"
	errConstructDependency    = "cannot construct dependency package"
	errCreateDependency       = "cannot create dependency package"
	errUpdateDependency       = "cannot update dependency package"
	errFmtSplit               = "package should have 2 segments after split but has %d"
	errFmtDiffConstraintTypes = "a dependency package has different types of parent constraints (%v)"
	errFmtDiffDigests         = "a dependency package has different digests in parent constraints (%v)"
	errCannotUpdateStatus     = "cannot update status"
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

// WithFeatures specifies which feature flags should be enabled.
func WithFeatures(f *feature.Flags) ReconcilerOption {
	return func(r *Reconciler) {
		r.features = f
	}
}

// WithDowngradesEnabled sets whether upgrades are enabled or not.
func WithDowngradesEnabled() ReconcilerOption {
	return func(r *Reconciler) {
		r.downgradesEnabled = true
	}
}

// Reconciler reconciles packages.
type Reconciler struct {
	client     client.Client
	log        logging.Logger
	lock       resource.Finalizer
	newDag     internaldag.NewDAGFn
	fetcher    xpkg.Fetcher
	config     xpkg.ConfigStore
	features   *feature.Flags
	conditions conditions.Manager

	downgradesEnabled bool
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
		WithConfigStore(xpkg.NewImageConfigStore(mgr.GetClient(), o.Namespace)),
		WithFeatures(o.Features),
	}

	if o.Features.Enabled(features.EnableAlphaDependencyVersionUpgrades) {
		opts = append(opts, WithNewDagFn(internaldag.NewUpgradingMapDag))

		if o.Features.Enabled(features.EnableAlphaDependencyVersionDowngrades) {
			opts = append(opts, WithDowngradesEnabled())
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.Lock{}).
		Watches(&v1.ConfigurationRevision{}, handler.EnqueueRequestsFromMapFunc(ForName(lockName))).
		Watches(&v1.ProviderRevision{}, handler.EnqueueRequestsFromMapFunc(ForName(lockName))).
		Watches(&v1.FunctionRevision{}, handler.EnqueueRequestsFromMapFunc(ForName(lockName))).
		// Ideally we should enqueue only if the ImageConfig applies to a
		// package in the Lock which would require getting/parsing the Lock and
		// checking the source of each package against the prefixes in the
		// ImageConfig. However, this is a bit more complex than needed, and we
		// don't expect to have many ImageConfigs so we just enqueue for all
		// ImageConfigs with a pull secret.
		Watches(&v1beta1.ImageConfig{}, handler.EnqueueRequestsFromMapFunc(ForName(lockName, HasPullSecret()))).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(NewReconciler(mgr, opts...)), o.GlobalRateLimiter))
}

// NewReconciler creates a new lock dependency reconciler.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client:     mgr.GetClient(),
		lock:       resource.NewAPIFinalizer(mgr.GetClient(), finalizer),
		log:        logging.NewNopLogger(),
		newDag:     internaldag.NewMapDag,
		fetcher:    xpkg.NewNopFetcher(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile the lock by resolving dependencies.
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

	status := r.conditions.For(lock)

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

		lock.CleanConditions()

		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
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
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errBuildDAG)))

		_ = r.client.Status().Update(ctx, lock)

		return reconcile.Result{}, errors.Wrap(err, errBuildDAG)
	}

	// Make sure we don't have any cyclical imports. If we do, refuse to
	// install additional packages.
	_, err = dag.Sort()
	if err != nil {
		log.Debug(errSortDAG, "error", err)
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errSortDAG)))

		_ = r.client.Status().Update(ctx, lock)

		return reconcile.Result{}, errors.Wrap(err, errSortDAG)
	}

	if len(implied) == 0 {
		status.MarkConditions(v1beta1.ResolutionSucceeded())
		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
	}

	// If we are missing a node, we want to create it. The resolver never
	// modifies the Lock. We only create the first implied node as we will
	// be requeued when it adds itself to the Lock, at which point we will
	// check for missing nodes again.
	dep, ok := implied[0].(*v1beta1.Dependency)

	depID := dep.Identifier()
	if !ok {
		log.Debug(errInvalidDependency, "error", errors.Errorf(errFmtMissingDependency, depID))
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Errorf(errFmtMissingDependency, depID)))

		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
	}

	// NOTE(phisco): dependencies identifiers are without registry and tag, so we can't enforce strict validation.
	ref, err := name.ParseReference(depID)
	if err != nil {
		log.Debug(errInvalidDependency, "error", err)
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errInvalidDependency)))

		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
	}

	var (
		pkg              *unstructured.Unstructured
		installedVersion string
	)

	if r.features.Enabled(features.EnableAlphaDependencyVersionUpgrades) {
		l, err := NewPackageList(dep)
		if err != nil {
			log.Debug(errGetDependency, "error", err)
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errGetDependency)))

			_ = r.client.Status().Update(ctx, lock)

			return reconcile.Result{}, errors.Wrap(err, errGetDependency)
		}

		if err := r.client.List(ctx, l); err != nil {
			log.Debug(errGetDependency, "error", err)
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errGetDependency)))

			_ = r.client.Status().Update(ctx, lock)

			return reconcile.Result{}, errors.Wrap(err, errGetDependency)
		}

		for _, p := range l.Items {
			// Start with spec.package, which should always be set.
			source, _ := fieldpath.Pave(p.Object).GetString("spec.package")

			// If status.resolvedPackage is set, use that. This is
			// the "real" package, as resolved by applying any
			// ImageConfigs that might rewrite spec.package.
			if resolved, err := fieldpath.Pave(p.Object).GetString("status.resolvedPackage"); err == nil && resolved != "" {
				source = resolved
			}

			pref, err := name.ParseReference(source, name.StrictValidation)
			if err != nil {
				continue
			}

			if pref.Context().Name() == ref.Context().Name() {
				pkg = &p
				installedVersion = pref.Identifier()
			}
		}
	}

	if pkg == nil {
		// At this point, we know that the dependency is either missing or does not satisfy the constraints.
		// Package does not exist. We need to create it.
		var addVer string
		if addVer, err = r.findDependencyVersionToInstall(ctx, dep, log, ref); err != nil {
			log.Debug(errFindDependency, "error", errors.Wrapf(err, depID, dep.Constraints))
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errFindDependency)))

			_ = r.client.Status().Update(ctx, lock)

			return reconcile.Result{Requeue: false}, errors.Wrap(err, errFindDependency)
		}

		// NOTE(hasheddan): consider creating event on package revision
		// dictating constraints.
		if addVer == "" {
			log.Debug(errFindDependencyUpgrade, "error", errors.Errorf(errFmtNoValidVersion, depID, dep.Constraints))
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Errorf(errFmtNoValidVersion, depID, dep.Constraints)))

			return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
		}

		pack, err := NewPackage(dep, addVer, ref)
		if err != nil {
			log.Debug(errConstructDependency, "error", err)
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errConstructDependency)))

			_ = r.client.Status().Update(ctx, lock)

			return reconcile.Result{}, errors.Wrap(err, errConstructDependency)
		}

		// NOTE(hasheddan): consider making the lock the controller of packages
		// it creates.
		if err := r.client.Create(ctx, pack); err != nil && !kerrors.IsAlreadyExists(err) {
			log.Debug(errCreateDependency, "error", err)
			status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errCreateDependency)))

			_ = r.client.Status().Update(ctx, lock)

			return reconcile.Result{}, errors.Wrap(err, errCreateDependency)
		}

		status.MarkConditions(v1beta1.ResolutionSucceeded())

		return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
	}

	if !r.features.Enabled(features.EnableAlphaDependencyVersionUpgrades) {
		return reconcile.Result{}, nil
	}

	// The package is installed, but does not satisfy the constraints and
	// upgrade flag is enabled. We need to search for a new version that
	// satisfies all the constraints.

	n, err := dag.GetNode(depID)
	if err != nil {
		log.Debug(errInvalidDependency, "error", errors.Errorf(errFmtMissingDependency, depID))
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Errorf(errFmtMissingDependency, depID)))

		_ = r.client.Status().Update(ctx, lock)

		return reconcile.Result{}, errors.Errorf(errFmtMissingDependency, depID)
	}

	newVer, err := r.findDependencyVersionToUpdate(ctx, ref, installedVersion, n, log)
	if err != nil {
		log.Debug(errFindDependencyUpgrade, "error", errors.Wrapf(err, depID, dep.Constraints))
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errFindDependencyUpgrade)))

		_ = r.client.Status().Update(ctx, lock)

		return reconcile.Result{}, errors.Wrap(err, errFindDependencyUpgrade)
	}

	// Update the package with the new version.
	format := packageTagFmt
	if strings.HasPrefix(newVer, "sha256:") {
		format = packageDigestFmt
	}

	_ = fieldpath.Pave(pkg.Object).SetString("spec.package", fmt.Sprintf(format, ref.String(), newVer))

	if err := r.client.Update(ctx, pkg); err != nil {
		log.Debug(errUpdateDependency, "error", err)
		status.MarkConditions(v1beta1.ResolutionFailed(errors.Wrap(err, errUpdateDependency)))

		_ = r.client.Status().Update(ctx, lock)

		return reconcile.Result{}, errors.Wrap(err, errUpdateDependency)
	}

	status.MarkConditions(v1beta1.ResolutionSucceeded())

	return reconcile.Result{}, errors.Wrap(r.client.Status().Update(ctx, lock), errCannotUpdateStatus)
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

	// Rewrite the image path if necessary. We need to do this before looking
	// for pull secrets, since the rewritten path may use different secrets than
	// the original.
	rewriteConfigName, newPath, err := r.config.RewritePath(ctx, ref.String())
	if err != nil {
		log.Info("cannot rewrite image path using config", "error", err)
		return "", errors.Wrap(err, errRewriteImage)
	}

	if newPath != "" {
		// NOTE(phisco): newPath is a dependency identifier, which are without registry and tag, so we can't enforce strict validation.
		ref, err = name.ParseReference(newPath)
		if err != nil {
			log.Info("rewritten image path is invalid", "error", err)
			return "", errors.Wrap(err, errInvalidRewrite)
		}
	}

	psConfig, ps, err := r.config.PullSecretFor(ctx, ref.String())
	if err != nil {
		log.Info("cannot get pull secret from image config store", "error", err)
		return "", errors.Wrap(err, errGetPullConfig)
	}

	var s []string

	if ps != "" {
		log.Debug("Selected pull secret from image config store", "image", ref.String(), "pullSecretConfig", psConfig, "pullSecret", ps, "rewriteConfig", rewriteConfigName)
		s = append(s, ps)
	}
	// NOTE(hasheddan): we will be unable to fetch tags for private
	// dependencies because we do not attach any secrets. Consider copying
	// secrets from parent dependencies.
	tags, err := r.fetcher.Tags(ctx, ref, s...)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return "", errors.Wrap(err, errFetchTags)
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

// findDependencyVersionToUpdate finds a valid version to update the dependency considering the parent constraints.
func (r *Reconciler) findDependencyVersionToUpdate(ctx context.Context, ref name.Reference, insVer string, dep internaldag.Node, log logging.Logger) (string, error) {
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

	// Rewrite the image path if necessary. We need to do this before looking
	// for pull secrets, since the rewritten path may use different secrets than
	// the original.
	rewriteConfigName, newPath, err := r.config.RewritePath(ctx, ref.String())
	if err != nil {
		log.Info("cannot rewrite image path using config", "error", err)
		return "", errors.Wrap(err, errRewriteImage)
	}

	if newPath != "" {
		// NOTE(phisco): it's a dependency's reference, so we can not enforce strict validation.
		ref, err = name.ParseReference(newPath)
		if err != nil {
			log.Info("rewritten image path is invalid", "error", err)
			return "", errors.Wrap(err, errInvalidRewrite)
		}
	}

	psConfig, ps, err := r.config.PullSecretFor(ctx, ref.String())
	if err != nil {
		log.Info("cannot get pull secret from image config store", "error", err)
		return "", errors.Wrap(err, errGetPullConfig)
	}

	var s []string

	if ps != "" {
		log.Debug("Selected pull secret from image config store", "image", ref.String(), "pullSecretConfig", psConfig, "pullSecret", ps, "rewriteConfig", rewriteConfigName)
		s = append(s, ps)
	}

	tags, err := r.fetcher.Tags(ctx, ref, s...)
	if err != nil {
		log.Debug(errFetchTags, "error", err)
		return "", errors.Wrap(err, errFetchTags)
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

	var targetVersion *semver.Version

	// We aim to find the lowest version that satisfies all parent constraints and is greater than the current version.
	for _, v := range availableVersions {
		valid := true

		for _, c := range parentConstraints {
			if !c.Check(v) {
				valid = false
				break
			}
		}

		// If we're upgrading, we target the first valid version that is greater than the current version.
		if (v.GreaterThan(currentVersion) || v.Equal(currentVersion)) && valid {
			return v.Original(), nil
		}

		// If we're downgrading, we target the largest valid version that is less than the current version.
		if r.downgradesEnabled && valid {
			targetVersion = v
		}
	}

	if targetVersion != nil {
		return targetVersion.Original(), nil
	}

	log.Debug(errFindDependencyUpgrade, "error", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.GetParentConstraints()))

	return "", errors.Errorf(errFmtNoValidVersion, dep.Identifier(), dep.GetParentConstraints())
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

// NewPackage creates a new package from the given dependency and version.
func NewPackage(dep *v1beta1.Dependency, version string, ref name.Reference) (*unstructured.Unstructured, error) {
	pack := &unstructured.Unstructured{}
	pack.SetName(xpkg.ToDNSLabel(ref.Context().RepositoryStr()))

	format := packageTagFmt
	if strings.HasPrefix(version, "sha256:") {
		format = packageDigestFmt
	}

	_ = fieldpath.Pave(pack.Object).SetString("spec.package", fmt.Sprintf(format, ref.String(), version))

	switch {
	case dep.APIVersion != nil && dep.Kind != nil:
		pack.SetAPIVersion(*dep.APIVersion)
		pack.SetKind(*dep.Kind)
	case ptr.Deref(dep.Type, "") == v1beta1.ConfigurationPackageType:
		pack.SetAPIVersion(v1.ConfigurationGroupVersionKind.GroupVersion().String())
		pack.SetKind(v1.ConfigurationKind)
	case ptr.Deref(dep.Type, "") == v1beta1.ProviderPackageType:
		pack.SetAPIVersion(v1.ProviderGroupVersionKind.GroupVersion().String())
		pack.SetKind(v1.ProviderKind)
	case ptr.Deref(dep.Type, "") == v1beta1.FunctionPackageType:
		pack.SetAPIVersion(v1.FunctionGroupVersionKind.GroupVersion().String())
		pack.SetKind(v1.FunctionKind)
	default:
		return nil, errors.Errorf("encountered an invalid dependency: package dependencies must specify either a valid type, or an explicit apiVersion, kind, and package")
	}

	return pack, nil
}

// NewPackageList creates an empty package list suitable to get packages.
func NewPackageList(dep *v1beta1.Dependency) (*unstructured.UnstructuredList, error) {
	l := &unstructured.UnstructuredList{}

	switch {
	case dep.APIVersion != nil && dep.Kind != nil:
		l.SetAPIVersion(*dep.APIVersion)
		l.SetKind(*dep.Kind + "List")
	case ptr.Deref(dep.Type, "") == v1beta1.ConfigurationPackageType:
		l.SetAPIVersion(v1.ConfigurationGroupVersionKind.GroupVersion().String())
		l.SetKind(v1.ConfigurationKind + "List")
	case ptr.Deref(dep.Type, "") == v1beta1.ProviderPackageType:
		l.SetAPIVersion(v1.ProviderGroupVersionKind.GroupVersion().String())
		l.SetKind(v1.ProviderKind + "List")
	case ptr.Deref(dep.Type, "") == v1beta1.FunctionPackageType:
		l.SetAPIVersion(v1.FunctionGroupVersionKind.GroupVersion().String())
		l.SetKind(v1.FunctionKind + "List")
	default:
		return nil, errors.Errorf("encountered an invalid dependency: package dependencies must specify either a valid type, or an explicit apiVersion, kind, and package")
	}

	return l, nil
}
