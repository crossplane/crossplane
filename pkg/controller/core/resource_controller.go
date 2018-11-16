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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
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
	errorResourceClassNotDefined    = "Resource class is not provided"
	errorResourceProvisioning       = "Failed to provision new resource"
	errorResourceHandlerIsNotFound  = "Resource handler is not found"
	errorRetrievingResourceClass    = "Failed to retrieve resource class"
	errorRetrievingResourceInstance = "Failed to retrieve resource instance"
	errorRetrievingResourceSecret   = "Failed to retrieve resource secret"
	errorApplyingInstanceSecret     = "Failed to apply instance secret"
	errorSettingResourceBindStatus  = "Failed to set resource binding status"
	waitResourceIsNotAvailable      = "Waiting for resource to become available"
)

var (
	Result        = reconcile.Result{}
	ResultRequeue = reconcile.Result{Requeue: true}
	ctx           = context.Background()
)

// ResourceHandler defines resource handing functions
type ResourceHandler interface {
	Provision(*corev1alpha1.ResourceClass, corev1alpha1.AbstractResource, client.Client) (corev1alpha1.ConcreteResource, error)
	Find(types.NamespacedName, client.Client) (corev1alpha1.ConcreteResource, error)
	SetBindStatus(types.NamespacedName, client.Client, bool) error
}

// Reconciler reconciles an abstract resource
type Reconciler struct {
	client.Client
	scheme        *runtime.Scheme
	kubeclient    kubernetes.Interface
	recorder      record.EventRecorder
	finalizerName string
	handlers      map[string]ResourceHandler

	DoReconcile func(corev1alpha1.AbstractResource) (reconcile.Result, error)
	provision   func(corev1alpha1.AbstractResource) (reconcile.Result, error)
	bind        func(corev1alpha1.AbstractResource) (reconcile.Result, error)
	delete      func(corev1alpha1.AbstractResource) (reconcile.Result, error)
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

	return r
}

// _reconcile runs the main reconcile loop of this controller, given the requested instance
func (r *Reconciler) _reconcile(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
	// Check for deletion
	if instance.GetObjectMeta().DeletionTimestamp != nil && instance.ResourceStatus().Condition(corev1alpha1.Deleting) == nil {
		return r.delete(instance)
	}

	// Add finalizer
	if !util.HasFinalizer(instance.GetObjectMeta(), r.finalizerName) {
		util.AddFinalizer(instance.GetObjectMeta(), r.finalizerName)
		if err := r.Update(ctx, instance); err != nil {
			return ResultRequeue, err
		}
	}

	// check if instance reference is set, if not - create new instance
	if instance.ResourceRef() == nil {
		return r.provision(instance)
	}

	// bind to the resource
	return r.bind(instance)
}

// _provision based on class and parameters
func (r *Reconciler) _provision(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
	instance.ResourceStatus().SetUnbound()

	classRef := instance.ClassRef()
	if classRef == nil {
		// TODO: add support for default resource class, until then - fail
		return r.fail(instance, errorResourceClassNotDefined, "")
	}

	// retrieve classRef for this instance
	class := &corev1alpha1.ResourceClass{}
	if err := r.Get(ctx, util.NamespaceNameFromObjectRef(classRef), class); err != nil {
		return r.fail(instance, errorRetrievingResourceClass, err.Error())
	}

	// find handler for this class
	handler, ok := r.handlers[class.Provisioner]
	if !ok {
		// handler is not found - fail and do not requeue
		err := fmt.Errorf("handler [%s] is not defined", class.Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return Result, r.Update(ctx, instance)
	}

	// create new resource
	res, err := handler.Provision(class, instance, r.Client)
	if err != nil {
		return r.fail(instance, errorResourceProvisioning, err.Error())
	}

	// set resource reference to the newly created resource
	instance.SetResourceRef(res.ObjectReference())

	// set status values
	instance.ResourceStatus().Provisioner = class.Provisioner
	instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Creating, "", ""))

	// update instance
	return Result, r.Update(ctx, instance)
}

