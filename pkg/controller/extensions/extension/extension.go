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

package extension

import (
	"context"
	"fmt"
	"time"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	controllerHandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/extensions/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	controllerName = "extension.extensions.crossplane.io"

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second

	reasonCreatingRBAC       = "failed to create RBAC"
	reasonCreatingDeployment = "failed to create deployment"
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

// Add creates a new Extension Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		kube:    mgr.GetClient(),
		factory: &extensionHandlerFactory{},
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Extension
	err = c.Watch(&source.Kind{Type: &v1alpha1.Extension{}}, &controllerHandler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the Extension for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ExtensionKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	i := &v1alpha1.Extension{}
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

type extensionHandler struct {
	kube client.Client
	i    *v1alpha1.Extension
}

type factory interface {
	newHandler(context.Context, *v1alpha1.Extension, client.Client) handler
}

type extensionHandlerFactory struct{}

func (f *extensionHandlerFactory) newHandler(ctx context.Context, i *v1alpha1.Extension, kube client.Client) handler {
	return &extensionHandler{
		kube: kube,
		i:    i,
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *extensionHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.i.Status.ControllerRef == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

func (h *extensionHandler) create(ctx context.Context) (reconcile.Result, error) {
	// create RBAC permissions
	if err := h.processRBAC(ctx); err != nil {
		return fail(ctx, h.kube, h.i, reasonCreatingRBAC, err.Error())
	}

	// create controller deployment
	if err := h.processDeployment(ctx); err != nil {
		return fail(ctx, h.kube, h.i, reasonCreatingDeployment, err.Error())
	}

	// the extension has successfully been created, the extension is ready
	h.i.Status.UnsetAllConditions()
	h.i.Status.SetReady()
	return requeueOnSuccess, h.kube.Status().Update(ctx, h.i)
}

func (h *extensionHandler) update(ctx context.Context) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (h *extensionHandler) processRBAC(ctx context.Context) error {
	if len(h.i.Spec.Permissions.Rules) == 0 {
		return nil
	}

	// create service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.i.Name,
			Namespace:       h.i.Namespace,
			OwnerReferences: []metav1.OwnerReference{h.i.OwnerReference()},
		},
	}
	if err := h.kube.Create(ctx, sa); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service account: %+v", err)
	}

	// create role
	cr := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.i.Name,
			OwnerReferences: []metav1.OwnerReference{h.i.OwnerReference()},
		},
		Rules: h.i.Spec.Permissions.Rules,
	}
	if err := h.kube.Create(ctx, cr); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create cluster role: %+v", err)
	}

	// create rolebinding between service account and role
	crb := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.i.Name,
			OwnerReferences: []metav1.OwnerReference{h.i.OwnerReference()},
		},
		RoleRef: rbac.RoleRef{APIGroup: rbac.GroupName, Kind: "ClusterRole", Name: h.i.Name},
		Subjects: []rbac.Subject{
			{Name: h.i.Name, Namespace: h.i.Namespace, Kind: rbac.ServiceAccountKind},
		},
	}
	if err := h.kube.Create(ctx, crb); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create cluster role binding: %+v", err)
	}

	return nil
}

func (h *extensionHandler) processDeployment(ctx context.Context) error {
	controllerDeployment := h.i.Spec.Controller.Deployment
	if controllerDeployment == nil {
		return nil
	}

	// ensure the deployment is set to use this extension's service account that we created
	deploymentSpec := *controllerDeployment.Spec.DeepCopy()
	deploymentSpec.Template.Spec.ServiceAccountName = h.i.Name

	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            controllerDeployment.Name,
			Namespace:       h.i.Namespace,
			OwnerReferences: []metav1.OwnerReference{h.i.OwnerReference()},
		},
		Spec: deploymentSpec,
	}
	if err := h.kube.Create(ctx, d); err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create deployment: %+v", err)
	}

	// save a reference to the extension's controller
	h.i.Status.ControllerRef = &corev1.ObjectReference{
		APIVersion: d.APIVersion,
		Kind:       d.Kind,
		Name:       d.Name,
		Namespace:  d.Namespace,
		UID:        d.ObjectMeta.UID,
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i *v1alpha1.Extension, reason, msg string) (reconcile.Result, error) {
	log.V(logging.Debug).Info("failed extension", "i", i.Name, "reason", reason, "message", msg)
	i.Status.SetFailed(reason, msg)
	i.Status.UnsetCondition(corev1alpha1.Ready)
	return resultRequeue, kube.Status().Update(ctx, i)
}
