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

// Package roles implements the RBAC manager's support for providers.
package roles

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/rbac/controller"
)

const (
	timeout = 2 * time.Minute

	errGetPR               = "cannot get ProviderRevision"
	errListPRs             = "cannot list ProviderRevisions"
	errApplyRole           = "cannot apply ClusterRole"
	errValidatePermissions = "cannot validate permission requests"
	errRejectedPermission  = "refusing to apply any RBAC roles due to request for disallowed permission"
)

// Event reasons.
const (
	reasonApplyRoles event.Reason = "ApplyClusterRoles"
)

// A PermissionRequestsValidator validates requested RBAC rules.
type PermissionRequestsValidator interface {
	// ValidatePermissionRequests validates the supplied slice of RBAC rules. It
	// returns a slice of any rejected (i.e. disallowed) rules. It returns an
	// error if it is unable to validate permission requests.
	ValidatePermissionRequests(ctx context.Context, requested ...rbacv1.PolicyRule) ([]Rule, error)
}

// A PermissionRequestsValidatorFn validates requested RBAC rules.
type PermissionRequestsValidatorFn func(ctx context.Context, requested ...rbacv1.PolicyRule) ([]Rule, error)

// ValidatePermissionRequests validates the supplied slice of RBAC rules. It
// returns a slice of any rejected (i.e. disallowed) rules. It returns an error
// if it is unable to validate permission requests.
func (fn PermissionRequestsValidatorFn) ValidatePermissionRequests(ctx context.Context, requested ...rbacv1.PolicyRule) ([]Rule, error) {
	return fn(ctx, requested...)
}

// A ClusterRoleRenderer renders ClusterRoles for the given resources.
type ClusterRoleRenderer interface {
	// RenderClusterRoles for the supplied resources.
	RenderClusterRoles(pr *v1.ProviderRevision, rs []Resource) []rbacv1.ClusterRole
}

// A ClusterRoleRenderFn renders ClusterRoles for the supplied resources.
type ClusterRoleRenderFn func(pr *v1.ProviderRevision, rs []Resource) []rbacv1.ClusterRole

// RenderClusterRoles renders ClusterRoles for the supplied CRDs.
func (fn ClusterRoleRenderFn) RenderClusterRoles(pr *v1.ProviderRevision, rs []Resource) []rbacv1.ClusterRole {
	return fn(pr, rs)
}

// Setup adds a controller that reconciles a ProviderRevision by creating a
// series of opinionated ClusterRoles that may be bound to allow access to the
// resources it defines.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "rbac/" + strings.ToLower(v1.ProviderRevisionGroupKind)

	if o.AllowClusterRole == "" {
		r := NewReconciler(mgr,
			WithLogger(o.Logger.WithValues("controller", name)),
			WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

		return ctrl.NewControllerManagedBy(mgr).
			Named(name).
			For(&v1.ProviderRevision{}).
			Owns(&rbacv1.ClusterRole{}).
			WithOptions(o.ForControllerRuntime()).
			Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
	}

	wrh := &EnqueueRequestForAllRevisionsWithRequests{
		client:          mgr.GetClient(),
		clusterRoleName: o.AllowClusterRole,
	}

	sfh := &EnqueueRequestForAllRevisionsInFamily{
		client: mgr.GetClient(),
	}

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithPermissionRequestsValidator(NewClusterRoleBackedValidator(mgr.GetClient(), o.AllowClusterRole)),
		WithOrgDiffer(OrgDiffer{DefaultRegistry: o.DefaultRegistry}))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&rbacv1.ClusterRole{}).
		Watches(&rbacv1.ClusterRole{}, wrh).
		Watches(&v1.ProviderRevision{}, sfh).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, errors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

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

// WithClientApplicator specifies how the Reconciler should interact with the
// Kubernetes API.
func WithClientApplicator(ca resource.ClientApplicator) ReconcilerOption {
	return func(r *Reconciler) {
		r.client = ca
	}
}

// WithClusterRoleRenderer specifies how the Reconciler should render RBAC
// ClusterRoles.
func WithClusterRoleRenderer(rr ClusterRoleRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.rbac.ClusterRoleRenderer = rr
	}
}

// WithPermissionRequestsValidator specifies how the Reconciler should validate
// requests for extra RBAC permissions.
func WithPermissionRequestsValidator(rv PermissionRequestsValidator) ReconcilerOption {
	return func(r *Reconciler) {
		r.rbac.PermissionRequestsValidator = rv
	}
}