// _bind KubernetesCluster to a concrete Resource
func (r *Reconciler) _bind(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
	// retrieve finding function for this resource
	handler, ok := r.handlers[instance.ResourceStatus().Provisioner]
	if !ok {
		// finder function is not found, this condition should never happened
		// provisioner and finder should be added together for the same resource kind/version
		// fail and do not requeue
		err := fmt.Errorf("provisioner [%s] is not defined", instance.ResourceStatus().Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return Result, r.Update(ctx, instance)
	}

	// find resource instance
	resNName := util.NamespaceNameFromObjectRef(instance.ResourceRef())
	resource, err := handler.Find(resNName, r.Client)
	if err != nil {
		// failed to retrieve the resource - requeue
		return r.fail(instance, errorRetrievingResourceInstance, "")
	}

	// check for resource instance state and requeue if not running
	if !resource.IsAvailable() {
		instance.ResourceStatus().UnsetAllConditions()
		instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Pending, waitResourceIsNotAvailable, "Resource is not in running state"))
		return ResultRequeue, r.Update(ctx, instance)
	}

	// Object reference to the resource: needed to retrieve resource's namespace to retrieve resource's secret
	or := resource.ObjectReference()

	// retrieve resource's secret
	secret, err := r.kubeclient.CoreV1().Secrets(or.Namespace).Get(resource.ConnectionSecretName(), metav1.GetOptions{})
	if err != nil {
		return r.fail(instance, errorRetrievingResourceSecret, err.Error())
	}

	// replace secret metadata with the consumer's metadata (same as in service)
	secret.ObjectMeta = metav1.ObjectMeta{
		Namespace:       instance.GetObjectMeta().Namespace,
		Name:            instance.GetObjectMeta().Name,
		OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
	}
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, errorApplyingInstanceSecret, err.Error())
	}

	// update resource binding status
	if err := handler.SetBindStatus(resNName, r.Client, true); err != nil {
		return r.fail(instance, errorSettingResourceBindStatus, err.Error())
	}

	// set instance binding status
	instance.ResourceStatus().SetBound()

	// update conditions
	instance.ResourceStatus().UnsetAllConditions()
	instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", ""))

	return Result, r.Update(ctx, instance)
}

func (r *Reconciler) _delete(instance corev1alpha1.AbstractResource) (reconcile.Result, error) {
	// retrieve finding function for this resource
	handler, ok := r.handlers[instance.ResourceStatus().Provisioner]
	if !ok {
		// finder function is not found, this condition should never happened
		// provisioner and finder should be added together for the same resource kind/version
		// fail and do not requeue
		err := fmt.Errorf("provisioner [%s] is not defined", instance.ResourceStatus().Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return Result, r.Update(ctx, instance)
	}

	// update resource binding status
	resNName := util.NamespaceNameFromObjectRef(instance.ResourceRef())

	// TODO: decide how to handle resource binding status update error
	// - ignore the error for now
	handler.SetBindStatus(resNName, r.Client, false)

	// update instance status and remove finalizer
	instance.ResourceStatus().UnsetAllConditions()
	instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(instance.GetObjectMeta(), r.finalizerName)
	return reconcile.Result{}, r.Update(ctx, instance)
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance corev1alpha1.AbstractResource, reason, msg string) (reconcile.Result, error) {
	instance.ResourceStatus().SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return ResultRequeue, r.Update(ctx, instance)
}

// ResolveClassInstanceValues validates instance value against resource class properties.
// if both values are defined, then the instance value is validated against the resource class value and expected to match
// TODO: the "matching" process will be further refined once we implement constraint policies at the resource class level
func ResolveClassInstanceValues(classValue, instanceValue string) (string, error) {
	if classValue == "" {
		return instanceValue, nil
	}
	if instanceValue == "" {
		return classValue, nil
	}
	if classValue != instanceValue {
		return "", fmt.Errorf("mysql instance value [%s] does not match the one defined in the resource class [%s]", instanceValue, classValue)
	}
	return instanceValue, nil
}
