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
	azureclient "github.com/crossplaneio/crossplane/pkg/clients/azure"
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
	errorCreatingClientRG      = "Failed to create Azure client"
	errorTestingClientRG       = "Failed testing Azure client"
	errorFetchingAzureProvider = "failed to fetch Azure Provider"
)

// ResourceGroupReconciler reconciles a Provider object
type ResourceGroupReconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	create func(*azureclient.Client, string, string) error
}

// newResourceGroupReconciler returns a new reconcile.Reconciler
func newResourceGroupReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &ResourceGroupReconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerNameRG),
	}
	r.create = r._create
	return r
}

// AddRG creates a new Resource Group Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddRG(mgr manager.Manager) error {
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

func (r *ResourceGroupReconciler) _create(client *azureclient.Client, name string, location string) error {
	return azureclient.CreateOrUpdateGroup(client, name, location)
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

	// look up the provider information for this instance
	provider := &v1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	if err := r.Get(ctx, providerNamespacedName, provider); err != nil {
		return r.fail(instance, errorFetchingAzureProvider, fmt.Sprintf("failed to get provider on behalf of resource group %+v: %+v", providerNamespacedName, err))
	}

	// Retrieve azure client from the given provider config
	azureClient, err := azureclient.NewClient(provider, r.kubeclient)
	if err != nil {
		return r.fail(instance, errorCreatingClientRG, err.Error())
	}

	// Create or Update ResourceGroup
	// TODO: Do we want to run the Create / Update on every reconciliation?
	if err := r.create(azureClient, instance.Spec.Name, instance.Spec.Location); err != nil {
		return r.fail(instance, errorCreatingClientRG, err.Error())
	}

	// Update status condition
	instance.Status.UnsetAllConditions()
	instance.Status.SetReady()

	return result, r.Update(ctx, instance)
}
