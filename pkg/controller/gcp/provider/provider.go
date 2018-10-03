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

	"github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Add creates a new Provider Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, &CredentialsValidator{}))
}

var _ reconcile.Reconciler = &ReconcileProvider{}

// ReconcileProvider reconciles a Provider object
type ReconcileProvider struct {
	client.Client
	Validator
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, validator Validator) reconcile.Reconciler {
	return &ReconcileProvider{
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
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudsql.gcp.conductor.io,resources=instances,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileProvider) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	// Fetch Provider Secret
	secret, err := r.kubeclient.CoreV1().Secrets(request.Namespace).Get(instance.Spec.SecretKey.Name, metav1.GetOptions{})
	if err != nil {
		r.recorder.Event(instance, corev1.EventTypeWarning, "Error", err.Error())
		return reconcile.Result{}, err
	}

	// Retrieve credentials.json
	data, ok := secret.Data[instance.Spec.SecretKey.Key]
	if !ok {
		instance.Status.SetInvalid(fmt.Sprintf("invalid GCP Provider secret, %s data is not found", instance.Spec.SecretKey.Key), "")
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	// Validate credentials
	if err := r.Validate(data, instance.Spec.RequiredPermissions, instance.Spec.ProjectID); err != nil {
		instance.Status.SetInvalid(err.Error(), "")
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	instance.Status.SetValid("Valid")
	return reconcile.Result{}, r.Update(ctx, instance)
}
