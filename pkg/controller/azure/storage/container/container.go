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

package container

import (
	"context"
	"reflect"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/to"
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
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/storage"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerName = "container.storage.azure.crossplane.io"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout      = 2 * time.Minute
	requeueAfterOnSuccess = 1 * time.Minute

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
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}

	log = logging.Logger.WithName("controller." + controllerName)
)

// Reconciler reconciles an Azure storage container
type Reconciler struct {
	client.Client
	syncdeleterMaker
}

// Add creates a newSyncdeleter Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		Client:           mgr.GetClient(),
		syncdeleterMaker: &containerSyncdeleterMaker{mgr.GetClient()},
	}

	// Create a newSyncdeleter controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to storage container
	if err := c.Watch(&source.Kind{Type: &v1alpha1.Container{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to account Secret
	return c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Container{},
	})
}

// Reconcile reads that state of the cluster for a Provider acct and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ContainerKindAPIVersion, "request", request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	c := &v1alpha1.Container{}
	if err := r.Get(ctx, request.NamespacedName, c); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	sd, err := r.newSyncdeleter(ctx, c)
	if err != nil {
		c.Status.SetFailed(failedToGetHandler, err.Error())
		return resultRequeue, r.Status().Update(ctx, c)
	}

	// Check for deletion
	if c.DeletionTimestamp != nil {
		return sd.delete(ctx)
	}

	return sd.sync(ctx)
}

type syncdeleterMaker interface {
	newSyncdeleter(context.Context, *v1alpha1.Container) (syncdeleter, error)
}

type containerSyncdeleterMaker struct {
	client.Client
}

func (m *containerSyncdeleterMaker) newSyncdeleter(ctx context.Context, c *v1alpha1.Container) (syncdeleter, error) {
	// Retrieve storage account reference object
	acct := &v1alpha1.Account{}
	n := types.NamespacedName{Namespace: c.GetNamespace(), Name: c.Spec.AccountRef.Name}
	if err := m.Get(ctx, n, acct); err != nil {
		// For storage account not found errors - check if we are on deletion path
		// if so - remove finalizer from this container object
		if kerrors.IsNotFound(err) && c.DeletionTimestamp != nil {
			meta.RemoveFinalizer(c, finalizer)
			if err := m.Client.Update(ctx, c); err != nil {
				return nil, errors.Wrapf(err, "failed to update after removing finalizer")
			}
		}
		return nil, errors.Wrapf(err, "failed to retrieve storage account reference: %s", n)
	}

	// Retrieve storage account secret
	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: acct.GetNamespace(), Name: acct.Status.ConnectionSecretRef.Name}
	if err := m.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve storage account secret: %s", n)
	}

	accountName := string(s.Data[corev1alpha1.ResourceCredentialsSecretUserKey])
	accountPassword := string(s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey])
	containerName := c.GetContainerName()

	ch, err := storage.NewContainerHandle(accountName, accountPassword, containerName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client handle: %s, storage account: %s", containerName, accountName)
	}

	// set owner reference on the container to storage account, thus
	// if the account is delete - container is garbage collected as well
	or := meta.AsOwner(meta.ReferenceTo(acct))
	or.BlockOwnerDeletion = to.BoolPtr(true)
	meta.AddOwnerReference(c, or)

	// save connection secret reference
	c.Status.ConnectionSecretRef = corev1.LocalObjectReference{Name: s.Name}

	return &containerSyncdeleter{
		createupdater: &containerCreateUpdater{
			ContainerOperations: ch,
			kube:                m.Client,
			container:           c,
		},
		ContainerOperations: ch,
		kube:                m.Client,
		container:           c,
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
	update(context.Context, *azblob.PublicAccessType, azblob.Metadata) (reconcile.Result, error)
}

type syncdeleter interface {
	deleter
	syncer
}

type containerSyncdeleter struct {
	createupdater
	storage.ContainerOperations
	kube      client.Client
	container *v1alpha1.Container
}

func (csd *containerSyncdeleter) delete(ctx context.Context) (reconcile.Result, error) {
	if csd.container.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if err := csd.Delete(ctx); err != nil && !azure.IsNotFound(err) {
			csd.container.Status.SetFailed(failedToDelete, err.Error())
			return resultRequeue, csd.kube.Status().Update(ctx, csd.container)
		}
	}
	meta.RemoveFinalizer(csd.container, finalizer)
	return reconcile.Result{}, csd.kube.Update(ctx, csd.container)
}

func (csd *containerSyncdeleter) sync(ctx context.Context) (reconcile.Result, error) {
	access, meta, err := csd.Get(ctx)
	if err != nil && !storage.IsNotFoundError(err) {
		csd.container.Status.SetFailed(failedToRetrieve, err.Error())
		return resultRequeue, csd.kube.Status().Update(ctx, csd.container)
	}

	if access == nil {
		return csd.create(ctx)
	}

	return csd.update(ctx, access, meta)
}

type createupdater interface {
	creater
	updater
}

// containerCreateUpdater implementation of createupdater interface
type containerCreateUpdater struct {
	storage.ContainerOperations
	kube      client.Client
	container *v1alpha1.Container
}

var _ createupdater = &containerCreateUpdater{}

func (ccu *containerCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	container := ccu.container

	meta.AddFinalizer(container, finalizer)
	if err := ccu.kube.Update(ctx, container); err != nil {
		return resultRequeue, errors.Wrapf(err, "failed to update container spec")
	}

	spec := container.Spec
	if err := ccu.Create(ctx, spec.PublicAccessType, spec.Metadata); err != nil {
		ccu.container.Status.SetFailed(failedToCreate, err.Error())
		return resultRequeue, ccu.kube.Status().Update(ctx, container)
	}

	return ccu.updateStatusIfNotReady(ctx)
}

func (ccu *containerCreateUpdater) update(ctx context.Context, accessType *azblob.PublicAccessType, meta azblob.Metadata) (reconcile.Result, error) {
	container := ccu.container
	spec := container.Spec

	if reflect.DeepEqual(*accessType, spec.PublicAccessType) && reflect.DeepEqual(meta, spec.Metadata) {
		return ccu.updateStatusIfNotReady(ctx)
	}

	if err := ccu.Update(ctx, spec.PublicAccessType, spec.Metadata); err != nil {
		container.Status.SetFailed(failedToUpdate, err.Error())
		return resultRequeue, ccu.kube.Status().Update(ctx, container)
	}

	return ccu.updateStatusIfNotReady(ctx)
}

func (ccu *containerCreateUpdater) updateStatusIfNotReady(ctx context.Context) (reconcile.Result, error) {
	if !ccu.container.Status.IsReady() {
		ccu.container.Status.UnsetAllDeprecatedConditions()
		ccu.container.Status.SetReady()
		return reconcile.Result{}, ccu.kube.Status().Update(ctx, ccu.container)
	}
	return requeueOnSuccess, nil
}
