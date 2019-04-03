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

package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclient "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
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
	controllerNameRG           = "azure.resourcegroup"
	finalizer                  = "finalizer." + controllerName
	errorCreatingClientRG      = "Failed to create Azure client"
	errorDeletingClientRG      = "Failed to delete Azure resource group client"
	errorSyncResourceGroup     = "Failed to sync Azure resource group"
	errorTestingClientRG       = "Failed testing Azure client"
	errorFetchingAzureProvider = "failed to fetch Azure Provider"
)

// ResourceGroupReconciler reconciles a Provider object
type ResourceGroupReconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	connect func(*v1alpha1.ResourceGroup) (*azureclient.Client, error)
	create  func(*v1alpha1.ResourceGroup, *azureclient.Client) (reconcile.Result, error)
	sync    func(*v1alpha1.ResourceGroup, *azureclient.Client) (reconcile.Result, error)
	delete  func(*v1alpha1.ResourceGroup, *azureclient.Client) (reconcile.Result, error)
}

// newResourceGroupReconciler returns a new reconcile.Reconciler
func newResourceGroupReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ResourceGroupReconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerNameRG),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete
	return r
}

// AddResourceGroup creates a new Resource Group Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddResourceGroup(mgr manager.Manager) error {
	return addRG(mgr, newResourceGroupReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func addRG(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerNameRG, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &v1alpha1.ResourceGroup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *ResourceGroupReconciler) fail(instance *v1alpha1.ResourceGroup, reason, msg string) (reconcile.Result, error) {
	instance.Status.UnsetAllConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *ResourceGroupReconciler) _connect(instance *v1alpha1.ResourceGroup) (*azureclient.Client, error) {
	// Fetch Provider
	provider := &v1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	if err := r.Get(ctx, providerNamespacedName, provider); err != nil {
		return nil, fmt.Errorf(errorFetchingAzureProvider)
	}

	return azureclient.NewClient(provider, r.kubeclient)
}

func (r *ResourceGroupReconciler) _create(instance *v1alpha1.ResourceGroup, client *azureclient.Client) (reconcile.Result, error) {
	if err := azureclient.CreateOrUpdateGroup(client, instance.Spec.Name, instance.Spec.Location); err != nil {
		return r.fail(instance, errorCreatingClientRG, err.Error())
	}

	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()
	instance.Status.Name = instance.Spec.Name

	return resultRequeue, r.Update(ctx, instance)
}

func (r *ResourceGroupReconciler) _sync(instance *v1alpha1.ResourceGroup, client *azureclient.Client) (reconcile.Result, error) {
	if exists, err := azureclient.CheckExistence(client, instance.Spec.Name, instance.Spec.Location); err != nil || exists == false {
		return r.fail(instance, errorSyncResourceGroup, err.Error())
	}

	if !instance.Status.IsReady() {
		instance.Status.UnsetAllConditions()
		instance.Status.SetReady()
	}

	return result, r.Update(ctx, instance)
}

func (r *ResourceGroupReconciler) _delete(instance *v1alpha1.ResourceGroup, client *azureclient.Client) (reconcile.Result, error) {
	deleteFuture, err := azureclient.DeleteGroup(client, instance.Spec.Name, instance.Spec.Name)
	if err != nil && !azureclient.IsNotFound(err) {
		return r.fail(instance, errorDeletingClientRG, fmt.Sprintf("failed to delete resource group %s: %+v", instance.Name, err))
	}
	deleteFutureJSON, _ := deleteFuture.MarshalJSON()
	log.Printf("started delete of resource group %s, operation: %s", instance.Name, string(deleteFutureJSON))

	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.UnsetAllConditions()
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a ResourceGroup object and makes changes based on the state read
// and what is in the ResourceGroup.Spec
func (r *ResourceGroupReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", v1alpha1.ProviderKindAPIVersion, request)
	instance := &v1alpha1.ResourceGroup{}

	// Fetch the ResourceGroup instance
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Printf("failed to get object at start of reconcile loop: %+v", err)
		return reconcile.Result{}, err
	}

	// Retrieve azure client from the given provider config
	azureClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorCreatingClientRG, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		if instance.Status.Condition(corev1alpha1.Deleting) == nil {
			// we haven't started the deletion of the resource group yet, do it now
			log.Printf("resource group %s has been deleted, running finalizer now", instance.Name)
			return r.delete(instance, azureClient)
		}
		// we already started the deletion of the resource group, nothing more to do
		return result, nil
	}

	// Add finalizer
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return resultRequeue, err
		}
	}

	if instance.Status.Name == "" {
		return r.create(instance, azureClient)
	}

	return r.sync(instance, azureClient)
}
