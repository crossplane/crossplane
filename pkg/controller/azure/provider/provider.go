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

package provider

import (
	"context"
	"fmt"

	"github.com/upbound/conductor/pkg/apis/azure/v1alpha1"
	azureclient "github.com/upbound/conductor/pkg/clients/azure"
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
	recorderName        = "azure.provider"
	errorCreatingClient = "Failed to create Azure client"
	errorTestingClient  = "Failed testing Azure client"
)

// Add creates a new Provider Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, &ConfigurationValidator{}))
}

var _ reconcile.Reconciler = &Reconciler{}

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	Validator
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, validator Validator) reconcile.Reconciler {
	return &Reconciler{
		Client:     mgr.GetClient(),
		Validator:  validator,
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(recorderName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("instance-controller", mgr, controller.Options{Reconciler: r})
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
	client, err := azureclient.NewClient(instance, r.kubeclient)
	if err != nil {
		instance.Status.SetInvalid(errorCreatingClient, fmt.Sprintf("failed to get Azure client: %+v", err))
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	// Validate azure client
	if err := r.Validate(client); err != nil {
		instance.Status.SetInvalid(errorTestingClient, fmt.Sprintf("Azure client failed validation test: %+v", err))
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	instance.Status.SetValid("The Azure provider information is valid")
	return reconcile.Result{}, r.Update(ctx, instance)
}

// Validator - defines provider validation functions
type Validator interface {
	Validate(*azureclient.Client) error
}

// ConfigurationValidator - validates Azure client
type ConfigurationValidator struct{}

// Validate Azure client
func (pc *ConfigurationValidator) Validate(client *azureclient.Client) error {
	return azureclient.ValidateClient(client)
}
