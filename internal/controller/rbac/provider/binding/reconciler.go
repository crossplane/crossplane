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

// Package binding implements the RBAC manager's support for providers.
package binding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/controller/rbac/controller"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/roles"
)

const (
	timeout = 2 * time.Minute

	errGetPR        = "cannot get ProviderRevision"
	errDeployments  = "cannot list Deployments"
	errApplyBinding = "cannot apply ClusterRoleBinding"

	kindClusterRole = "ClusterRole"
)

// Event reasons.
const (
	reasonBind event.Reason = "BindClusterRole"
)

// Setup adds a controller that reconciles a ProviderRevision by creating a
// ClusterRoleBinding that binds a provider's service account to its system
// ClusterRole.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "rbac/" + strings.ToLower(v1.ProviderRevisionGroupKind)

	r := NewReconciler(mgr,
		WithLogger(o.Logger.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1.ProviderRevision{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &v1.ProviderRevision{})).
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

// NewReconciler returns a Reconciler of ProviderRevisions.
func NewReconciler(mgr manager.Manager, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		// TODO(negz): Is Updating appropriate here? Probably.
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIUpdatingApplicator(mgr.GetClient()),
		},

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

	log    logging.Logger
	record event.Recorder
}

// Reconcile a ProviderRevision by creating a ClusterRoleBinding that binds a
// provider's service account to its system ClusterRole.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
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

	l := &appsv1.DeploymentList{}
	if err := r.client.List(ctx, l); err != nil {
		err = errors.Wrap(err, errDeployments)
		r.record.Event(pr, event.Warning(reasonBind, err))
		return reconcile.Result{}, err
	}

	// Filter down to the Deployments that are owned by this
	// ProviderRevision. Each revision should control at most one, but it's easy
	// and relatively harmless for us to handle there being many.
	subjects := make([]rbacv1.Subject, 0)
	subjectStrings := make([]string, 0)
	for _, d := range l.Items {
		for _, ref := range d.GetOwnerReferences() {
			if ref.UID == pr.GetUID() {
				sa := d.Spec.Template.Spec.ServiceAccountName
				ns := d.Namespace

				subjects = append(subjects, rbacv1.Subject{
					Kind:      rbacv1.ServiceAccountKind,
					Namespace: ns,
					Name:      sa,
				})
				subjectStrings = append(subjectStrings, ns+"/"+sa)
			}
		}
	}

	n := roles.SystemClusterRoleName(pr.GetName())
	ref := meta.AsController(meta.TypedReferenceTo(pr, v1.ProviderRevisionGroupVersionKind))
	rb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            n,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     kindClusterRole,
			Name:     n,
		},
		Subjects: subjects,
	}

	log = log.WithValues(
		"binding-name", n,
		"role-name", n,
		"subjects", subjects,
	)

	err := r.client.Apply(ctx, rb, resource.MustBeControllableBy(pr.GetUID()), resource.AllowUpdateIf(ClusterRoleBindingsDiffer))
	if resource.IsNotAllowed(err) {
		log.Debug("Skipped no-op ClusterRoleBinding apply")
		return reconcile.Result{}, nil
	}
	if err != nil {
		if kerrors.IsConflict(err) {
			return reconcile.Result{Requeue: true}, nil
		}
		err = errors.Wrap(err, errApplyBinding)
		r.record.Event(pr, event.Warning(reasonBind, err))
		return reconcile.Result{}, err
	}

	r.record.Event(pr, event.Normal(reasonBind, fmt.Sprintf("Bound system ClusterRole %q to provider ServiceAccount(s): %s", n, strings.Join(subjectStrings, ", "))))

	// There's no need to requeue explicitly - we're watching all PRs.
	return reconcile.Result{Requeue: false}, nil
}

// ClusterRoleBindingsDiffer returns true if the supplied objects are different ClusterRoleBindings. We
// consider ClusterRoleBindings to be different if the subjects, the roleRefs, or the owner ref
// is different.
func ClusterRoleBindingsDiffer(current, desired runtime.Object) bool {
	// Calling this with anything but ClusterRoleBindings is a programming
	// error. If it happens, we probably do want to panic.
	c := current.(*rbacv1.ClusterRoleBinding) //nolint:forcetypeassert // See above.
	d := desired.(*rbacv1.ClusterRoleBinding) //nolint:forcetypeassert // See above.
	return !cmp.Equal(c.Subjects, d.Subjects) || !cmp.Equal(c.RoleRef, d.RoleRef) || !cmp.Equal(c.GetOwnerReferences(), d.GetOwnerReferences())
}
