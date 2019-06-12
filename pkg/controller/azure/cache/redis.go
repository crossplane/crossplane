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

package cache

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/redis"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const (
	controllerName = "redis.cache.azure.crossplane.io"
	finalizerName  = "finalizer." + controllerName

	reasonFetchingClient   = "failed to fetch Azure Cache client"
	reasonCreatingResource = "failed to create Azure Cache resource"
	reasonDeletingResource = "failed to delete Azure Cache resource"
	reasonSyncingResource  = "failed to sync Azure Cache resource"
	reasonSyncingSecret    = "failed to sync Azure Cache connection secret" // nolint:gas,gosec
	reasonGettingKey       = "failed to get Azure Cache access key"

	reconcileTimeout = 1 * time.Minute
)

var log = logging.Logger.WithName("controller." + controllerName)

// A creator can create resources in an external store - e.g. the Azure API.
type creator interface {
	// Create the supplied resource in the external store. Returns true if the
	// resource requires further reconciliation.
	Create(ctx context.Context, r *v1alpha1.Redis) (requeue bool)
}

// A syncer can sync resources with an external store - e.g. the Azure API.
type syncer interface {
	// Sync the supplied resource with the external store. Returns true if the
	// resource requires further reconciliation.
	Sync(ctx context.Context, r *v1alpha1.Redis) (requeue bool)
}

// A deleter can delete resources from an external store - e.g. the Azure API.
type deleter interface {
	// Delete the supplied resource from the external store. Returns true if the
	// resource requires further reconciliation.
	Delete(ctx context.Context, r *v1alpha1.Redis) (requeue bool)
}

// A keyer can read the primary access key for the supplied resource.
type keyer interface {
	// Key returns the primary access key for the supplied resource.
	Key(ctx context.Context, r *v1alpha1.Redis) (key string)
}

// A createsyncdeletekeyer an create, sync, and delete resources in an external
// store - e.g. the Azure API. It can also return keys (i.e. credentials) for
// resources.
type createsyncdeletekeyer interface {
	creator
	syncer
	deleter
	keyer
}

// azureRedisCache is a createsyncdeletekeyer using the Azure Azure Cache API.
type azureRedisCache struct {
	client redis.Client
}

func (a *azureRedisCache) Create(ctx context.Context, r *v1alpha1.Redis) bool {
	n := redis.NewResourceName(r)
	if _, err := a.client.Create(ctx, r.Spec.ResourceGroupName, n, redis.NewCreateParameters(r)); err != nil {
		r.Status.SetFailed(reasonCreatingResource, err.Error())
		return true
	}

	r.Status.ResourceName = n
	r.Status.UnsetAllDeprecatedConditions()
	r.Status.SetCreating()
	meta.AddFinalizer(r, finalizerName)

	return true
}

func (a *azureRedisCache) Sync(ctx context.Context, r *v1alpha1.Redis) bool {
	n := redis.NewResourceName(r)
	cacheResource, err := a.client.Get(ctx, r.Spec.ResourceGroupName, n)
	if err != nil {
		r.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}

	r.Status.State = string(cacheResource.ProvisioningState)
	r.Status.UnsetAllDeprecatedConditions()

	switch r.Status.State {
	case v1alpha1.ProvisioningStateSucceeded:
		// TODO(negz): Set r.Status.State to something like 'Ready'? The Azure
		// portal shows an instance as 'Ready', but the API shows only that the
		// provisioning state is 'Succeeded'. It's a little weird to see a Redis
		// resource in state 'Succeeded' in kubectl.
		r.Status.SetReady()
	case v1alpha1.ProvisioningStateCreating:
		r.Status.SetCreating()
		return true
	case v1alpha1.ProvisioningStateDeleting:
		r.Status.SetDeleting()
		return false
	default:
		// TODO(negz): Don't requeue in this scenario? The instance could be
		// failed, disabled, scaling, etc.
		return true
	}

	r.Status.Endpoint = azure.ToString(cacheResource.HostName)
	r.Status.Port = azure.ToInt(cacheResource.Port)
	r.Status.SSLPort = azure.ToInt(cacheResource.SslPort)
	r.Status.RedisVersion = azure.ToString(cacheResource.RedisVersion)
	r.Status.ProviderID = azure.ToString(cacheResource.ID)

	if !redis.NeedsUpdate(r, cacheResource) {
		return false
	}

	if _, err := a.client.Update(ctx, r.Spec.ResourceGroupName, n, redis.NewUpdateParameters(r)); err != nil {
		r.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}

	return false
}

