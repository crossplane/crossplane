/*
Copyright 2018 The Crossplane Authors.

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

package core

import (
	"context"
	"fmt"
	"log"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	errorResourceClassNotDefined   = "Resource class is not provided"
	errorResourceProvisioning      = "Failed to provision new resource"
	errorResourceHandlerIsNotFound = "Resource handler is not found"
	errorRetrievingHandler         = "Failed to retrieve handler"
	errorRetrievingResourceClass   = "Failed to retrieve resource class"
	errorRetrievingResource        = "Failed to retrieve resource"
	errorRetrievingResourceSecret  = "Failed to retrieve resource secret"
	errorApplyingResourceSecret    = "Failed to apply resource secret"
	errorSettingResourceBindStatus = "Failed to set resource binding status"
	waitResourceIsNotAvailable     = "Waiting for resource to become available"
)

var (
	Result        = reconcile.Result{}
	ResultRequeue = reconcile.Result{Requeue: true}
	ctx           = context.Background()
)

// ResourceHandler defines resource handing functions
type ResourceHandler interface {
	Provision(*corev1alpha1.ResourceClass, corev1alpha1.ResourceClaim, client.Client) (corev1alpha1.Resource, error)
	Find(types.NamespacedName, client.Client) (corev1alpha1.Resource, error)
	SetBindStatus(types.NamespacedName, client.Client, bool) error
}

// Reconciler reconciles a resource claim
type Reconciler struct {
	client.Client
	scheme        *runtime.Scheme
	kubeclient    kubernetes.Interface
	recorder      record.EventRecorder
	finalizerName string
	handlers      map[string]ResourceHandler

	DoReconcile func(corev1alpha1.ResourceClaim) (reconcile.Result, error)
	provision   func(corev1alpha1.ResourceClaim, ResourceHandler) (reconcile.Result, error)
	bind        func(corev1alpha1.ResourceClaim, ResourceHandler) (reconcile.Result, error)
	delete      func(corev1alpha1.ResourceClaim, ResourceHandler) (reconcile.Result, error)
	getHandler  func(claim corev1alpha1.ResourceClaim) (ResourceHandler, error)
}

func NewReconciler(mgr manager.Manager, controllerName, finalizerName string, handlers map[string]ResourceHandler) *Reconciler {
	r := &Reconciler{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		kubeclient:    kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:      mgr.GetRecorder(controllerName),
		finalizerName: finalizerName,
		handlers:      handlers,
	}
	r.DoReconcile = r._reconcile
	r.provision = r._provision
	r.bind = r._bind
	r.delete = r._delete
	r.getHandler = r._getHandler

	return r
}

// _reconcile runs the main reconcile loop of this controller, given the requested claim
func (r *Reconciler) _reconcile(claim corev1alpha1.ResourceClaim) (reconcile.Result, error) {
	// get the resource handler for this claim
	handler, err := r.getHandler(claim)
	if err != nil {
		return r.fail(claim, errorRetrievingHandler, err.Error())
	} else if handler == nil {
		// handler is not found - log this but don't fail, let an external provisioner handle it
		log.Printf("handler for claim %s is unknown, ignoring reconcile to allow external provisioners to handle it", claim.GetObjectMeta().Name)
		return Result, nil
	}

	// Check for deletion
	if claim.GetObjectMeta().DeletionTimestamp != nil && claim.ClaimStatus().Condition(corev1alpha1.Deleting) == nil {
		return r.delete(claim, handler)
	}

	// Add finalizer
	if !util.HasFinalizer(claim.GetObjectMeta(), r.finalizerName) {
		util.AddFinalizer(claim.GetObjectMeta(), r.finalizerName)
		if err := r.Update(ctx, claim); err != nil {
			return ResultRequeue, err
		}
	}

	// check if claim reference is set, if not - provision new resource
	if claim.ResourceRef() == nil {
		return r.provision(claim, handler)
	}

	// bind to the resource
	return r.bind(claim, handler)
}

// _provision based on class and parameters
func (r *Reconciler) _provision(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
	// initialize the claim to an unbound state
	claimStatus := claim.ClaimStatus()
	claimStatus.SetUnbound()

	// get the resource class for this claim
	class, err := r.getResourceClass(claim)
	if err != nil {
		return r.fail(claim, errorRetrievingResourceClass, err.Error())
	}

	// create new resource
	res, err := handler.Provision(class, claim, r.Client)
	if err != nil {
		return r.fail(claim, errorResourceProvisioning, err.Error())
	}

	// set resource reference to the newly created resource
	claim.SetResourceRef(res.ObjectReference())

	// set status values
	claimStatus.Provisioner = class.Provisioner
	claimStatus.SetCreating()

	// update claim
	return Result, r.Update(ctx, claim)
}

// _bind KubernetesCluster to a concrete Resource
func (r *Reconciler) _bind(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
	// find resource instance
	resNName := util.NamespaceNameFromObjectRef(claim.ResourceRef())
	resource, err := handler.Find(resNName, r.Client)
	if err != nil {
		// failed to retrieve the resource - requeue
		return r.fail(claim, errorRetrievingResource, "")
	}

	// check for resource state and requeue if not running
	if !resource.IsAvailable() {
		claim.ClaimStatus().UnsetAllConditions()
		claim.ClaimStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Pending, waitResourceIsNotAvailable, "Resource is not in running state"))
		return ResultRequeue, r.Update(ctx, claim)
	}

	// Object reference to the resource: needed to retrieve resource's namespace to retrieve resource's secret
	or := resource.ObjectReference()

	// retrieve resource's secret
	secret, err := r.kubeclient.CoreV1().Secrets(or.Namespace).Get(resource.ConnectionSecretName(), metav1.GetOptions{})
	if err != nil {
		return r.fail(claim, errorRetrievingResourceSecret, err.Error())
	}

	// replace secret metadata with the consumer's metadata (same as in service)
	secret.ObjectMeta = metav1.ObjectMeta{
		Namespace:       claim.GetObjectMeta().Namespace,
		Name:            claim.GetObjectMeta().Name,
		OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
	}
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(claim, errorApplyingResourceSecret, err.Error())
	}

	// update resource binding status
	if err := handler.SetBindStatus(resNName, r.Client, true); err != nil {
		return r.fail(claim, errorSettingResourceBindStatus, err.Error())
	}

	// set instance binding status
	claimStatus := claim.ClaimStatus()
	claimStatus.SetBound()

	// update conditions
	if !claimStatus.IsReady() {
		claimStatus.UnsetAllConditions()
		claimStatus.SetReady()
	}

	return Result, r.Update(ctx, claim)
}

func (r *Reconciler) _delete(claim corev1alpha1.ResourceClaim, handler ResourceHandler) (reconcile.Result, error) {
	// update resource binding status
	resNName := util.NamespaceNameFromObjectRef(claim.ResourceRef())

	// TODO: decide how to handle resource binding status update error
	// - ignore the error for now
	_ = handler.SetBindStatus(resNName, r.Client, false)

	// update claim status and remove finalizer
	claimStatus := claim.ClaimStatus()
	claimStatus.UnsetAllConditions()
	claimStatus.SetDeleting()
	util.RemoveFinalizer(claim.GetObjectMeta(), r.finalizerName)
	return reconcile.Result{}, r.Update(ctx, claim)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(claim corev1alpha1.ResourceClaim, reason, msg string) (reconcile.Result, error) {
	claim.ClaimStatus().SetFailed(reason, msg)
	return ResultRequeue, r.Update(ctx, claim)
}

func (r *Reconciler) _getHandler(claim corev1alpha1.ResourceClaim) (ResourceHandler, error) {
	var provisioner string

	// first check if the claim already has the provisioner set on the resource status
	resourceStatus := claim.ClaimStatus()
	if resourceStatus != nil && resourceStatus.Provisioner != "" {
		provisioner = claim.ClaimStatus().Provisioner
	} else {
		// try looking up the provisioner through the claim's resource class
		class, err := r.getResourceClass(claim)
		if err != nil {
			return nil, err
		}

		provisioner = class.Provisioner
	}

	handler, ok := r.handlers[provisioner]
	if ok {
		// found the handler, return it now
		return handler, nil
	}

	// didn't find a known handler, but there wasn't any error on the way to figuring that out either
	return nil, nil
}

func (r *Reconciler) getResourceClass(claim corev1alpha1.ResourceClaim) (*corev1alpha1.ResourceClass, error) {
	classRef := claim.ClassRef()
	if classRef == nil {
		// TODO: add support for default resource class, until then - fail
		return nil, fmt.Errorf("resource claim does not reference a resource class")
	}

	// retrieve resource class for this claim
	class := &corev1alpha1.ResourceClass{}
	if err := r.Get(ctx, util.NamespaceNameFromObjectRef(classRef), class); err != nil {
		return nil, err
	}

	return class, nil
}

// ResolveClassClaimValues validates claim value against resource class properties.
// if both values are defined, then the claim value is validated against the resource class value and expected to match
// TODO: the "matching" process will be further refined once we implement constraint policies at the resource class level
func ResolveClassClaimValues(classValue, claimValue string) (string, error) {
	if classValue == "" {
		return claimValue, nil
	}
	if claimValue == "" {
		return classValue, nil
	}
	if classValue != claimValue {
		return "", fmt.Errorf("mysql claim value [%s] does not match the one defined in the resource class [%s]", claimValue, classValue)
	}
	return claimValue, nil
}
