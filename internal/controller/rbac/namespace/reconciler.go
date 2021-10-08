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

package namespace

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	shortWait = 30 * time.Second

	timeout        = 2 * time.Minute
	maxConcurrency = 5

	errGetNamespace = "cannot get CompositeResourceDefinition"
	errApplyRole    = "cannot apply Roles"
	errListRoles    = "cannot list ClusterRoles"
)

// Event reasons.
const (
	reasonApplyRoles event.Reason = "ApplyRoles"
)

// A RoleRenderer renders Roles for a given Namespace.
type RoleRenderer interface {
	// RenderRoles for the supplied Namespace.
	RenderRoles(d *corev1.Namespace, crs []rbacv1.ClusterRole) []rbacv1.Role
}

// A RoleRenderFn renders Roles for the supplied Namespace.
type RoleRenderFn func(d *corev1.Namespace, crs []rbacv1.ClusterRole) []rbacv1.Role

// RenderRoles renders Roles for the supplied Namespace.
func (fn RoleRenderFn) RenderRoles(d *corev1.Namespace, crs []rbacv1.ClusterRole) []rbacv1.Role {
	return fn(d, crs)
}

// Setup adds a controller that reconciles a Namespace by creating a series of
// opinionated Roles that may be bound to allow access to resources within that
// namespace.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "rbac/namespace"

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&corev1.Namespace{}).
		Owns(&rbacv1.Role{}).
		Watches(&source.Kind{Type: &rbacv1.ClusterRole{}}, &EnqueueRequestForNamespaces{client: mgr.GetClient()}).
		WithOptions(kcontroller.Options{MaxConcurrentReconciles: maxConcurrency}).
		Complete(NewReconciler(mgr,
			WithLogger(log.WithValues("controller", name)),
			WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name)))))
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

// WithRoleRenderer specifies how the Reconciler should render RBAC
// Roles.
func WithRoleRenderer(rr RoleRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.rbac = rr
	}
}

// NewReconciler returns a Reconciler of Namespaces.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		// TODO(negz): Is Updating appropriate here? Probably.
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIUpdatingApplicator(mgr.GetClient()),
		},

		rbac: RoleRenderFn(RenderRoles),

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles Namespaces.
type Reconciler struct {
	client resource.ClientApplicator
	rbac   RoleRenderer

	log    logging.Logger
	record event.Recorder
}

// Reconcile a Namespace by creating a series of opinionated Roles that may be
// bound to allow access to resources within that namespace.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ns := &corev1.Namespace{}
	if err := r.client.Get(ctx, req.NamespacedName, ns); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetNamespace, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetNamespace)
	}

	log = log.WithValues(
		"uid", ns.GetUID(),
		"version", ns.GetResourceVersion(),
		"name", ns.GetName(),
	)

	if meta.WasDeleted(ns) {
		// There's nothing to do if our namespace is being deleted. Any Roles we
		// created will be deleted along with the namespace.
		return reconcile.Result{Requeue: false}, nil
	}

	// NOTE(negz): We don't expect there to be an unwieldy amount of roles, so
	// we just list and pass them all. We're listing from a cache that handles
	// label selectors locally, so filtering with a label selector here won't
	// meaningfully improve performance relative to filtering in RenderRoles.
	// https://github.com/kubernetes-sigs/controller-runtime/blob/d6829e9/pkg/cache/internal/cache_reader.go#L131
	l := &rbacv1.ClusterRoleList{}
	if err := r.client.List(ctx, l); err != nil {
		log.Debug(errListRoles, "error", err)
		r.record.Event(ns, event.Warning(reasonApplyRoles, errors.Wrap(err, errListRoles)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	for _, rl := range r.rbac.RenderRoles(ns, l.Items) {
		log = log.WithValues("role-name", rl.GetName())
		rl := rl // Pin range variable so we can take its address.

		err := r.client.Apply(ctx, &rl, resource.MustBeControllableBy(ns.GetUID()), resource.AllowUpdateIf(RolesDiffer))
		if resource.IsNotAllowed(err) {
			log.Debug("Skipped no-op RBAC Role apply")
			continue
		}
		if err != nil {
			log.Debug(errApplyRole, "error", err)
			r.record.Event(ns, event.Warning(reasonApplyRoles, errors.Wrap(err, errApplyRole)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
		}

		log.Debug("Applied RBAC Role")
	}

	r.record.Event(ns, event.Normal(reasonApplyRoles, "Applied RBAC Roles"))
	return reconcile.Result{Requeue: false}, nil
}

// RolesDiffer returns true if the supplied objects are different Roles. We
// consider Roles to be different if their annotations and rules do not match.
func RolesDiffer(current, desired runtime.Object) bool {
	c := current.(*rbacv1.Role)
	d := desired.(*rbacv1.Role)
	return !cmp.Equal(c.GetAnnotations(), d.GetAnnotations()) || !cmp.Equal(c.Rules, d.Rules)
}
