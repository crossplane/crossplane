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

package roles

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	errListCRDs            = "cannot list CustomResourceDefinitions"
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

// A ClusterRoleRenderer renders ClusterRoles for the given CRDs.
type ClusterRoleRenderer interface {
	// RenderClusterRoles for the supplied CRDs.
	RenderClusterRoles(pr *v1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole
}

// A ClusterRoleRenderFn renders ClusterRoles for the supplied CRDs.
type ClusterRoleRenderFn func(pr *v1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole

// RenderClusterRoles renders ClusterRoles for the supplied CRDs.
func (fn ClusterRoleRenderFn) RenderClusterRoles(pr *v1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole {
	return fn(pr, crds)
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
			Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
	}

	h := &EnqueueRequestForAllRevisionsWithRequests{
		client:          mgr.GetClient(),
		clusterRoleName: o.AllowClusterRole}

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		WithPermissionRequestsValidator(NewClusterRoleBackedValidator(mgr.GetClient(), o.AllowClusterRole)))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&rbacv1.ClusterRole{}).
		Watches(&source.Kind{Type: &rbacv1.ClusterRole{}}, h).
		WithOptions(o.ForControllerRuntime()).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
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

	log    logging.Logger
	record event.Recorder
}

// Reconcile a ProviderRevision by creating a series of opinionated ClusterRoles
// that may be bound to allow access to the resources it defines.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo
	// NOTE(negz): This reconciler is a little over our desired cyclomatic
	// complexity score. Be wary of adding additional complexity.

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

	if meta.WasDeleted(pr) {
		// There's nothing to do if our PR is being deleted. Any ClusterRoles
		// we created will be garbage collected by Kubernetes.
		return reconcile.Result{Requeue: false}, nil
	}

	l := &extv1.CustomResourceDefinitionList{}
	if err := r.client.List(ctx, l); err != nil {
		log.Debug(errListCRDs, "error", err)
		err = errors.Wrap(err, errListCRDs)
		r.record.Event(pr, event.Warning(reasonApplyRoles, err))
		return reconcile.Result{}, err
	}

	// Filter down to the CRDs that are owned by this ProviderRevision - i.e.
	// those that it may become the active revision for.
	crds := make([]extv1.CustomResourceDefinition, 0)
	for _, crd := range l.Items {
		for _, ref := range crd.GetOwnerReferences() {
			if ref.UID == pr.GetUID() {
				crds = append(crds, crd)
			}
		}
	}

	rejected, err := r.rbac.ValidatePermissionRequests(ctx, pr.Status.PermissionRequests...)
	if err != nil {
		log.Debug(errValidatePermissions, "error", err)
		err = errors.Wrap(err, errValidatePermissions)
		r.record.Event(pr, event.Warning(reasonApplyRoles, err))
		return reconcile.Result{}, err
	}

	for _, rule := range rejected {
		log.Debug(errRejectedPermission, "rule", rule)
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

	for _, cr := range r.rbac.RenderClusterRoles(pr, crds) {
		cr := cr // Pin range variable so we can take its address.
		log = log.WithValues("role-name", cr.GetName())
		err := r.client.Apply(ctx, &cr, resource.MustBeControllableBy(pr.GetUID()), resource.AllowUpdateIf(ClusterRolesDiffer))
		if resource.IsNotAllowed(err) {
			log.Debug("Skipped no-op RBAC ClusterRole apply")
			continue
		}
		if err != nil {
			log.Debug(errApplyRole, "error", err)
			err = errors.Wrap(err, errApplyRole)
			r.record.Event(pr, event.Warning(reasonApplyRoles, err))
			return reconcile.Result{}, err
		}
		log.Debug("Applied RBAC ClusterRole")
	}

	// TODO(negz): Add a condition that indicates the RBAC manager is
	// managing cluster roles for this ProviderRevision?
	r.record.Event(pr, event.Normal(reasonApplyRoles, "Applied RBAC ClusterRoles"))

	// There's no need to requeue explicitly - we're watching all PRs.
	return reconcile.Result{Requeue: false}, nil
}

// ClusterRolesDiffer returns true if the supplied objects are different
// ClusterRoles. We consider ClusterRoles to be different if their labels and
// rules do not match.
func ClusterRolesDiffer(current, desired runtime.Object) bool {
	c := current.(*rbacv1.ClusterRole)
	d := desired.(*rbacv1.ClusterRole)
	return !cmp.Equal(c.GetLabels(), d.GetLabels()) || !cmp.Equal(c.Rules, d.Rules)
}
