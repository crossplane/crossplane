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

package resourcegroup

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/resourcegroup"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerName = "azure.resourcegroup"
	finalizer      = "finalizer." + controllerName

	reconcileTimeout = 1 * time.Minute
)

var log = logging.Logger.WithName("controller." + controllerName)

var errDeleted = errors.New("resource has been deleted on Azure")

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
	r.Status.SetConditions(corev1alpha1.Creating())
	if _, err := a.client.CreateOrUpdate(ctx, r.Spec.Name, resourcegroup.NewParameters(r)); err != nil {
		r.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return true
	}

	r.Status.Name = r.Spec.Name
	meta.AddFinalizer(r, finalizer)
	r.Status.SetConditions(corev1alpha1.ReconcileSuccess())

	return true
}

func (a *azureResourceGroup) Sync(ctx context.Context, r *v1alpha1.ResourceGroup) bool {
	res, err := a.client.CheckExistence(ctx, r.Spec.Name)
	if err != nil {
		r.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return true
	}

	switch res.Response.StatusCode {
	case http.StatusNoContent:
		r.Status.SetConditions(corev1alpha1.Available(), corev1alpha1.ReconcileSuccess())
		return false
	case http.StatusNotFound:
		// Custom error passed to SetFailed due to Azure API returning 404 instead of error
		r.Status.SetConditions(corev1alpha1.ReconcileError(errDeleted))
		return true
	}

	r.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return true
}

func (a *azureResourceGroup) Delete(ctx context.Context, r *v1alpha1.ResourceGroup) bool {
	r.Status.SetConditions(corev1alpha1.Deleting())
	if r.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if _, err := a.client.Delete(ctx, r.Spec.Name); err != nil {
			r.Status.SetConditions(corev1alpha1.ReconcileError(err))
			return true
		}
	}
	meta.RemoveFinalizer(r, finalizer)
	r.Status.SetConditions(corev1alpha1.ReconcileSuccess())

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
	n := meta.NamespacedNameOf(r.Spec.ProviderReference)
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClient(s.Data[p.Spec.Secret.Key])
	return &azureResourceGroup{client: client}, errors.Wrap(err, "cannot create new Azure Resource Group client")
}

// Reconciler reconciles Resource Group read from the Kubernetes API
// with an external store, typically the Azure API.
type Reconciler struct {
	connecter
	kube client.Client
}

// Controller is responsible for adding the ResourceGroup controller and its
// corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a Controller that reconciles ResourceGroup resources.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		connecter: &providerConnecter{kube: mgr.GetClient(), newClient: resourcegroup.NewClient},
		kube:      mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.ResourceGroup{}).
		Complete(r)
}

// Reconcile Azure Resource Group resources with the Azure API.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ResourceGroupKindAPIVersion, "request", req)

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
		rg.Status.SetConditions(corev1alpha1.ReconcileError(err))
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