func (a *azureRedisCache) Delete(ctx context.Context, r *v1alpha1.Redis) bool {
	if r.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if _, err := a.client.Delete(ctx, r.Spec.ResourceGroupName, redis.NewResourceName(r)); err != nil {
			r.Status.SetFailed(reasonDeletingResource, err.Error())
			return true
		}
	}
	r.Status.SetDeleting()
	meta.RemoveFinalizer(r, finalizerName)
	return false
}

func (a *azureRedisCache) Key(ctx context.Context, r *v1alpha1.Redis) string {
	n := redis.NewResourceName(r)
	k, err := a.client.ListKeys(ctx, r.Spec.ResourceGroupName, n)
	if err != nil {
		r.Status.SetFailed(reasonGettingKey, err.Error())
		return ""
	}
	return azure.ToString(k.PrimaryKey)
}

// A connecter returns a createsyncdeletekeyer that can create, sync, and delete
// Azure Cache resources with an external store - for example the Azure API.
type connecter interface {
	Connect(context.Context, *v1alpha1.Redis) (createsyncdeletekeyer, error)
}

// providerConnecter is a connecter that returns a createsyncdeletekeyer
// authenticated using credentials read from a Crossplane Provider resource.
type providerConnecter struct {
	kube      client.Client
	newClient func(ctx context.Context, creds []byte) (redis.Client, error)
}

// Connect returns a createsyncdeletekeyer backed by the Azure API. Azure
// credentials are read from the Crossplane Provider referenced by the supplied
// Redis.
func (c *providerConnecter) Connect(ctx context.Context, r *v1alpha1.Redis) (createsyncdeletekeyer, error) {
	p := &azurev1alpha1.Provider{}
	n := types.NamespacedName{Namespace: r.GetNamespace(), Name: r.Spec.ProviderRef.Name}
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClient(ctx, s.Data[p.Spec.Secret.Key])
	return &azureRedisCache{client: client}, errors.Wrap(err, "cannot create new Azure Cache client")
}

// Reconciler reconciles Redis read from the Kubernetes API
// with an external store, typically the Azure API.
type Reconciler struct {
	connecter
	kube client.Client
}

// Add creates a new Redis Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		connecter: &providerConnecter{kube: mgr.GetClient(), newClient: redis.NewClient},
		kube:      mgr.GetClient(),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	return c.Watch(&source.Kind{Type: &v1alpha1.Redis{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile Google Azure Cache resources with the Azure API.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.RedisKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	rd := &v1alpha1.Redis{}
	if err := r.kube.Get(ctx, req.NamespacedName, rd); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get resource %s", req.NamespacedName)
	}

	client, err := r.Connect(ctx, rd)
	if err != nil {
		rd.Status.SetFailed(reasonFetchingClient, err.Error())
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	// The resource has been deleted from the API server. Delete from Azure.
	if rd.DeletionTimestamp != nil {
		return reconcile.Result{Requeue: client.Delete(ctx, rd)}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	// The resource is unnamed. Assume it has not been created in Azure.
	if rd.Status.ResourceName == "" {
		return reconcile.Result{Requeue: client.Create(ctx, rd)}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	if err := r.upsertSecret(ctx, connectionSecret(rd, client.Key(ctx, rd))); err != nil {
		rd.Status.SetFailed(reasonSyncingSecret, err.Error())
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	// The resource exists in the API server and Azure. Sync it.
	return reconcile.Result{Requeue: client.Sync(ctx, rd)}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
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

func connectionSecret(r *v1alpha1.Redis, accessKey string) *corev1.Secret {
	ref := meta.AsOwner(meta.ReferenceTo(r, v1alpha1.RedisGroupVersionKind))
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            r.ConnectionSecretName(),
			Namespace:       r.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},

		// TODO(negz): Include the ports here too?
		// TODO(negz): Include both access keys? Azure has two because reasons.
		Data: map[string][]byte{
			corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(r.Status.Endpoint),
			corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(accessKey),
		},
	}
}
