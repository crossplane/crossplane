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

	"github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/gcp"
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
	recorderName = "gcp.provider"
)

var _ reconcile.Reconciler = &Reconciler{}

// Add creates a new Provider Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, &ProviderValidator{}))
}

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
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudsql.gcp.conductor.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
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

	err = r.Validate(r.kubeclient, instance)
	if err != nil {
		instance.Status.SetInvalid("Invalid credentials", err.Error())
		return reconcile.Result{Requeue: true}, r.Update(ctx, instance)
	}

	instance.Status.SetValid("Valid")
	return reconcile.Result{}, r.Update(ctx, instance)
}

// Credentials - defines provider validation functions
type Validator interface {
	Validate(kubernetes.Interface, *v1alpha1.Provider) error
}

// CredentialsValidator - provides functionality for validating provider credentials
type ProviderValidator struct{}

// Validate GCP credentials secret
func (pv *ProviderValidator) Validate(k kubernetes.Interface, p *v1alpha1.Provider) error {
	// Retrieve credentials
	creds, err := gcp.ProviderCredentials(k, p)
	if err != nil {
		return err
	}

	if len(p.Spec.RequiredPermissions) == 0 {
		return nil
	}

	err = gcp.TestPermissions(creds, p.Spec.RequiredPermissions)
	if err != nil {
		return err
	}

	return nil
}
