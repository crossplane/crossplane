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
	"log"
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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	gcpstorage "github.com/crossplaneio/crossplane/pkg/clients/gcp/storage"
	"github.com/crossplaneio/crossplane/pkg/util"
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
	failedToSaveSecret = "error saving connection secret"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// Reconciler reconciles a GCP storage bucket obj
type Reconciler struct {
	client.Client
	factory
}

// Add creates a newHandler Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:  mgr.GetClient(),
		factory: &bucketFactory{mgr.GetClient()},
	}

	// Create a newHandler controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	return c.Watch(&source.Kind{Type: &v1alpha1.Bucket{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile reads that state of the cluster for a Provider obj and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", v1alpha1.BucketKindAPIVersion, request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	b := &v1alpha1.Bucket{}
	if err := r.Get(ctx, request.NamespacedName, b); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	bh, err := r.newHandler(ctx, b)
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
	newHandler(context.Context, *v1alpha1.Bucket) (syncdeleter, error)
}

type bucketFactory struct {
	client.Client
}

func (m *bucketFactory) newHandler(ctx context.Context, b *v1alpha1.Bucket) (syncdeleter, error) {
	p := &gcpv1alpha1.Provider{}
	n := types.NamespacedName{Namespace: b.GetNamespace(), Name: b.Spec.ProviderRef.Name}
	if err := m.Get(ctx, n, p); err != nil {
		return nil, err
	}

	// Check provider status
	if !p.IsValid() {
		return nil, errors.Errorf("provider: %s is not ready", n)
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
		return nil, errors.Wrapf(err, "error creating storage cc")
	}

	return newBucketSyncDeleter(&gcpstorage.BucketClient{BucketHandle: sc.Bucket(string(b.GetUID()))}, m.Client, b, creds.ProjectID), nil
}

type deleter interface {
	delete(context.Context) (reconcile.Result, error)
}

type syncer interface {
	sync(context.Context) (reconcile.Result, error)
}

type creater interface {
	create(context.Context) (reconcile.Result, error)
}

type updater interface {
	update(context.Context, *storage.BucketAttrs) (reconcile.Result, error)
}

type syncdeleter interface {
	deleter
	syncer
}

type bucketSyncDeleter struct {
	createupdater
	gcpstorage.Client
	kube   client.Client
	object *v1alpha1.Bucket
}

func newBucketSyncDeleter(sc gcpstorage.Client, cc client.Client, b *v1alpha1.Bucket, projectID string) *bucketSyncDeleter {
	return &bucketSyncDeleter{
		createupdater: newBucketCreateUpdater(sc, cc, b, projectID),
		Client:        sc,
		kube:          cc,
		object:        b,
	}
}

func (bh *bucketSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	if bh.object.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := bh.Delete(ctx); err != nil && err != storage.ErrBucketNotExist {
			bh.object.Status.SetFailed(failedToDelete, err.Error())
			return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
		}
	}
	util.RemoveFinalizer(&bh.object.ObjectMeta, finalizer)
	return reconcile.Result{}, bh.kube.Update(ctx, bh.object)
}

// sync - synchronizes the state of the bucket resource with the state of the
// bucket Kubernetes obj
func (bh *bucketSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	// create connection secret if it doesn't exist
	if err := bh.kube.Create(ctx, bh.object.ConnectionSecret()); err != nil && !kerrors.IsAlreadyExists(err) {
		bh.object.Status.SetFailed(failedToSaveSecret, err.Error())
		return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
	}

	attrs, err := bh.Attrs(ctx)
	if err != nil && err != storage.ErrBucketNotExist {
		bh.object.Status.SetFailed(failedToRetrieve, err.Error())
		return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
	}

	if attrs == nil {
		return bh.create(ctx)
	}

	return bh.update(ctx, attrs)
}

// createupdater interface defining create and update operations on/for bucket resource
type createupdater interface {
	creater
	updater
}

// bucketCreateUpdater implementation of createupdater interface
type bucketCreateUpdater struct {
	gcpstorage.Client
	kube      client.Client
	object    *v1alpha1.Bucket
	projectID string
}

// newBucketCreateUpdater new instance of bucketCreateUpdater
func newBucketCreateUpdater(sc gcpstorage.Client, cc client.Client, b *v1alpha1.Bucket, pID string) *bucketCreateUpdater {
	return &bucketCreateUpdater{
		Client:    sc,
		kube:      cc,
		object:    b,
		projectID: pID,
	}
}

// create new bucket resource and save changes back to bucket specs
func (bh *bucketCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	util.AddFinalizer(&bh.object.ObjectMeta, finalizer)

	if err := bh.Create(ctx, bh.projectID, v1alpha1.CopyBucketSpecAttrs(&bh.object.Spec.BucketSpecAttrs)); err != nil {
		bh.object.Status.SetFailed(failedToCreate, err.Error())
		return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
	}

	bh.object.Status.SetReady()

	attrs, err := bh.Attrs(ctx)
	if err != nil {
		bh.object.Status.SetFailed(failedToRetrieve, err.Error())
		return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
	}

	bh.object.Spec.BucketSpecAttrs = v1alpha1.NewBucketSpecAttrs(attrs)
	if err := bh.kube.Update(ctx, bh.object); err != nil {
		return resultRequeue, err
	}

	bh.object.Status.BucketOutputAttrs = v1alpha1.NewBucketOutputAttrs(attrs)

	return requeueOnSuccess, bh.kube.Status().Update(ctx, bh.object)
}

// update bucket resource if needed
func (bh *bucketCreateUpdater) update(ctx context.Context, attrs *storage.BucketAttrs) (reconcile.Result, error) {

	current := v1alpha1.NewBucketUpdatableAttrs(attrs)
	if reflect.DeepEqual(*current, bh.object.Spec.BucketUpdatableAttrs) {
		return requeueOnSuccess, nil
	}

	attrs, err := bh.Update(ctx, v1alpha1.CopyToBucketUpdateAttrs(bh.object.Spec.BucketUpdatableAttrs, attrs.Labels))
	if err != nil {
		bh.object.Status.SetFailed(failedToUpdate, err.Error())
		return resultRequeue, bh.kube.Status().Update(ctx, bh.object)
	}

	// Sync attributes back to spec
	bh.object.Spec.BucketSpecAttrs = v1alpha1.NewBucketSpecAttrs(attrs)
	if err := bh.kube.Update(ctx, bh.object); err != nil {
		return resultRequeue, err
	}

	return requeueOnSuccess, bh.kube.Status().Update(ctx, bh.object)
}
