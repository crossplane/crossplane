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

package cache

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/gcp/apis/cache/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudmemorystore"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

const (
	controllerName   = "cloudmemorystoreinstances.cache.gcp.crossplane.io"
	finalizerName    = "finalizer." + controllerName
	reconcileTimeout = 1 * time.Minute
)

var log = logging.Logger.WithName("controller." + controllerName)

// A creator can create instances in an external store - e.g. the GCP API.
type creator interface {
	// Create the supplied instance in the external store. Returns true if the
	// instance requires further reconciliation.
	Create(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (requeue bool)
}

// A syncer can sync instances with an external store - e.g. the GCP API.
type syncer interface {
	// Sync the supplied instance with the external store. Returns true if the
	// instance requires further reconciliation.
	Sync(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (requeue bool)
}

// A deleter can delete instances from an external store - e.g. the GCP API.
type deleter interface {
	// Delete the supplied instance from the external store. Returns true if the
	// instance requires further reconciliation.
	Delete(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (requeue bool)
}

// A createsyncdeleter an create, sync, and delete instances in an external
// store - e.g. the GCP API.
type createsyncdeleter interface {
	creator
	syncer
	deleter
}

// cloudMemorystore is a createsyncdeleter using the GCP CloudMemorystore API.
type cloudMemorystore struct {
	client  cloudmemorystore.Client
	project string
}

func (c *cloudMemorystore) Create(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	i.Status.SetConditions(corev1alpha1.Creating())

	id := cloudmemorystore.NewInstanceID(c.project, i)
	if _, err := c.client.CreateInstance(ctx, cloudmemorystore.NewCreateInstanceRequest(id, i)); err != nil {
		i.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return true
	}

	i.Status.InstanceName = id.Instance
	meta.AddFinalizer(i, finalizerName)
	i.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return true
}

func (c *cloudMemorystore) Sync(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	id := cloudmemorystore.NewInstanceID(c.project, i)
	gcpInstance, err := c.client.GetInstance(ctx, cloudmemorystore.NewGetInstanceRequest(id))
	if err != nil {
		i.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return true
	}

	i.Status.State = gcpInstance.GetState().String()

	switch i.Status.State {
	case v1alpha1.StateReady:
		i.Status.SetConditions(corev1alpha1.Available())
		resource.SetBindable(i)
	case v1alpha1.StateCreating:
		i.Status.SetConditions(corev1alpha1.Creating(), corev1alpha1.ReconcileSuccess())
		return true
	case v1alpha1.StateDeleting:
		i.Status.SetConditions(corev1alpha1.Deleting(), corev1alpha1.ReconcileSuccess())
		return false
	default:
		// TODO(negz): Don't requeue in this scenario? The instance is probably
		// in maintenance, updating, or repairing, which can take minutes.
		i.Status.SetConditions(corev1alpha1.ReconcileSuccess())
		return true
	}

	i.Status.Endpoint = gcpInstance.GetHost()
	i.Status.Port = int(gcpInstance.GetPort())
	i.Status.ProviderID = gcpInstance.GetName()

	if !cloudmemorystore.NeedsUpdate(i, gcpInstance) {
		i.Status.SetConditions(corev1alpha1.ReconcileSuccess())
		return false
	}

	if _, err := c.client.UpdateInstance(ctx, cloudmemorystore.NewUpdateInstanceRequest(id, i)); err != nil {
		i.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return true
	}

	i.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return false
}

func (c *cloudMemorystore) Delete(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) bool {
	i.Status.SetConditions(corev1alpha1.Deleting())

	if i.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		id := cloudmemorystore.NewInstanceID(c.project, i)
		if _, err := c.client.DeleteInstance(ctx, cloudmemorystore.NewDeleteInstanceRequest(id)); err != nil {
			i.Status.SetConditions(corev1alpha1.ReconcileError(err))
			return true
		}
	}

	meta.RemoveFinalizer(i, finalizerName)
	i.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return false
}

// A connecter returns a createsyncdeleter that can create, sync, and delete
// CloudMemorystore instances with an external store - for example the GCP API.
type connecter interface {
	Connect(context.Context, *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error)
}

// providerConnecter is a connecter that returns a createsyncdeleter
// authenticated using credentials read from a Crossplane Provider resource.
type providerConnecter struct {
	kube      client.Client
	newClient func(ctx context.Context, creds []byte) (cloudmemorystore.Client, error)
}

// Connect returns a createsyncdeleter backed by the GCP API. GCP credentials
// are read from the Crossplane Provider referenced by the supplied
// CloudMemorystoreInstance.
func (c *providerConnecter) Connect(ctx context.Context, i *v1alpha1.CloudMemorystoreInstance) (createsyncdeleter, error) {
	p := &gcpv1alpha1.Provider{}
	n := meta.NamespacedNameOf(i.Spec.ProviderReference)
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClient(ctx, s.Data[p.Spec.Secret.Key])
	return &cloudMemorystore{client: client, project: p.Spec.ProjectID}, errors.Wrap(err, "cannot create new CloudMemorystore client")
}

// Reconciler reconciles CloudMemorystoreInstances read from the Kubernetes API
// with an external store, typically the GCP API.
type Reconciler struct {
	connecter
	kube client.Client
}

// CloudMemorystoreInstanceController is responsible for adding the Cloud Memorystore
// controller and its corresponding reconciler to the manager with any runtime configuration.
type CloudMemorystoreInstanceController struct{}

// SetupWithManager creates a new CloudMemorystoreInstance Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func (c *CloudMemorystoreInstanceController) SetupWithManager(mgr ctrl.Manager) error {
	r := &Reconciler{
		connecter: &providerConnecter{kube: mgr.GetClient(), newClient: cloudmemorystore.NewClient},
		kube:      mgr.GetClient(),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.CloudMemorystoreInstance{}).
		Complete(r)
}

// Reconcile Google CloudMemorystore resources with the GCP API.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.CloudMemorystoreInstanceKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	i := &v1alpha1.CloudMemorystoreInstance{}
	if err := r.kube.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get instance %s", req.NamespacedName)
	}

	client, err := r.Connect(ctx, i)
	if err != nil {
		i.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, i), "cannot update instance %s", req.NamespacedName)
	}

	// The instance has been deleted from the API server. Delete from GCP.
	if i.DeletionTimestamp != nil {
		return reconcile.Result{Requeue: client.Delete(ctx, i)}, errors.Wrapf(r.kube.Update(ctx, i), "cannot update instance %s", req.NamespacedName)
	}

	// The instance is unnamed. Assume it has not been created in GCP.
	if i.Status.InstanceName == "" {
		return reconcile.Result{Requeue: client.Create(ctx, i)}, errors.Wrapf(r.kube.Update(ctx, i), "cannot update instance %s", req.NamespacedName)
	}

	if err := r.upsertSecret(ctx, connectionSecret(i)); err != nil {
		i.Status.SetConditions(corev1alpha1.ReconcileError(err))
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, i), "cannot update instance %s", req.NamespacedName)
	}

	// The instance exists in the API server and GCP. Sync it.
	return reconcile.Result{Requeue: client.Sync(ctx, i)}, errors.Wrapf(r.kube.Update(ctx, i), "cannot update instance %s", req.NamespacedName)
}

func (r *Reconciler) upsertSecret(ctx context.Context, s *corev1.Secret) error {
	n := types.NamespacedName{Namespace: s.GetNamespace(), Name: s.GetName()}
	if err := r.kube.Get(ctx, n, &corev1.Secret{}); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.kube.Create(ctx, s), "cannot create secret %s", n)
		}
		return errors.Wrapf(err, "cannot get secret %s", n)
	}
	return errors.Wrapf(r.kube.Update(ctx, s), "cannot update secret %s", n)
}

func connectionSecret(i *v1alpha1.CloudMemorystoreInstance) *corev1.Secret {
	// TODO(negz): Include the port here too?
	s := resource.ConnectionSecretFor(i, v1alpha1.CloudMemorystoreInstanceGroupVersionKind)
	s.Data = map[string][]byte{corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(i.Status.Endpoint)}
	return s
}
