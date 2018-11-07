/*
Copyright 2018 The Conductor Authors.

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

package mysql

import (
	"context"
	"fmt"

	awsdatabasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	mysqlv1alpha1 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws/rds"
	"github.com/upbound/conductor/pkg/util"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "rds-mysql-storage"
	finalizer      = "finalizer.mysql.storage.conductor.io"

	errorResourceClassNotDefined    = "Resource class is not provided"
	errorResourceProvisioning       = "Failed to _provision new resource"
	errorResourceHandlerIsNotFound  = "Resource handler is not found"
	errorRetrievingResourceClass    = "Failed to retrieve resource class"
	errorRetrievingResourceInstance = "Failed to retrieve resource instance"
	errorRetrievingResourceSecret   = "Failed to retrieve resource secret"
	errorApplyingInstanceSecret     = "Failed to apply instance secret"
	waitResourceIsNotAvailable      = "Waiting for resource to become available"
)

var (
	_   reconcile.Reconciler = &Reconciler{}
	ctx                      = context.Background()

	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}

	// map of supported resource handlers
	handlers = map[string]ResourceHandler{
		awsdatabasev1alpha1.RDSInstanceKindAPIVersion: &RDSInstanceHandler{},
	}
)

// Add creates a new Instance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	provision func(instance *mysqlv1alpha1.MySQLInstance) (reconcile.Result, error)
	bind      func(*mysqlv1alpha1.MySQLInstance) (reconcile.Result, error)
	delete    func(instance *mysqlv1alpha1.MySQLInstance) (reconcile.Result, error)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerName),
	}
	r.provision = r._provision
	r.bind = r._bind
	r.delete = r._delete
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Instance
	err = c.Watch(&source.Kind{Type: &mysqlv1alpha1.MySQLInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *mysqlv1alpha1.MySQLInstance, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return resultRequeue, r.Update(ctx, instance)
}

// _provision based on class and parameters
func (r *Reconciler) _provision(instance *mysqlv1alpha1.MySQLInstance) (reconcile.Result, error) {
	instance.Status.SetUnbound()

	classRef := instance.Spec.ClassRef
	if classRef == nil {
		// TODO: add support for default mysql resource class, until then - fail
		return r.fail(instance, errorResourceClassNotDefined, "")
	}

	// retrieve classRef for this instance
	class := &corev1alpha1.ResourceClass{}
	if err := r.Get(ctx, namespaceNameFromObjectRef(classRef), class); err != nil {
		return r.fail(instance, errorRetrievingResourceClass, err.Error())
	}

	// find handler for this class
	handler, ok := handlers[class.Provisioner]
	if !ok {
		// handler is not found - fail and do not requeue
		err := fmt.Errorf("handler [%s] is not defined", class.Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return result, r.Update(ctx, instance)
	}

	// create new resource
	res, err := handler.provision(class, instance, r.Client)
	if err != nil {
		return r.fail(instance, errorResourceProvisioning, err.Error())
	}

	// set resource reference to the newly created resource
	instance.Spec.ResourceRef = res.ObjectReference()

	// set status values
	instance.Status.Provisioner = class.Provisioner
	instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Creating, "", ""))

	// update instance
	return result, r.Update(ctx, instance)
}

// _bind MySQLInstance to a concrete Resource
func (r *Reconciler) _bind(instance *mysqlv1alpha1.MySQLInstance) (reconcile.Result, error) {
	// retrieve finding function for this resource
	handler, ok := handlers[instance.Status.Provisioner]
	if !ok {
		// finder function is not found, this condition should never happened
		// provisioner and finder should be added together for the same resource kind/version
		// fail and do not requeue
		err := fmt.Errorf("provisioner [%s] is not defined", instance.Status.Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return result, r.Update(ctx, instance)
	}

	// find resource instance
	resNName := namespaceNameFromObjectRef(instance.Spec.ResourceRef)
	resource, err := handler.find(resNName, r.Client)
	if err != nil {
		// failed to retrieve the resource - requeue
		return r.fail(instance, errorRetrievingResourceInstance, "")
	}

	// check for rds instance state and requeue if not running
	if !resource.IsAvailable() {
		instance.Status.UnsetAllConditions()
		instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Pending, waitResourceIsNotAvailable, "Resource is not in running state"))
		return resultRequeue, r.Update(ctx, instance)
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
		Namespace:       instance.Namespace,
		Name:            instance.Name,
		OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
	}
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return r.fail(instance, errorApplyingInstanceSecret, err.Error())
	}

	// update resource binding status
	handler.setBindStatus(resNName, r.Client, true)

	// set instance binding status
	instance.Status.SetBound()

	// update conditions
	instance.Status.UnsetAllConditions()
	instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Ready, "", ""))

	return result, r.Update(ctx, instance)
}

func (r *Reconciler) _delete(instance *mysqlv1alpha1.MySQLInstance) (reconcile.Result, error) {
	// retrieve finding function for this resource
	handler, ok := handlers[instance.Status.Provisioner]
	if !ok {
		// finder function is not found, this condition should never happened
		// provisioner and finder should be added together for the same resource kind/version
		// fail and do not requeue
		err := fmt.Errorf("provisioner [%s] is not defined", instance.Status.Provisioner)
		r.recorder.Event(instance, corev1.EventTypeWarning, "Fail", err.Error())
		instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Failed, errorResourceHandlerIsNotFound, err.Error()))
		return result, r.Update(ctx, instance)
	}

	// update resource binding status
	resNName := namespaceNameFromObjectRef(instance.Spec.ResourceRef)

	// TODO: decide how to handle resource binding status update error
	// - ignore the error for now
	handler.setBindStatus(resNName, r.Client, false)

	// update instance status and remove finalizer
	instance.Status.UnsetAllConditions()
	instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	return reconcile.Result{}, r.Update(ctx, instance)

}

// namespaceNameFromObjectRef helper function to create NamespacedName
func namespaceNameFromObjectRef(or *v1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{
		Namespace: or.Namespace,
		Name:      or.Name,
	}
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// fetch the CRD instance
	instance := &mysqlv1alpha1.MySQLInstance{}

	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return result, nil
		}
		return result, err
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil && instance.Status.Condition(corev1alpha1.Deleting) == nil {
		return r.delete(instance)
	}

	// Add finalizer
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return resultRequeue, err
		}
	}

	// check if instance reference is set, if not - create new instance
	if instance.Spec.ResourceRef == nil {
		return r.provision(instance)
	}

	// bind to the resource
	return r.bind(instance)
}

// ResourceHandler defines resource handing functions
type ResourceHandler interface {
	provision(*corev1alpha1.ResourceClass, *mysqlv1alpha1.MySQLInstance, client.Client) (corev1alpha1.Resource, error)
	find(types.NamespacedName, client.Client) (corev1alpha1.Resource, error)
	setBindStatus(types.NamespacedName, client.Client, bool) error
}

// RDSInstanceHandler handles RDS Instance functionality
type RDSInstanceHandler struct{}

// find RDSInstance
func (r *RDSInstanceHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	rdsInstance := &awsdatabasev1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	return rdsInstance, err
}

// provision create new RDSInstance
func (r *RDSInstanceHandler) provision(class *corev1alpha1.ResourceClass, instance *mysqlv1alpha1.MySQLInstance, c client.Client) (corev1alpha1.Resource, error) {
	// construct RDSInstance Spec from class definition
	rdsInstanceSpec := awsdatabasev1alpha1.NewRDSInstanceSpec(class.Parameters)

	// TODO: it is not clear if all concrete resource use the same constant value for database engine
	// if they do - we will need to refactor this value into constant.
	rdsInstanceSpec.Engine = "mysql"

	// translate mysql spec fields to RDSInstance
	rdsInstanceSpec.EngineVersion = instance.Spec.EngineVersion

	// assign provider reference and reclaim policy from the resource class
	rdsInstanceSpec.ProviderRef = class.ProviderRef
	rdsInstanceSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	rdsInstanceSpec.ClassRef = class.ObjectReference()
	rdsInstanceSpec.ClaimRef = instance.ObjectReference()

	// create and save RDSInstance
	rdsInstance := &awsdatabasev1alpha1.RDSInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: awsdatabasev1alpha1.APIVersion,
			Kind:       awsdatabasev1alpha1.RDSInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            util.GenerateName(instance.Name),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *rdsInstanceSpec,
	}

	err := c.Create(ctx, rdsInstance)
	return rdsInstance, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
func (r RDSInstanceHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	rdsInstance := &awsdatabasev1alpha1.RDSInstance{}
	err := c.Get(ctx, name, rdsInstance)
	if err != nil {
		// TODO:
		if rds.IsErrNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		rdsInstance.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		rdsInstance.Status.Phase = corev1alpha1.BindingStateBound
	}
	return c.Update(ctx, rdsInstance)
}
