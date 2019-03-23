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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "handle.storage.gcp.crossplane.io"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 30 * time.Second
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
	factory
}

// Add creates a new Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetRecorder(controllerName),
		factory:  &bucketFactory{mgr.GetClient()},
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &v1alpha1.Bucket{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
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

	bh, err := r.new(ctx, b)
	if err != nil {
		b.Status.SetFailed("failed get object handler", err.Error())
		return resultRequeue, r.Status().Update(ctx, b)
	}

	// Check for deletion
	if b.DeletionTimestamp != nil {
		return bh.delete(ctx)
	}

	return bh.sync(ctx)
}

type factory interface {
	new(context.Context, *v1alpha1.Bucket) (resourceHandler, error)
}

type bucketFactory struct {
	client.Client
}

func (m *bucketFactory) new(ctx context.Context, b *v1alpha1.Bucket) (resourceHandler, error) {
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
		return nil, errors.Wrapf(err, "error creating storage client")
	}

	return &bucketHandler{
		BucketHandle: sc.Bucket(string(b.GetUID())),
		client:       m.Client,
		object:       b,
		projectID:    creds.ProjectID,
	}, nil
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

type resourceHandler interface {
	deleter
	syncer
	creater
	updater
}

type bucketHandler struct {
	*storage.BucketHandle
	client    client.Client
	object    *v1alpha1.Bucket
	projectID string
}

func (bh *bucketHandler) delete(ctx context.Context) (reconcile.Result, error) {
	if bh.object.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := bh.Delete(ctx); err != nil && err != storage.ErrBucketNotExist {
			bh.object.Status.SetFailed("error deleting", err.Error())
			return resultRequeue, bh.client.Status().Update(ctx, bh.object)
		}
	}
	util.RemoveFinalizer(&bh.object.ObjectMeta, finalizer)
	return reconcile.Result{}, bh.client.Update(ctx, bh.object)
}

// sync - synchronizes the state of the bucket resource with the state of the
// bucket Kubernetes object
func (bh *bucketHandler) sync(ctx context.Context) (reconcile.Result, error) {

	attrs, err := bh.Attrs(ctx)
	if err != nil && err != storage.ErrBucketNotExist {
		bh.object.Status.SetFailed("error retrieving", err.Error())
		return resultRequeue, bh.client.Status().Update(ctx, bh.object)
	}

	if attrs == nil {
		return bh.create(ctx)
	}

	return bh.update(ctx, attrs)
}

// create - creates new bucket resource
func (bh *bucketHandler) create(ctx context.Context) (reconcile.Result, error) {
	util.AddFinalizer(&bh.object.ObjectMeta, finalizer)

	if err := bh.Create(ctx, bh.projectID, v1alpha1.CopyBucketSpecAttrs(&bh.object.Spec.BucketSpecAttrs)); err != nil {
		bh.object.Status.SetFailed("create", err.Error())
		return resultRequeue, bh.client.Status().Update(ctx, bh.object)
	}

	bh.object.Status.SetReady()

	attrs, err := bh.Attrs(ctx)
	if err != nil {
		bh.object.Status.SetFailed("error retrieving after create", err.Error())
		return resultRequeue, bh.client.Status().Update(ctx, bh.object)
	}

	bh.object.Spec.BucketSpecAttrs = v1alpha1.NewBucketSpecAttrs(attrs)
	if err := bh.client.Update(ctx, bh.object); err != nil {
		return resultRequeue, err
	}

	bh.object.Status.BucketOutputAttrs = v1alpha1.NewBucketOutputAttrs(attrs)

	return requeueOnSuccess, bh.client.Status().Update(ctx, bh.object)
}

func (bh *bucketHandler) update(ctx context.Context, attrs *storage.BucketAttrs) (reconcile.Result, error) {

	current := *v1alpha1.NewBucketUpdatableAttrs(attrs)
	if reflect.DeepEqual(current, bh.object.Spec.BucketUpdatableAttrs) {
		return requeueOnSuccess, nil
	}

	attrs, err := bh.Update(ctx, v1alpha1.CopyToBucketUpdateAttrs(bh.object.Spec.BucketUpdatableAttrs, attrs.Labels))
	if err != nil {
		bh.object.Status.SetFailed("error updating", err.Error())
		return resultRequeue, bh.client.Status().Update(ctx, bh.object)
	}

	// Sync attributes back to spec
	bh.object.Spec.BucketSpecAttrs = v1alpha1.NewBucketSpecAttrs(attrs)
	if err := bh.client.Update(ctx, bh.object); err != nil {
		return resultRequeue, err
	}

	return requeueOnSuccess, bh.client.Status().Update(ctx, bh.object)
}
