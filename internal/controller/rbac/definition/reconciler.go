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

// Package definition implements the RBAC manager's support for XRDs.
package definition

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/internal/controller/rbac/controller"
)

const (
	timeout = 2 * time.Minute

	errGetXRD    = "cannot get CompositeResourceDefinition"
	errApplyRole = "cannot apply ClusterRoles"
)

// Event reasons.
const (
	reasonApplyRoles event.Reason = "ApplyClusterRoles"
)

// A ClusterRoleRenderer renders ClusterRoles for a given XRD.
type ClusterRoleRenderer interface {
	// RenderClusterRoles for the supplied XRD.
	RenderClusterRoles(d *v2.CompositeResourceDefinition) []rbacv1.ClusterRole
}

// A ClusterRoleRenderFn renders ClusterRoles for the supplied XRD.
type ClusterRoleRenderFn func(d *v2.CompositeResourceDefinition) []rbacv1.ClusterRole

// RenderClusterRoles renders ClusterRoles for the supplied XRD.
func (fn ClusterRoleRenderFn) RenderClusterRoles(d *v2.CompositeResourceDefinition) []rbacv1.ClusterRole {
	return fn(d)
}

// Setup adds a controller that reconciles a CompositeResourceDefinition by
// creating a series of opinionated ClusterRoles that may be bound to allow
// access to the resources it defines.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "rbac/" + strings.ToLower(v2.CompositeResourceDefinitionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v2.CompositeResourceDefinition{}).
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
		r.rbac = rr
	}
}

// NewReconciler returns a Reconciler of CompositeResourceDefinitions.
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

// A Reconciler reconciles CompositeResourceDefinitions.
type Reconciler struct {
	client resource.ClientApplicator
	rbac   ClusterRoleRenderer

	log    logging.Logger
	record event.Recorder
}

// Reconcile a CompositeResourceDefinition by creating a series of opinionated
// ClusterRoles that may be bound to allow access to the resources it defines.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := &v2.CompositeResourceDefinition{}
	if err := r.client.Get(ctx, req.NamespacedName, d); err != nil {
		// In case object is not found, most likely the object was deleted and
		// then disappeared while the event was in the processing queue. We
		// don't need to take any action in that case.
		log.Debug(errGetXRD, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetXRD)
	}

	log = log.WithValues(
		"uid", d.GetUID(),
		"version", d.GetResourceVersion(),
		"name", d.GetName(),
	)

	if meta.WasDeleted(d) {
		// There's nothing to do if our XRD is being deleted. Any ClusterRoles
		// we created will be garbage collected by Kubernetes.
		return reconcile.Result{Requeue: false}, nil
	}

	applied := make([]string, 0)

	for _, cr := range r.rbac.RenderClusterRoles(d) {
		log := log.WithValues("role-name", cr.GetName())
		origRV := ""

		err := r.client.Apply(ctx, &cr,
			resource.MustBeControllableBy(d.GetUID()),
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
			r.record.Event(d, event.Warning(reasonApplyRoles, err))

			return reconcile.Result{}, err
		}

		if cr.GetResourceVersion() != origRV {
			log.Debug("Applied RBAC ClusterRole")

			applied = append(applied, cr.GetName())
		}
	}

	if len(applied) > 0 {
		r.record.Event(d, event.Normal(reasonApplyRoles, fmt.Sprintf("Applied RBAC ClusterRoles: %s", resource.StableNAndSomeMore(resource.DefaultFirstN, applied))))
	}

	// TODO(negz): Add a condition that indicates the RBAC manager is managing
	// cluster roles for this XRD?

	// There's no need to requeue explicitly - we're watching all XRDs.
	return reconcile.Result{Requeue: false}, nil
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
