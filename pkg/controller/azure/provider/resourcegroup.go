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

package provider

import (
	"context"
	"net/http"
	"time"

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

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/resourcegroup"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerNameRG = "azure.resourcegroup"
	finalizerRG      = "finalizer." + controllerNameRG

	reasonFetchingClient   = "failed to fetch Azure Resource Group client"
	reasonCreatingResource = "failed to create Azure Resource Group resource"
	reasonDeletingResource = "failed to delete Azure Resource Group resource"
	reasonSyncingResource  = "failed to sync Azure Resource Group resource"

	azureDeletedMessage = "resource has been deleted on Azure"

	reconcileTimeout = 1 * time.Minute
)

var logRG = logging.Logger.WithName("controller." + controllerNameRG)

// A creator can create resources in an external store - e.g. the Azure API.
type creator interface {
	// Create the supplied resource in the external store. Returns true if the
	// resource requires further reconciliation.
	Create(ctx context.Context, r *v1alpha1.ResourceGroup) (requeue bool)
}

// A syncer can sync resources with an external store - e.g. the Azure API.
type syncer interface {
	// Sync the supplied resource with the external store. Returns true if the
	// resource requires further reconciliation.
	Sync(ctx context.Context, r *v1alpha1.ResourceGroup) (requeue bool)
}

// A deleter can delete resources from an external store - e.g. the Azure API.
type deleter interface {
	// Delete the supplied resource from the external store. Returns true if the
	// resource requires further reconciliation.
	Delete(ctx context.Context, r *v1alpha1.ResourceGroup) (requeue bool)
}

// A createsyncdeleter can create, sync, and delete resources in an external
// store - e.g. the Azure API.
type createsyncdeleter interface {
	creator
	syncer
	deleter
}

// azureResourceGroup is a createsyncdeleter using the Azure Groups API.
type azureResourceGroup struct {
	client resourcegroup.GroupsClient
}

func (a *azureResourceGroup) Create(ctx context.Context, r *v1alpha1.ResourceGroup) bool {
	if _, err := a.client.CreateOrUpdate(ctx, r.Spec.Name, resourcegroup.NewParameters(r)); err != nil {
		r.Status.SetFailed(reasonCreatingResource, err.Error())
		return true
	}

	r.Status.Name = r.Spec.Name
	r.Status.UnsetAllDeprecatedConditions()
	r.Status.SetCreating()
	util.AddFinalizer(&r.ObjectMeta, finalizerRG)

	return true
}

func (a *azureResourceGroup) Sync(ctx context.Context, r *v1alpha1.ResourceGroup) bool {
	res, err := a.client.CheckExistence(ctx, r.Spec.Name)
	if err != nil {
		r.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}

	r.Status.UnsetAllDeprecatedConditions()

	switch res.Response.StatusCode {
	case http.StatusNoContent:
		r.Status.SetReady()
		return false
	case http.StatusNotFound:
		// Custom message passed to SetFailed due to Azure API returning 404 instead of error
		r.Status.SetFailed(reasonSyncingResource, azureDeletedMessage)
		return true
	}

	return true
}

func (a *azureResourceGroup) Delete(ctx context.Context, r *v1alpha1.ResourceGroup) bool {
	if r.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if _, err := a.client.Delete(ctx, r.Spec.Name); err != nil {
			r.Status.SetFailed(reasonDeletingResource, err.Error())
			return true
		}
	}
	r.Status.SetDeleting()
	util.RemoveFinalizer(&r.ObjectMeta, finalizerRG)

	return false
}

// A connecter returns a createsyncdeleter that can create, sync, and delete
// Azure Resource Group resources with an external store - for example the Azure API.
type connecter interface {
	Connect(context.Context, *v1alpha1.ResourceGroup) (createsyncdeleter, error)
}

// providerConnecter is a connecter that returns a createsyncdeleter
// authenticated using credentials read from a Crossplane Provider resource.
type providerConnecter struct {
	kube      client.Client
	newClient func(creds []byte) (resourcegroup.GroupsClient, error)
}

// Connect returns a createsyncdeleter backed by the Azure API. Azure
// credentials are read from the Crossplane Provider referenced by the supplied
// Resource Group.
func (c *providerConnecter) Connect(ctx context.Context, r *v1alpha1.ResourceGroup) (createsyncdeleter, error) {
	p := &v1alpha1.Provider{}
	n := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.Spec.ProviderRef.Name}
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	if !p.IsValid() {
		return nil, errors.Errorf("provider %s is not ready", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClient(s.Data[p.Spec.Secret.Key])
	return &azureResourceGroup{client: client}, errors.Wrap(err, "cannot create new Azure Resource Group client")
}

// ReconcilerRG reconciles Resource Group read from the Kubernetes API
// with an external store, typically the Azure API.
type ReconcilerRG struct {
	connecter
	kube client.Client
}

// AddResourceGroup creates a new Resource Group Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func AddResourceGroup(mgr manager.Manager) error {
	r := &ReconcilerRG{
		connecter: &providerConnecter{kube: mgr.GetClient(), newClient: resourcegroup.NewClient},
		kube:      mgr.GetClient(),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	return c.Watch(&source.Kind{Type: &v1alpha1.ResourceGroup{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile Azure Resource Group resources with the Azure API.
func (r *ReconcilerRG) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	logRG.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ResourceGroupKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	rg := &v1alpha1.ResourceGroup{}
	if err := r.kube.Get(ctx, req.NamespacedName, rg); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get resource %s", req.NamespacedName)
	}

	client, err := r.Connect(ctx, rg)
	if err != nil {
		rg.Status.SetFailed(reasonFetchingClient, err.Error())
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, rg), "cannot update resource %s", req.NamespacedName)
	}

	// The resource has been deleted from the API server. Delete from Azure.
	if rg.DeletionTimestamp != nil {
		return reconcile.Result{Requeue: client.Delete(ctx, rg)}, errors.Wrapf(r.kube.Update(ctx, rg), "cannot update resource %s", req.NamespacedName)
	}

	// The resource is unnamed. Assume it has not been created in Azure.
	if rg.Status.Name == "" {
		return reconcile.Result{Requeue: client.Create(ctx, rg)}, errors.Wrapf(r.kube.Update(ctx, rg), "cannot update resource %s", req.NamespacedName)
	}

	// The resource exists in the API server and Azure. Sync it.
	return reconcile.Result{Requeue: client.Sync(ctx, rg)}, errors.Wrapf(r.kube.Update(ctx, rg), "cannot update resource %s", req.NamespacedName)
}
