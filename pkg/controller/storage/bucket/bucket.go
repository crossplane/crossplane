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

package bucket

import (
	"context"
	"log"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	awsbucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

const (
	controllerName = "bucket.storage.crossplane.io"
	finalizer      = "finalizer." + controllerName
)

var (
	ctx = context.Background()

	// map of supported resource handlers
	handlers = map[string]corecontroller.ResourceHandler{
		awsbucketv1alpha1.S3BucketKindAPIVersion: &S3BucketHandler{},
	}
)

// Reconciler is the reconciler for Bucket objects
type Reconciler struct {
	*corecontroller.Reconciler
}

// Add creates a new Bucket Controller and adds it to the Manager with default RBAC.
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

	// Watch for changes to Bucket
	err = c.Watch(&source.Kind{Type: &bucketv1alpha1.Bucket{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", bucketv1alpha1.BucketKindApiVersion, request)

	// fetch the CRD instance
	instance := &bucketv1alpha1.Bucket{}
	if err := r.Get(ctx, request.NamespacedName, instance); err != nil {
		return corecontroller.HandleGetClaimError(err)
	}

	return r.DoReconcile(instance)
}
