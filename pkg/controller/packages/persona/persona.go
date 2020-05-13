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

package persona

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane/pkg/packages"
)

const (
	managedRolesLabel   = "rbac.crossplane.io/managed-roles"
	managedRolesEnabled = "true"

	reconcileTimeout = 1 * time.Minute

	loggerName = "packages/namespace-personas"

	adminPersona = "admin"
	editPersona  = "edit"
	viewPersona  = "view"

	errFailedToCreateClusterRole  = "failed to create clusterrole"
	errFailedToDeleteClusterRoles = "failed to delete clusterroles"
	errFailedToGetNamespace       = "failed to get namespace"

	logFailedToCreateDuringSync = "failed to create during sync"
	logFailedToDeleteDuringSync = "failed to delete during sync"
)

var (
	personas = []string{adminPersona, editPersona, viewPersona}

	resultRequeue = reconcile.Result{Requeue: true}
)

// Reconciler reconciles Namespaces
type Reconciler struct {
	kube client.Client
	log  logging.Logger
	factory
}

// Setup adds a controller that reconciles Namespaces.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	r := &Reconciler{
		kube:    mgr.GetClient(),
		factory: &nsPersonaHandlerFactory{},
		log:     l.WithValues("controller", loggerName),
	}

	// TODO(displague) Should we own the ClusterRole and watch the Namespace?
	// Not doing so means that changes to the ClusterRole won't be addressed by
	// this controller. OTOH, Permitting such changes keeps this controller
	// simple and gives users the flexibility to modify the clusterrole

	return ctrl.NewControllerManagedBy(mgr).
		Named(loggerName).
		For(&corev1.Namespace{}).
		Complete(r)
}

// Reconcile changes on Namespaces that may or may not have Packages managed RBAC
// labels.
//
// Reconcile gets the Namespace with the requested namespace name with the
// management enabling label. When the label is found, we create matching
// clusterroles.  When the label is not found, possibly removed, the
// namespace-persona clusterroles are deleted.
//
// When a namespace is deleted, the clusterroles will be removed through
// garbage-collection using OwnerReferences.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	r.log.Debug("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()
	ns := &corev1.Namespace{}

	if err := r.kube.Get(ctx, req.NamespacedName, ns); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		r.log.Debug(errFailedToGetNamespace, "request", req, "error", err)

		return reconcile.Result{}, err
	}

	handler := r.factory.newHandler(r.log, ns, r.kube)
	return handler.sync(ctx)
}

type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) error
	delete(context.Context) error
}

type nsPersonaHandler struct {
	kube client.Client
	ns   *corev1.Namespace
	log  logging.Logger
}

type factory interface {
	newHandler(logging.Logger, *corev1.Namespace, client.Client) handler
}

type nsPersonaHandlerFactory struct{}

func (f *nsPersonaHandlerFactory) newHandler(log logging.Logger, ns *corev1.Namespace, kube client.Client) handler {
	return &nsPersonaHandler{
		kube: kube,
		ns:   ns,
		log:  log,
	}
}

// sync compares the namespace being handled to the desired labels
// Matches warrant Clusterrole creation
// Non-matches warrant Clusterrole deletion
func (h *nsPersonaHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if nsHasPersonaManagement(h.ns) && !meta.WasDeleted(h.ns) {
		if err := h.create(ctx); err != nil {
			h.log.Debug(logFailedToCreateDuringSync, "namespace", h.ns.GetName(), "error", err)
			return resultRequeue, err
		}
	} else {
		if err := h.delete(ctx); err != nil {
			h.log.Debug(logFailedToDeleteDuringSync, "namespace", h.ns.GetName(), "error", err)
			return resultRequeue, err
		}
	}

	return reconcile.Result{}, nil
}

// generateNamespaceClusterRoles generates roles for a given namespace
// These clusterroles are named crossplane:ns:{nsName}:{persona}
func generateNamespaceClusterRoles(ns *corev1.Namespace) (roles []*rbacv1.ClusterRole) {
	nsName := ns.GetName()

	for _, persona := range personas {
		name := fmt.Sprintf(packages.NamespaceClusterRoleNameFmt, nsName, persona)

		labels := map[string]string{
			fmt.Sprintf(packages.LabelNamespaceFmt, nsName): "true",
			packages.LabelScope:                             packages.NamespaceScoped,
		}

		if persona == adminPersona {
			labels[fmt.Sprintf(packages.LabelAggregateFmt, "crossplane", persona)] = "true"
		}

		// By specifying MatchLabels, ClusterRole Aggregation will pass
		// along the rules from other ClusterRoles with the desired labels.
		// This is why we don't define any Rules here.
		role := &rbacv1.ClusterRole{
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							fmt.Sprintf(packages.LabelAggregateFmt, packages.NamespaceScoped, persona): "true",
							fmt.Sprintf(packages.LabelNamespaceFmt, nsName):                            "true",
						},
					},
					{
						MatchLabels: map[string]string{
							fmt.Sprintf(packages.LabelAggregateFmt, "namespace-default", persona): "true",
						},
					},
				},
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}

		// Parent labels enable more precise DeleteAllOf calls than matching
		// on clusterrole name
		meta.AddLabels(role, packages.ParentLabels(ns))

		roles = append(roles, role)
	}

	return roles
}

func nsHasPersonaManagement(ns *corev1.Namespace) bool {
	v, ok := ns.GetLabels()[managedRolesLabel]
	return ok && v == managedRolesEnabled
}

// create ClusterRoles for namespace personas
// example: crossplane:ns:{name}:{persona}
func (h *nsPersonaHandler) create(ctx context.Context) error {
	roles := generateNamespaceClusterRoles(h.ns)

	for _, role := range roles {
		// When the namespace is deleted, clusterroles are no longer needed.
		// Set the owner to the Namespace for garbage collection.
		role.SetOwnerReferences([]metav1.OwnerReference{
			meta.AsOwner(meta.ReferenceTo(h.ns, corev1.SchemeGroupVersion.WithKind("Namespace"))),
		})

		// Creating the clusterroles. Rules in these clusterroles are populated
		// through aggregation from the packages installed in the namespaces, we
		// won't need to update them.
		//
		// We are not patching existing clusterroles. This permits user
		// modification.
		h.log.Debug("Creating ClusterRole", "name", role.GetName())
		if err := h.kube.Create(ctx, role); err != nil && !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, errFailedToCreateClusterRole)
		}
	}
	return nil
}

// delete ClusterRoles for namespace personas
func (h *nsPersonaHandler) delete(ctx context.Context) error {
	// Logging that clusterroles are attempting to be deleted would
	// either be noisy (logged on unmanaged namespaces) or cost a
	// lookup for existing clusterroles

	labels := packages.ParentLabels(h.ns)
	if err := h.kube.DeleteAllOf(ctx, &rbacv1.ClusterRole{}, client.MatchingLabels(labels)); err != nil {
		return errors.Wrapf(err, errFailedToDeleteClusterRoles)
	}

	return nil
}
