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

package storage

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	v1alpha13 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"

	storage2 "google.golang.org/api/storage/v1"

	"cloud.google.com/go/storage"
	gcpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	v1alpha12 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/util"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/api/errors"
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
	controllerName = "bucket.storage.gcp.crossplane.io"
	finalizer      = "finalizer." + controllerName
	namePrefix     = "b-"

	errorClusterClient = "Failed to create cluster client"
	errorCreateCluster = "Failed to create new cluster"
	errorSyncCluster   = "Failed to sync cluster state"
	errorDeleteCluster = "Failed to delete cluster"

	reconcileTimeout = 1 * time.Minute
)

var (
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder
}

// Add creates a new Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerName),
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &gcpcomputev1alpha1.GKECluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", gcpcomputev1alpha1.GKEClusterKindAPIVersion, request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	instance := &v1alpha1.Bucket{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	p := &v1alpha12.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err = r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return resultRequeue, err
	}

	// Check provider status
	if !p.IsValid() {
		return resultRequeue, fmt.Errorf("provider status is invalid")
	}

	creds, err := gcp.ProviderCredentials(r.kubeclient, p, storage2.CloudPlatformScope, storage2.DevstorageFullControlScope)
	if err != nil {
		return resultRequeue, err
	}

	client, err := storage.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		return resultRequeue, err
	}

	bucket := client.Bucket("name")

	attrs, err := bucket.Attrs(ctx)
	if err != nil && !gcp.IsErrorNotFound(err) {
		return resultRequeue, err
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		if attrs != nil && instance.Spec.ReclaimPolicy == v1alpha13.ReclaimDelete {
			if err := bucket.Delete(ctx); err != nil {
				return resultRequeue, err
			}
		}
		util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
		return result, r.Update(ctx, instance)
	}

	// Check if need to create
	if attrs == nil {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		return resultRequeue, bucket.Create(ctx, creds.ProjectID, v1alpha1.CopyBucketSpecAttrs(instance.Spec.BucketSpecAttrs))
	}

	// Check if need to update
	if current := v1alpha1.NewBucketUpdateAttrs(attrs); reflect.DeepEqual(current, instance.Spec.BucketUpdatableAttrs) {
		bucket.Update(ctx, *v1alpha1.CopyToBucketUpdateAttrs(instance.Spec.BucketUpdatableAttrs))
	}
}
