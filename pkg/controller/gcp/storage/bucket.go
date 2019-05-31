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
	"reflect"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	gcpstorage "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage"
	"github.com/crossplaneio/crossplane/pkg/logging"
)

const (
	controllerName = "bucket.storage.gcp.crossplane.io"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 30 * time.Second

	// failed condition reasons
	//
	// failed to get resource handler
	failedToGetHandler = "error getting handler"
	// failed to delete bucket resource
	failedToDelete = "error deleting"
	// failed to retrieve bucket resource
	failedToRetrieve = "error retrieving"
	// failed to create bucket resource
	failedToCreate = "error creating"
	// failed to update bucket resource
	failedToUpdate = "error updating"
	// failed to save connection secret
	failedToUpdateSecret = "error updating connection secret"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}

	log = logging.Logger.WithName("controller." + controllerName)
)

// Reconciler reconciles a GCP storage bucket bucket
type Reconciler struct {
	client.Client
	factory
}

// Add creates a newSyncDeleter Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:  mgr.GetClient(),
		factory: &bucketFactory{mgr.GetClient()},
	}

	// Create a newSyncDeleter controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to bucket
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Bucket{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to Instance Secret
	return c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Bucket{},
	})
}

// Reconcile reads that state of the cluster for a Provider bucket and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.BucketKindAPIVersion, "request", request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	b := &v1alpha1.Bucket{}
	if err := r.Get(ctx, request.NamespacedName, b); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	bh, err := r.newSyncDeleter(ctx, b)
	if err != nil {
		b.Status.SetFailed(failedToGetHandler, err.Error())
		return resultRequeue, r.Status().Update(ctx, b)
	}

	// Check for deletion
	if b.DeletionTimestamp != nil {
		return bh.delete(ctx)
	}

	return bh.sync(ctx)
}

type factory interface {
	newSyncDeleter(context.Context, *v1alpha1.Bucket) (syncdeleter, error)
}

type bucketFactory struct {
	client.Client
}

func (m *bucketFactory) newSyncDeleter(ctx context.Context, b *v1alpha1.Bucket) (syncdeleter, error) {
	p := &gcpv1alpha1.Provider{}
	n := types.NamespacedName{Namespace: b.GetNamespace(), Name: b.Spec.ProviderRef.Name}
	if err := m.Get(ctx, n, p); err != nil {
		return nil, err
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	if err := m.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider's secret %s", n)
	}

	creds, err := google.CredentialsFromJSON(context.Background(), s.Data[p.Spec.Secret.Key], storage.ScopeFullControl)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot retrieve creds from json")
	}

	sc, err := storage.NewClient(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, errors.Wrapf(err, "error creating storage client")
	}

	ops := &bucketHandler{
		Bucket: b,
		gcp:    &gcpstorage.BucketClient{BucketHandle: sc.Bucket(b.GetBucketName())},
		kube:   m.Client,
	}

	return &bucketSyncDeleter{
		operations:    ops,
		createupdater: &bucketCreateUpdater{operations: ops, projectID: creds.ProjectID},
	}, nil

}

type syncdeleter interface {
	delete(context.Context) (reconcile.Result, error)
	sync(context.Context) (reconcile.Result, error)
}

type bucketSyncDeleter struct {
	operations
	createupdater
}

func newBucketSyncDeleter(ops operations, projectID string) *bucketSyncDeleter {
	return &bucketSyncDeleter{
		operations:    ops,
		createupdater: newBucketCreateUpdater(ops, projectID),
	}
}

func (bh *bucketSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	if bh.isReclaimDelete() {
		if err := bh.deleteBucket(ctx); err != nil && err != storage.ErrBucketNotExist {
			return resultRequeue, bh.failReconcile(ctx, failedToDelete, err.Error())
		}
	}
	bh.removeFinalizer()
	return reconcile.Result{}, bh.updateObject(ctx)
}

// sync - synchronizes the state of the bucket resource with the state of the
// bucket Kubernetes bucket
func (bh *bucketSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	if err := bh.updateSecret(ctx); err != nil {
		return resultRequeue, bh.failReconcile(ctx, failedToUpdateSecret, err.Error())
	}

	attrs, err := bh.getAttributes(ctx)
	if err != nil && err != storage.ErrBucketNotExist {
		return resultRequeue, bh.failReconcile(ctx, failedToRetrieve, err.Error())
	}

	if attrs == nil {
		return bh.create(ctx)
	}

	return bh.update(ctx, attrs)
}

// createupdater interface defining create and update operations on/for bucket resource
type createupdater interface {
	create(context.Context) (reconcile.Result, error)
	update(context.Context, *storage.BucketAttrs) (reconcile.Result, error)
}

// bucketCreateUpdater implementation of createupdater interface
type bucketCreateUpdater struct {
	operations
	projectID string
}

// newBucketCreateUpdater new instance of bucketCreateUpdater
func newBucketCreateUpdater(ops operations, pID string) *bucketCreateUpdater {
	return &bucketCreateUpdater{
		operations: ops,
		projectID:  pID,
	}
}

// create new bucket resource and save changes back to bucket specs
func (bh *bucketCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	bh.addFinalizer()

	if err := bh.createBucket(ctx, bh.projectID); err != nil {
		return resultRequeue, bh.failReconcile(ctx, failedToCreate, err.Error())
	}

	bh.setReady()

	attrs, err := bh.getAttributes(ctx)
	if err != nil {
		return resultRequeue, bh.failReconcile(ctx, failedToRetrieve, err.Error())
	}

	bh.setSpecAttrs(attrs)

	if err := bh.updateObject(ctx); err != nil {
		return resultRequeue, err
	}

	bh.setStatusAttrs(attrs)

	return requeueOnSuccess, bh.updateStatus(ctx)
}

// update bucket resource if needed
func (bh *bucketCreateUpdater) update(ctx context.Context, attrs *storage.BucketAttrs) (reconcile.Result, error) {
	current := v1alpha1.NewBucketUpdatableAttrs(attrs)
	if reflect.DeepEqual(*current, bh.getSpecAttrs()) {
		return requeueOnSuccess, nil
	}

	attrs, err := bh.updateBucket(ctx, attrs.Labels)
	if err != nil {
		return resultRequeue, bh.failReconcile(ctx, failedToUpdate, err.Error())
	}

	// Sync attributes back to spec
	bh.setSpecAttrs(attrs)
	if err := bh.updateObject(ctx); err != nil {
		return resultRequeue, err
	}

	return requeueOnSuccess, bh.updateStatus(ctx)
}
