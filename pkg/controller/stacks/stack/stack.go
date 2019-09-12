/*
Copyright 2019 The Crossplane Authors.

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

package stack

import (
	"context"
	"time"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

const (
	controllerName = "stack.stacks.crossplane.io"

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second
)

var (
	log              = logging.Logger.WithName(controllerName)
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	kube client.Client
	factory
}

// Controller is responsible for adding the Stack
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		kube:    mgr.GetClient(),
		factory: &stackHandlerFactory{},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.Stack{}).
		Complete(r)
}

// Reconcile reads that state of the Stack for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.StackKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	i := &v1alpha1.Stack{}
	if err := r.kube.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	handler := r.factory.newHandler(ctx, i, r.kube)

	return handler.sync(ctx)
}

type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) (reconcile.Result, error)
	update(context.Context) (reconcile.Result, error)
}

type stackHandler struct {
	kube client.Client
	ext  *v1alpha1.Stack
}

type factory interface {
	newHandler(context.Context, *v1alpha1.Stack, client.Client) handler
}

type stackHandlerFactory struct{}

func (f *stackHandlerFactory) newHandler(ctx context.Context, ext *v1alpha1.Stack, kube client.Client) handler {
	return &stackHandler{
		kube: kube,
		ext:  ext,
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *stackHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.ext.Status.ControllerRef == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

func (h *stackHandler) create(ctx context.Context) (reconcile.Result, error) {
	h.ext.Status.SetConditions(runtimev1alpha1.Creating())

	// create RBAC permissions
	if err := h.processRBAC(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// create controller deployment or job
	if err := h.processDeployment(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.processJob(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// the stack has successfully been created, the stack is ready
	h.ext.Status.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())
	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *stackHandler) update(ctx context.Context) (reconcile.Result, error) {
	log.V(logging.Debug).Info("updating not supported yet", "stack", h.ext.Name)
	return reconcile.Result{}, nil
}

func (h *stackHandler) createNamespacedRoleBinding(ctx context.Context, owner metav1.OwnerReference) error {
	cr := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Rules: h.ext.Spec.Permissions.Rules,
	}
	if err := h.kube.Create(ctx, cr); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create role")
	}

	// create rolebinding between service account and role
	crb := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		RoleRef: rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "Role", Name: h.ext.Name},
		Subjects: []rbac.Subject{
			{Name: h.ext.Name, Namespace: h.ext.Namespace, Kind: rbac.ServiceAccountKind},
		},
	}
	if err := h.kube.Create(ctx, crb); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create role binding")
	}
	return nil
}

func (h *stackHandler) createClusterRoleBinding(ctx context.Context, owner metav1.OwnerReference) error {
	cr := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Rules: h.ext.Spec.Permissions.Rules,
	}

	if err := h.kube.Create(ctx, cr); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create cluster role")
	}

	// create clusterrolebinding between service account and role
	crb := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		RoleRef: rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "ClusterRole", Name: h.ext.Name},
		Subjects: []rbac.Subject{
			{Name: h.ext.Name, Namespace: h.ext.Namespace, Kind: rbac.ServiceAccountKind},
		},
	}
	if err := h.kube.Create(ctx, crb); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create cluster role binding")
	}
	return nil
}

func (h *stackHandler) processRBAC(ctx context.Context) error {
	if len(h.ext.Spec.Permissions.Rules) == 0 {
		return nil
	}

	owner := meta.AsOwner(meta.ReferenceTo(h.ext, v1alpha1.StackGroupVersionKind))

	// create service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}

	if err := h.kube.Create(ctx, sa); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create service account")
	}

	switch apiextensions.ResourceScope(h.ext.Spec.PermissionScope) {
	case apiextensions.ClusterScoped:
		return h.createClusterRoleBinding(ctx, owner)
	case "", apiextensions.NamespaceScoped:
		return h.createNamespacedRoleBinding(ctx, owner)
	}

	return errors.New("invalid permissionScope for stack")
}

func (h *stackHandler) processDeployment(ctx context.Context) error {
	controllerDeployment := h.ext.Spec.Controller.Deployment
	if controllerDeployment == nil {
		return nil
	}

	// ensure the deployment is set to use this stack's service account that we created
	deploymentSpec := *controllerDeployment.Spec.DeepCopy()
	deploymentSpec.Template.Spec.ServiceAccountName = h.ext.Name

	ref := meta.AsOwner(meta.ReferenceTo(h.ext, v1alpha1.StackGroupVersionKind))
	gvk := schema.GroupVersionKind{
		Group:   apps.GroupName,
		Kind:    "Deployment",
		Version: apps.SchemeGroupVersion.Version,
	}
	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            controllerDeployment.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: deploymentSpec,
	}

	if err := h.kube.Create(ctx, d); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create deployment")
	}

	// save a reference to the stack's controller
	h.ext.Status.ControllerRef = meta.ReferenceTo(d, gvk)

	return nil
}

func (h *stackHandler) processJob(ctx context.Context) error {
	controllerJob := h.ext.Spec.Controller.Job
	if controllerJob == nil {
		return nil
	}

	// ensure the job is set to use this stack's service account that we created
	jobSpec := *controllerJob.Spec.DeepCopy()
	jobSpec.Template.Spec.ServiceAccountName = h.ext.Name

	ref := meta.AsOwner(meta.ReferenceTo(h.ext, v1alpha1.StackGroupVersionKind))
	gvk := schema.GroupVersionKind{
		Group:   batch.GroupName,
		Kind:    "Job",
		Version: batch.SchemeGroupVersion.Version,
	}
	j := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            controllerJob.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: jobSpec,
	}
	if err := h.kube.Create(ctx, j); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create job")
	}

	// save a reference to the stack's controller
	h.ext.Status.ControllerRef = meta.ReferenceTo(j, gvk)

	return nil
}

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i *v1alpha1.Stack, err error) (reconcile.Result, error) {
	log.V(logging.Debug).Info("failed stack", "i", i.Name, "error", err)
	i.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
