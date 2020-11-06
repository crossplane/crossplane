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
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	shortWait = 30 * time.Second

	timeout        = 2 * time.Minute
	maxConcurrency = 5

	errGetPR     = "cannot get ProviderRevision"
	errListCRDs  = "cannot list CustomResourceDefinitions"
	errApplyRole = "cannot apply ClusterRole"
)

// Event reasons.
const (
	reasonApplyRoles event.Reason = "ApplyClusterRoles"
)

// A ClusterRoleRenderer renders ClusterRoles for the given CRDs.
type ClusterRoleRenderer interface {
	// RenderClusterRoles for the supplied CRDs.
	RenderClusterRoles(pr *v1alpha1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole
}

// A ClusterRoleRenderFn renders ClusterRoles for the supplied CRDs.
type ClusterRoleRenderFn func(pr *v1alpha1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole

// RenderClusterRoles renders ClusterRoles for the supplied CRDs.
func (fn ClusterRoleRenderFn) RenderClusterRoles(pr *v1alpha1.ProviderRevision, crds []extv1.CustomResourceDefinition) []rbacv1.ClusterRole {
	return fn(pr, crds)
}

// Setup adds a controller that reconciles a ProviderRevision by creating a
// series of opinionated ClusterRoles that may be bound to allow access to the
// resources it defines.
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "rbac/" + strings.ToLower(v1alpha1.ProviderRevisionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ProviderRevision{}).
		Owns(&rbacv1.ClusterRole{}).
		Watches(&source.Kind{Type: &extv1.CustomResourceDefinition{}}, &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.ProviderRevision{}}).
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

// WithClusterRoleRenderer specifies how the Reconciler should render RBAC
// ClusterRoles.
func WithClusterRoleRenderer(rr ClusterRoleRenderer) ReconcilerOption {
	return func(r *Reconciler) {
		r.rbac = rr
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

		rbac: ClusterRoleRenderFn(RenderClusterRoles),

		log:    logging.NewNopLogger(),
		record: event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

// A Reconciler reconciles ProviderRevisions.
type Reconciler struct {
	client resource.ClientApplicator
	rbac   ClusterRoleRenderer

	log    logging.Logger
	record event.Recorder
}

// Reconcile a ProviderRevision by creating a series of opinionated ClusterRoles
// that may be bound to allow access to the resources it defines.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { //nolint:gocyclo
	// NOTE(negz): This reconciler is a tiny bit over our desired cyclomatic
	// complexity score. Be wary of adding additional complexity.

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pr := &v1alpha1.ProviderRevision{}
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
		r.record.Event(pr, event.Warning(reasonApplyRoles, errors.Wrap(err, errListCRDs)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
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
			r.record.Event(pr, event.Warning(reasonApplyRoles, errors.Wrap(err, errApplyRole)))
			return reconcile.Result{RequeueAfter: shortWait}, nil
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
