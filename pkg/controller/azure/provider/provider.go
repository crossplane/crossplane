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

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	azureclient "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	controllerName      = "azure.provider"
	errorCreatingClient = "Failed to create Azure client"
	errorTestingClient  = "Failed testing Azure client"
)

var (
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Add creates a new Provider Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	validate func(*azureclient.Client) error
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerName),
	}
	r.validate = r._validate
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &v1alpha1.Provider{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *v1alpha1.Provider, reason, msg string) (reconcile.Result, error) {
	instance.Status.UnsetAllConditions()
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *Reconciler) _validate(client *azureclient.Client) error {
	return azureclient.ValidateClient(client)
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	// Fetch the Provider instance
	instance := &v1alpha1.Provider{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Retrieve azure client from the given provider config
	azureClient, err := azureclient.NewClient(instance, r.kubeclient)
	if err != nil {
		return r.fail(instance, errorCreatingClient, err.Error())
	}

	// Validate azure client
	if err := r.validate(azureClient); err != nil {
		return r.fail(instance, errorTestingClient, err.Error())
	}

	if instance.Status.IsReady() {
		return result, nil
	}

	// Update status condition
	instance.Status.UnsetAllConditions()
	instance.Status.SetReady()

	return result, r.Update(ctx, instance)
}
