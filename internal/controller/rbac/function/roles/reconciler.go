/*
Copyright 2025 The Crossplane Authors.

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

// Package roles implements the RBAC manager's support for functions.
package roles

import (
	"context"
	"fmt"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/controller"
	"github.com/crossplane/crossplane/v2/internal/controller/rbac/roles"
)

const (
	timeout = 2 * time.Minute

	errGetFR     = "cannot get FunctionRevision"
	errApplyRole = "cannot apply ClusterRole"
)

// Event reasons.
const (
	reasonApplyRoles event.Reason = "ApplyClusterRoles"
)

// A ClusterRoleRenderer renders ClusterRoles for the given resources.
type ClusterRoleRenderer interface {
	// RenderClusterRoles for the supplied resources.
	RenderClusterRoles(fr *v1.FunctionRevision, rs []roles.Resource) []rbacv1.ClusterRole
}

// A ClusterRoleRenderFn renders ClusterRoles for the supplied resources.
type ClusterRoleRenderFn func(fr *v1.FunctionRevision, rs []roles.Resource) []rbacv1.ClusterRole

// RenderClusterRoles renders ClusterRoles for the supplied CRDs.
func (fn ClusterRoleRenderFn) RenderClusterRoles(fr *v1.FunctionRevision, rs []roles.Resource) []rbacv1.ClusterRole {
	return fn(fr, rs)
}

// Setup adds a controller that reconciles a FunctionRevision by creating a
// series of opinionated ClusterRoles that may be bound to allow access to the
// resources it defines.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "rbac-roles/" + strings.ToLower(v1.FunctionRevisionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.FunctionRevision{}).
		Owns(&rbacv1.ClusterRole{}).
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

// NewReconciler returns a Reconciler of FunctionRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		// TODO(negz): Is Updating appropriate here? Probably.
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIUpdatingApplicator(mgr.GetClient()),
		},

		rbac: rbac{
			ClusterRoleRenderer: ClusterRoleRenderFn(RenderClusterRoles),
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
	ClusterRoleRenderer
}

// A Reconciler reconciles FunctionRevisions.
type Reconciler struct {
	client resource.ClientApplicator
	rbac   rbac

	log    logging.Logger
	record event.Recorder
}

// Reconcile a FunctionRevision by creating a series of opinionated ClusterRoles
// that may be bound to allow access to the resources it defines.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fr := &v1.FunctionRevision{}
	if err := r.client.Get(ctx, req.NamespacedName, fr); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetFR, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetFR)
	}

	log = log.WithValues(
		"uid", fr.GetUID(),
		"version", fr.GetResourceVersion(),
		"name", fr.GetName(),
	)

	// Check the pause annotation and return if it has the value "true"
	// after logging, publishing an event and updating the SYNC status condition
	if meta.IsPaused(fr) {
		return reconcile.Result{}, nil
	}

	if meta.WasDeleted(fr) {
		// There's nothing to do if our FR is being deleted. Any ClusterRoles
		// we created will be garbage collected by Kubernetes.
		return reconcile.Result{Requeue: false}, nil
	}

	resources := roles.DefinedResources(fr.Status.ObjectRefs)

	applied := make([]string, 0)

	for _, cr := range r.rbac.RenderClusterRoles(fr, resources) {
		log := log.WithValues("role-name", cr.GetName())
		origRV := ""

		err := r.client.Apply(ctx, &cr,
			resource.MustBeControllableBy(fr.GetUID()),
			resource.AllowUpdateIf(roles.ClusterRolesDiffer),
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
			r.record.Event(fr, event.Warning(reasonApplyRoles, err))

			return reconcile.Result{}, err
		}

		if cr.GetResourceVersion() != origRV {
			log.Debug("Applied RBAC ClusterRole")

			applied = append(applied, cr.GetName())
		}
	}

	if len(applied) > 0 {
		r.record.Event(fr, event.Normal(reasonApplyRoles, fmt.Sprintf("Applied RBAC ClusterRoles: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, applied))))
	}

	// TODO(negz): Add a condition that indicates the RBAC manager is
	// managing cluster roles for this FunctionRevision?

	// There's no need to requeue explicitly - we're watching all FRs.
	return reconcile.Result{Requeue: false}, nil
}