// WithOrgDiffer specifies how the Reconciler should diff OCI orgs. It does this
// to ensure that two providers may only be part of the same family if they're
// in the same OCI org.
func WithOrgDiffer(d OrgDiffer) ReconcilerOption {
	return func(r *Reconciler) {
		r.org = d
	}
}

// NewReconciler returns a Reconciler of ProviderRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		// TODO(negz): Is Updating appropriate here? Probably.
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIUpdatingApplicator(mgr.GetClient()),
		},

		rbac: rbac{
			PermissionRequestsValidator: PermissionRequestsValidatorFn(VerySecureValidator),
			ClusterRoleRenderer:         ClusterRoleRenderFn(RenderClusterRoles),
		},

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

type rbac struct {
	PermissionRequestsValidator
	ClusterRoleRenderer
}

// A Reconciler reconciles ProviderRevisions.
type Reconciler struct {
	client resource.ClientApplicator
	rbac   rbac
	org    OrgDiffer

	log    logging.Logger
	record event.Recorder
}

// Reconcile a ProviderRevision by creating a series of opinionated ClusterRoles
// that may be bound to allow access to the resources it defines.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocognit // Slightly over (13).
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pr := &v1.ProviderRevision{}
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetPR, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPR)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(pr) {
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(pr) {
		// There's nothing to do if our PR is being deleted. Any ClusterRoles
		// we created will be garbage collected by Kubernetes.
		return reconcile.Result{Requeue: false}, nil
	}

	resources := DefinedResources(pr.Status.ObjectRefs)

	// If this revision is part of a provider family we consider it to 'own' all
	// of the family's CRDs (despite it not actually being an owner reference).
	// This allows the revision to use core types installed by another provider,
	// e.g. ProviderConfigs. It also allows the provider to cross-resource
	// reference resources from other providers within its family.
	//
	// TODO(negz): Once generic cross-resource references are implemented we can
	// reduce this to only allowing access to core types, like ProviderConfig.
	// https://github.com/crossplane/crossplane/issues/1770
	if family := pr.GetLabels()[v1.LabelProviderFamily]; family != "" {
		// TODO(negz): Get active revisions in family.
		prs := &v1.ProviderRevisionList{}
		if err := r.client.List(ctx, prs, client.MatchingLabels{v1.LabelProviderFamily: family}); err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errListPRs)
			r.record.Event(pr, event.Warning(reasonApplyRoles, err))
			return reconcile.Result{}, err
		}

		// TODO(negz): Should we filter down to only active revisions? I don't
		// think there's any benefit. If the revision is inactive and never
		// created any CRDs there will be no CRDs to grant permissions for. If
		// it's inactive but did create (or would share) CRDs then this provider
		// might try use them, and we should let it.
		for _, member := range prs.Items {
			// We already added our own resources.
			if member.GetUID() == pr.GetUID() {
				continue
			}

			// We only allow packages in the same OCI registry and org to be
			// part of the same family. This prevents a malicious provider from
			// declaring itself part of a family and thus being granted RBAC
			// access to its types.
			// TODO(negz): Consider using package signing here in future.
			if r.org.Differs(pr.Spec.Package, member.Spec.Package) {
				continue
			}

			resources = append(resources, DefinedResources(member.Status.ObjectRefs)...)
		}
	}

	rejected, err := r.rbac.ValidatePermissionRequests(ctx, pr.Status.PermissionRequests...)
	if err != nil {
		err = errors.Wrap(err, errValidatePermissions)
		r.record.Event(pr, event.Warning(reasonApplyRoles, err))
		return reconcile.Result{}, err
	}

	for _, rule := range rejected {
		r.record.Event(pr, event.Warning(reasonApplyRoles, errors.Errorf("%s %s", errRejectedPermission, rule)))
	}

	// We return early and don't grant _any_ RBAC permissions if we would reject
	// any requested permission. It's better for the provider to be completely
	// and obviously broken than for it to be subtly broken in a way that may
	// not surface immediately, i.e. due to missing an RBAC permission it only
	// occasionally needs. There's no need to requeue - the revisions requests
	// won't change, and we're watching the ClusterRole of allowed requests.
	if len(rejected) > 0 {
		return reconcile.Result{Requeue: false}, nil
	}

	applied := make([]string, 0)
	for _, cr := range r.rbac.RenderClusterRoles(pr, resources) {
		log := log.WithValues("role-name", cr.GetName())
		origRV := ""
		err := r.client.Apply(ctx, &cr,
			resource.MustBeControllableBy(pr.GetUID()),
			resource.AllowUpdateIf(ClusterRolesDiffer),
			resource.StoreCurrentRV(&origRV),
		)
		if resource.IsNotAllowed(err) {
			log.Debug("Skipped no-op RBAC ClusterRole apply")
			continue
		}
		if err != nil {
			if kerrors.IsConflict(err) {
				return reconcile.Result{Requeue: true}, nil
			}
			err = errors.Wrap(err, errApplyRole)
			r.record.Event(pr, event.Warning(reasonApplyRoles, err))
			return reconcile.Result{}, err
		}
		if cr.GetResourceVersion() != origRV {
			log.Debug("Applied RBAC ClusterRole")
			applied = append(applied, cr.GetName())
		}
	}

	if len(applied) > 0 {
		r.record.Event(pr, event.Normal(reasonApplyRoles, fmt.Sprintf("Applied RBAC ClusterRoles: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, applied))))
	}

	// TODO(negz): Add a condition that indicates the RBAC manager is
	// managing cluster roles for this ProviderRevision?

	// There's no need to requeue explicitly - we're watching all PRs.
	return reconcile.Result{Requeue: false}, nil
}

// DefinedResources returns the resources defined by the supplied references.
func DefinedResources(refs []xpv1.TypedReference) []Resource {
	out := make([]Resource, 0, len(refs))
	for _, ref := range refs {
		// This would only return an error if the APIVersion contained more than
		// one "/". This should be impossible, but if it somehow happens we'll
		// just skip this resource since it can't be a CRD.
		gv, _ := schema.ParseGroupVersion(ref.APIVersion)

		// We're only concerned with CRDs.
		if gv.Group != apiextensions.GroupName || ref.Kind != "CustomResourceDefinition" {
			continue
		}

		p, g, valid := strings.Cut(ref.Name, ".")
		if !valid {
			// This shouldn't be possible - CRDs must be named <plural>.<group>.
			continue
		}

		out = append(out, Resource{Group: g, Plural: p})
	}
	return out
}

// ClusterRolesDiffer returns true if the supplied objects are different
// ClusterRoles. We consider ClusterRoles to be different if their labels and
// rules do not match.
func ClusterRolesDiffer(current, desired runtime.Object) bool {
	// Calling this with anything but ClusterRoles is a programming error. If it
	// happens, we probably do want to panic.
	c := current.(*rbacv1.ClusterRole) //nolint:forcetypeassert // See above.
	d := desired.(*rbacv1.ClusterRole) //nolint:forcetypeassert // See above.
	return !cmp.Equal(c.GetLabels(), d.GetLabels()) || !cmp.Equal(c.Rules, d.Rules)
}

// An OrgDiffer determines whether two references are part of the same org. In
// this context we consider an org to consist of:
//
//   - The registry (e.g. xpkg.upbound.io or index.docker.io).
//   - The part of the repository path before the first slash (e.g. crossplane
//     in crossplane/provider-aws).
type OrgDiffer struct {
	// The default OCI registry to use when parsing references.
	DefaultRegistry string
}

// Differs returns true if the supplied references are not part of the same OCI
// registry and org.
func (d OrgDiffer) Differs(a, b string) bool {
	// If we can't parse either reference we can't compare them. Safest thing to
	// do is to assume they're not part of the same org.
	ra, err := name.ParseReference(a, name.WithDefaultRegistry(d.DefaultRegistry))
	if err != nil {
		return true
	}
	rb, err := name.ParseReference(b, name.WithDefaultRegistry(d.DefaultRegistry))
	if err != nil {
		return true
	}

	ca := ra.Context()
	cb := rb.Context()

	// If the registries (e.g. xpkg.upbound.io) don't match they're not in the
	// same org.
	if ca.RegistryStr() != cb.RegistryStr() {
		return true
	}

	oa := strings.Split(ca.RepositoryStr(), "/")[0]
	ob := strings.Split(cb.RepositoryStr(), "/")[0]

	return oa != ob
}
