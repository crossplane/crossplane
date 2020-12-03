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

package binding

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/controller/rbac/provider/roles"
)

const (
	shortWait = 30 * time.Second

	timeout        = 2 * time.Minute
	maxConcurrency = 5

	errGetPR        = "cannot get ProviderRevision"
	errListSAs      = "cannot list ServiceAccounts"
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
func Setup(mgr ctrl.Manager, log logging.Logger) error {
	name := "rbac/" + strings.ToLower(v1beta1.ProviderRevisionGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1beta1.ProviderRevision{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Watches(&source.Kind{Type: &corev1.ServiceAccount{}}, &handler.EnqueueRequestForOwner{OwnerType: &v1beta1.ProviderRevision{}}).
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
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {

	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pr := &v1beta1.ProviderRevision{}
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

	l := &corev1.ServiceAccountList{}
	if err := r.client.List(ctx, l); err != nil {
		log.Debug(errListSAs, "error", err)
		r.record.Event(pr, event.Warning(reasonBind, errors.Wrap(err, errListSAs)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}

	// Filter down to the ServiceAccounts that are owned by this
	// ProviderRevision. Each revision should control at most one, but it's easy
	// and relatively harmless for us to handle there being many.
	subjects := make([]rbacv1.Subject, 0)
	for _, sa := range l.Items {
		for _, ref := range sa.GetOwnerReferences() {
			if ref.UID == pr.GetUID() {
				subjects = append(subjects, rbacv1.Subject{
					Kind:      rbacv1.ServiceAccountKind,
					Namespace: sa.GetNamespace(),
					Name:      sa.GetName(),
				})
			}
		}
	}

	n := roles.SystemClusterRoleName(pr.GetName())
	ref := meta.AsController(meta.TypedReferenceTo(pr, v1beta1.ProviderRevisionGroupVersionKind))
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

	if err := r.client.Apply(ctx, rb, resource.MustBeControllableBy(pr.GetUID())); err != nil {
		log.Debug(errApplyBinding, "error", err)
		r.record.Event(pr, event.Warning(reasonBind, errors.Wrap(err, errApplyBinding)))
		return reconcile.Result{RequeueAfter: shortWait}, nil
	}
	log.Debug("Applied system ClusterRoleBinding")
	r.record.Event(pr, event.Normal(reasonBind, "Bound system ClusterRole to provider ServiceAccount(s)"))

	// There's no need to requeue explicitly - we're watching all PRs.
	return reconcile.Result{Requeue: false}, nil
}
