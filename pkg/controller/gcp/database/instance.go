/*
Copyright 2019 The Crossplane Authors.

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

package database

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/util/googleapi"
)

const (
	controllerName = "cloudsqlinstance.database.gcp.crossplane.io"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout    = 1 * time.Minute
	requeueAfterWait    = 10 * time.Second
	requeueAfterSuccess = 5 * time.Minute
)

var (
	// requeueNever - object doesn't need any further reconciliations
	// typically used in terminal cases like: deletion and garbage collection
	requeueNever = reconcile.Result{}

	// requeueNow - put the object on the work queue immediately
	// NOTE: this result triggers exponential back-off delay
	requeueNow = reconcile.Result{Requeue: true}

	// requeueSync - object process was complete and successful and we want to re-sync
	// remote instance with this object after some delay interval
	requeueSync = reconcile.Result{RequeueAfter: requeueAfterSuccess}

	// requeueWait - object processing was partial, due the remote instance is not being ready, i.e.
	// performing managedOperations like: create  or update. We want to repeat the processing fo this
	// object after some short(er) (in comparison to above requeueSync) delay
	requeueWait = reconcile.Result{RequeueAfter: requeueAfterWait}

	log = logging.Logger.WithName("controller." + controllerName)
)

// Reconciler reconciles cloudsql instance objects
type Reconciler struct {
	client  client.Client
	factory factory
}

// Reconcile reads that state of the cloudsql instance object and makes changes based on the state read
// and what is in the instance Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.CloudsqlInstanceGroupVersionKind, "request", request)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	i := &v1alpha1.CloudsqlInstance{}
	if err := r.client.Get(ctx, request.NamespacedName, i); err != nil {
		return requeueNever, handleNotFound(err)
	}

	// create local operations to handle Kubernetes (local) types operations
	lops := r.factory.makeLocalOperations(i, r.client)

	// create managed operations to handle CloudSQL (managed) instance operations
	mops, err := r.factory.makeManagedOperations(ctx, i, lops)
	if err != nil {
		return requeueNow, lops.updateReconcileStatus(ctx, err)
	}

	// create syncdeleter to handle top level reconciliation operations
	sd := r.factory.makeSyncDeleter(mops)

	if meta.WasDeleted(i) {
		return sd.delete(ctx)
	}

	return sd.sync(ctx)
}

type factory interface {
	makeLocalOperations(*v1alpha1.CloudsqlInstance, client.Client) localOperations
	makeManagedOperations(context.Context, *v1alpha1.CloudsqlInstance, localOperations) (managedOperations, error)
	makeSyncDeleter(managedOperations) syncdeleter
}

type operationsFactory struct {
	client.Client
}

var _ factory = &operationsFactory{}

func (f *operationsFactory) makeLocalOperations(inst *v1alpha1.CloudsqlInstance, kube client.Client) localOperations {
	return newLocalHandler(inst, kube)
}

func (f *operationsFactory) makeManagedOperations(ctx context.Context, inst *v1alpha1.CloudsqlInstance, ops localOperations) (managedOperations, error) {
	p := &gcpv1alpha1.Provider{}
	n := meta.NamespacedNameOf(inst.GetProviderReference())
	if err := f.Get(ctx, n, p); err != nil {
		return nil, err
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	if err := f.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider's secret %s", n)
	}

	creds, err := google.CredentialsFromJSON(ctx, s.Data[p.Spec.Secret.Key], cloudsql.DefaultScope)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot retrieve creds from json")
	}

	return newManagedHandler(ctx, inst, ops, creds)
}

func (f *operationsFactory) makeSyncDeleter(ops managedOperations) syncdeleter {
	return &instanceSyncDeleter{
		managedOperations: ops,
		createupdater:     &instanceCreateUpdater{managedOperations: ops},
	}
}

type syncdeleter interface {
	sync(context.Context) (reconcile.Result, error)
	delete(context.Context) (reconcile.Result, error)
}

type instanceSyncDeleter struct {
	managedOperations
	createupdater
}

func (sd *instanceSyncDeleter) delete(ctx context.Context) (reconcile.Result, error) {
	if sd.isReclaimDelete() {
		if err := handleNotFound(sd.deleteInstance(ctx)); err != nil {
			return requeueNow, sd.updateReconcileStatus(ctx, err)
		}
	}
	return requeueNow, sd.removeFinalizer(ctx)
}

// sync - synchronizes the state of the cloudsql instance instance with the
// state of the obj object
func (sd *instanceSyncDeleter) sync(ctx context.Context) (reconcile.Result, error) {
	inst, err := sd.getInstance(ctx)
	if resource.Ignore(googleapi.IsErrorNotFound, err) != nil {
		return requeueNow, sd.updateReconcileStatus(ctx, err)
	}

	if inst == nil {
		return sd.create(ctx)
	}

	return sd.update(ctx, inst)
}

// createupdater interface defining create and update managedOperations on/for cloudsql instance instance
type createupdater interface {
	create(context.Context) (reconcile.Result, error)
	update(context.Context, *sqladmin.DatabaseInstance) (reconcile.Result, error)
}

// instanceCreateUpdater implementation of createupdater interface
type instanceCreateUpdater struct {
	managedOperations
}

// newInstanceCreateUpdater new instance of instanceCreateUpdater
func newInstanceCreateUpdater(ops managedOperations) *instanceCreateUpdater {
	return &instanceCreateUpdater{
		managedOperations: ops,
	}
}

// create new instance instance
func (ih *instanceCreateUpdater) create(ctx context.Context) (reconcile.Result, error) {
	if err := ih.addFinalizer(ctx); err != nil {
		return requeueNow, errors.Wrap(err, "failed to update instance object")
	}

	return requeueNow, ih.updateReconcileStatus(ctx, ih.createInstance(ctx))
}

// update cloudsql instance instance if needed
func (ih *instanceCreateUpdater) update(ctx context.Context, inst *sqladmin.DatabaseInstance) (reconcile.Result, error) {
	if err := ih.updateInstanceStatus(ctx, inst); err != nil {
		return requeueNow, errors.Wrapf(err, "failed to update instance status")
	}

	if !ih.isInstanceReady() {
		return requeueWait, ih.updateReconcileStatus(ctx, nil)
	}

	// NOTE: needsUpdate(...) always returns false, for details see needsUpdate function call
	if ih.needsUpdate(inst) {
		return requeueNow, ih.updateReconcileStatus(ctx, ih.updateInstance(ctx))
	}

	return requeueSync, ih.updateReconcileStatus(ctx, ih.updateUserCreds(ctx))
}

func handleNotFound(err error) error {
	if kerrors.IsNotFound(err) || googleapi.IsErrorNotFound(err) {
		return nil
	}
	return err
}
