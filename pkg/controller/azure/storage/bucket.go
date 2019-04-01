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

	"github.com/go-test/deep"

	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	azurestorage "github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "account.storage.azure.crossplane.io"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 30 * time.Second

	// failed condition reasons
	//
	// failed to get resource handler
	failedToGetHandler = "error getting handler"
	// failed to delete account resource
	failedToDelete = "error deleting"
	// failed to retrieve account resource
	failedToRetrieve = "error retrieving"
	// failed to create account resource
	failedToCreate = "error creating"
	// failed to update account resource
	failedToUpdate = "error updating"
	// failed to save connection secret
	failedToSaveSecret = "error saving connection secret"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}
)

// Reconciler reconciles a GCP storage account obj
type Reconciler struct {
	client.Client
	factory
}

// Add creates a newHandler Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:  mgr.GetClient(),
		factory: &accountHandleMaker{mgr.GetClient()},
	}

	// Create a newHandler controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	return c.Watch(&source.Kind{Type: &v1alpha1.Account{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile reads that state of the cluster for a Provider obj and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", v1alpha1.AccountKindAPIVersion, request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	b := &v1alpha1.Account{}
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
	newHandler(context.Context, *v1alpha1.Account) (syncdeleter, error)
}

type accountHandleMaker struct {
	client.Client
}

func (m *accountHandleMaker) newHandler(ctx context.Context, b *v1alpha1.Account) (syncdeleter, error) {
	p := &azurev1alpha1.Provider{}
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

	storageClient, err := azurestorage.NewStorageAccountClient(s.Data[p.Spec.Secret.Key])
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create storageClient from json")
	}

	return newAccountSyncDeleter(
		azurestorage.NewAccountHandle(storageClient, b.Spec.GroupName, b.Spec.StorageAccountName),
		m.Client, b), nil
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
	update(context.Context, *storage.Account) (reconcile.Result, error)
}

type syncdeleter interface {
	deleter
	syncer
}

type accountSyncDeleter struct {
	createupdater
	azurestorage.AccountOperations
	kube   client.Client
	object *v1alpha1.Account
}

func newAccountSyncDeleter(sc azurestorage.AccountOperations, cc client.Client, b *v1alpha1.Account) *accountSyncDeleter {
	return &accountSyncDeleter{
		createupdater:     newAccountCreateUpdater(sc, cc, b),
		AccountOperations: sc,
		kube:              cc,
		object:            b,
	}
}

func (asd *accountSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	if asd.object.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := asd.Delete(ctx); err != nil && !azure.IsNotFound(err) {
			asd.object.Status.SetFailed(failedToDelete, err.Error())
			return resultRequeue, asd.kube.Status().Update(ctx, asd.object)
		}
	}
	util.RemoveFinalizer(&asd.object.ObjectMeta, finalizer)
	return reconcile.Result{}, asd.kube.Update(ctx, asd.object)
}

// sync - synchronizes the state of the storage account resource with the state of the
// account Kubernetes obj
func (asd *accountSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	// create connection secret if it doesn't exist
	if err := asd.kube.Create(ctx, asd.object.ConnectionSecret()); err != nil && !kerrors.IsAlreadyExists(err) {
		asd.object.Status.SetFailed(failedToSaveSecret, err.Error())
		return resultRequeue, asd.kube.Status().Update(ctx, asd.object)
	}

	account, err := asd.Get(ctx)
	if err != nil && !azure.IsNotFound(err) {
		asd.object.Status.SetFailed(failedToRetrieve, err.Error())
		return resultRequeue, asd.kube.Status().Update(ctx, asd.object)
	}

	if account == nil {
		return asd.create(ctx)
	}

	return asd.update(ctx, account)
}

// createupdater interface defining create and update operations on/for storage account resource
type createupdater interface {
	creater
	updater
}

// accountCreateUpdater implementation of createupdater interface
type accountCreateUpdater struct {
	azurestorage.AccountOperations
	kube      client.Client
	object    *v1alpha1.Account
	projectID string
}

// newAccountCreateUpdater new instance of accountCreateUpdater
func newAccountCreateUpdater(sc azurestorage.AccountOperations, cc client.Client, b *v1alpha1.Account) *accountCreateUpdater {
	return &accountCreateUpdater{
		AccountOperations: sc,
		kube:              cc,
		object:            b,
	}
}

// create new storage account resource and save changes back to account specs
func (acu *accountCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	util.AddFinalizer(&acu.object.ObjectMeta, finalizer)

	a, err := acu.Create(ctx, v1alpha1.ToStorageAccountCreate(acu.object.Spec.StorageAccountSpec))
	if err != nil {
		acu.object.Status.SetFailed(failedToCreate, err.Error())
		return resultRequeue, acu.kube.Status().Update(ctx, acu.object)
	}

	acu.object.Status.SetReady()

	acu.object.Spec.StorageAccountSpec = v1alpha1.NewStorageAccountSpec(a)
	if err := acu.kube.Update(ctx, acu.object); err != nil {
		return resultRequeue, err
	}

	acu.object.Status.StorageAccountStatus = v1alpha1.NewStorageAccountStatus(a)
	return requeueOnSuccess, acu.kube.Status().Update(ctx, acu.object)
}

// update storage account resource if needed
func (acu *accountCreateUpdater) update(ctx context.Context, account *storage.Account) (reconcile.Result, error) {

	current := v1alpha1.NewStorageAccountSpec(account)
	if reflect.DeepEqual(current, acu.object.Spec.StorageAccountSpec) {
		return requeueOnSuccess, nil
	}

	if diff := deep.Equal(current, acu.object.Spec.StorageAccountSpec); diff != nil {
		fmt.Println(diff)
	}

	a, err := acu.Update(ctx, v1alpha1.ToStorageAccountUpdate(acu.object.Spec.StorageAccountSpec))
	if err != nil {
		acu.object.Status.SetFailed(failedToUpdate, err.Error())
		return resultRequeue, acu.kube.Status().Update(ctx, acu.object)
	}

	// Sync attributes back to spec
	acu.object.Spec.StorageAccountSpec = v1alpha1.NewStorageAccountSpec(a)
	if err := acu.kube.Update(ctx, acu.object); err != nil {
		return resultRequeue, err
	}

	acu.object.Status.StorageAccountStatus = v1alpha1.NewStorageAccountStatus(a)
	return requeueOnSuccess, acu.kube.Status().Update(ctx, acu.object)
}
