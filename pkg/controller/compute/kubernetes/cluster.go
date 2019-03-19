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

package kubernetes

import (
	"context"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	awscomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	azurecomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	gcpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

const (
	controllerName = "kubernetes.compute.crossplane.io"
	finalizer      = "finalizer." + controllerName
)

var (
	ctx = context.Background()

	// map of supported resource handlers
	handlers = map[string]corecontroller.ResourceHandler{
		gcpcomputev1alpha1.GKEClusterKindAPIVersion:   &GKEClusterHandler{},
		awscomputev1alpha1.EKSClusterKindAPIVersion:   &AWSClusterHandler{},
		azurecomputev1alpha1.AKSClusterKindAPIVersion: &AKSClusterHandler{},
	}
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	*corecontroller.Reconciler
}

// Add creates a new KubernetesCluster Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Reconciler: corecontroller.NewReconciler(mgr, controllerName, finalizer, handlers),
	}
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to KubernetesCluster
	err = c.Watch(&source.Kind{Type: &computev1alpha1.KubernetesCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", computev1alpha1.KubernetesInstanceKindAPIVersion, request)

	// fetch the CRD instance
	instance := &computev1alpha1.KubernetesCluster{}
	if err := r.Get(ctx, request.NamespacedName, instance); err != nil {
		return corecontroller.HandleGetClaimError(err)
	}

	return r.DoReconcile(instance)
}
