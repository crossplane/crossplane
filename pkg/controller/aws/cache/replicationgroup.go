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

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "replicationgroup.cache.aws.crossplane.io"
	finalizerName  = "finalizer." + controllerName

	reasonFetchingClient   = "failed to fetch AWS Replication Group client"
	reasonCreatingResource = "failed to create AWS Replication Group"
	reasonDeletingResource = "failed to delete AWS Replication Group"
	reasonSyncingResource  = "failed to sync AWS Replication Group"
	reasonSyncingSecret    = "failed to sync AWS Replication Group connection secret" // nolint:gas,gosec

	reconcileTimeout = 1 * time.Minute

	// Note this is the length of the generated random byte slice before base64
	// encoding, which adds ~33% overhead. ElastiCache allows auth tokens to be
	maxAuthTokenData = 32
)

var log = logging.Logger.WithName("controller." + controllerName)

// A creator can create resources in an external store - e.g. the AWS API.
type creator interface {
	// Create the supplied resource in the external store. Returns true if the
	// resource requires further reconciliation, and an authentication token
	// used to connect to the Redis endpoint. The authentication token will be
	// an empty string if no token is required.
	Create(ctx context.Context, r *v1alpha1.ReplicationGroup) (requeue bool, authToken string)
}

// A syncer can sync resources with an external store - e.g. the AWS API.
type syncer interface {
	// Sync the supplied resource with the external store. Returns true if the
	// resource requires further reconciliation.
	Sync(ctx context.Context, r *v1alpha1.ReplicationGroup) (requeue bool)
}

// A deleter can delete resources from an external store - e.g. the AWS API.
type deleter interface {
	// Delete the supplied resource from the external store. Returns true if the
	// resource requires further reconciliation.
	Delete(ctx context.Context, r *v1alpha1.ReplicationGroup) (requeue bool)
}

// A createsyncdeleter can create, sync, and delete resources in an external
// store - e.g. the AWS API.
type createsyncdeleter interface {
	creator
	syncer
	deleter
}

// elastiCache is a createsyncdeleter using the AWS Replication Group API.
type elastiCache struct{ client elasticache.Client }

func (e *elastiCache) Create(ctx context.Context, g *v1alpha1.ReplicationGroup) (bool, string) {
	// Our create request will fail if auth is enabled but transit encryption is
	// not. We don't check for the latter here because it's less surprising to
	// submit the request as the operator intended and let the resource
	// transition to failed with an explanatory message from AWS explaining that
	// transit encryption is required.
	var authToken string
	if g.Spec.AuthEnabled {
		at, err := util.GeneratePassword(maxAuthTokenData)
		if err != nil {
			g.Status.SetFailed(reasonCreatingResource, err.Error())
			return true, authToken
		}
		authToken = at
	}

	req := e.client.CreateReplicationGroupRequest(elasticache.NewCreateReplicationGroupInput(g, authToken))
	req.SetContext(ctx)
	if _, err := req.Send(); err != nil {
		g.Status.SetFailed(reasonCreatingResource, err.Error())
		return true, authToken
	}

	g.Status.GroupName = elasticache.NewReplicationGroupID(g)
	g.Status.UnsetAllDeprecatedConditions()
	g.Status.SetCreating()
	meta.AddFinalizer(g, finalizerName)

	return true, authToken
}

// TODO(negz): This method's cyclomatic complexity is a little high. Consider
// refactoring to reduce said complexity if you touch it.
// nolint:gocyclo
func (e *elastiCache) Sync(ctx context.Context, g *v1alpha1.ReplicationGroup) bool {
	drg := e.client.DescribeReplicationGroupsRequest(elasticache.NewDescribeReplicationGroupsInput(g))
	drg.SetContext(ctx)
	rsp, err := drg.Send()
	if err != nil {
		g.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}
	// DescribeReplicationGroups can return one or many replication groups. We
	// ask for one group by name, so we should get either a single element list
	//  or an error.
	replicationGroup := rsp.ReplicationGroups[0]

	g.Status.State = aws.StringValue(replicationGroup.Status)
	g.Status.UnsetAllDeprecatedConditions()

	switch g.Status.State {
	case v1alpha1.StatusAvailable:
		g.Status.SetReady()
	case v1alpha1.StatusCreating:
		g.Status.SetCreating()
		return true
	case v1alpha1.StatusDeleting:
		g.Status.SetDeleting()
		return false
	default:
		// TODO(negz): Don't requeue in this scenario? The instance could be
		// modifying, snapshotting, etc. It seems instances go into modifying by
		// themselves shortly after creation, seemingly as part of the creation
		// process?
		return true
	}

	ep := elasticache.ConnectionEndpoint(replicationGroup)
	g.Status.Endpoint = ep.Address
	g.Status.Port = ep.Port
	g.Status.ProviderID = aws.StringValue(replicationGroup.ReplicationGroupId)
	g.Status.ClusterEnabled = aws.BoolValue(replicationGroup.ClusterEnabled)
	g.Status.MemberClusters = replicationGroup.MemberClusters

	ccsNeedsUpdate, err := e.cacheClustersNeedUpdate(ctx, g)
	if err != nil {
		g.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}

	if !ccsNeedsUpdate && !elasticache.ReplicationGroupNeedsUpdate(g, replicationGroup) {
		return false
	}

	mrg := e.client.ModifyReplicationGroupRequest(elasticache.NewModifyReplicationGroupInput(g))
	mrg.SetContext(ctx)
	if _, err := mrg.Send(); err != nil {
		g.Status.SetFailed(reasonSyncingResource, err.Error())
		return true
	}

	return false
}

func (e *elastiCache) cacheClustersNeedUpdate(ctx context.Context, g *v1alpha1.ReplicationGroup) (bool, error) {
	for _, cc := range g.Status.MemberClusters {
		dcc := e.client.DescribeCacheClustersRequest(elasticache.NewDescribeCacheClustersInput(cc))
		dcc.SetContext(ctx)
		rsp, err := dcc.Send()
		if err != nil {
			return false, errors.Wrapf(err, "cannot describe cache cluster %s", cc)
		}

		// DescribeCacheClusters can return one or many cache clusters. We ask
		// for one cluster by name, so we should get either a single element
		// list or an error.
		if elasticache.CacheClusterNeedsUpdate(g, rsp.CacheClusters[0]) {
			return true, nil
		}
	}

	return false, nil
}

func (e *elastiCache) Delete(ctx context.Context, g *v1alpha1.ReplicationGroup) bool {
	if g.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		req := e.client.DeleteReplicationGroupRequest(elasticache.NewDeleteReplicationGroupInput(g))
		req.SetContext(ctx)
		if _, err := req.Send(); err != nil {
			g.Status.SetFailed(reasonDeletingResource, err.Error())
			return true
		}
	}
	g.Status.SetDeleting()
	meta.RemoveFinalizer(g, finalizerName)
	return false
}

// A connecter returns a createsyncdeletekeyer that can create, sync, and delete
// AWS Replication Group resources with an external store - for example the AWS API.
type connecter interface {
	Connect(context.Context, *v1alpha1.ReplicationGroup) (createsyncdeleter, error)
}

// providerConnecter is a connecter that returns a createsyncdeleter
// authenticated using credentials read from a Crossplane Provider resource.
type providerConnecter struct {
	kube      client.Client
	newClient func(creds []byte, region string) (elasticache.Client, error)
}

// Connect returns a createsyncdeletekeyer backed by the AWS API. AWS
// credentials are read from the Crossplane Provider referenced by the supplied
// ReplicationGroup.
func (c *providerConnecter) Connect(ctx context.Context, g *v1alpha1.ReplicationGroup) (createsyncdeleter, error) {
	p := &awsv1alpha1.Provider{}
	n := types.NamespacedName{Namespace: g.GetNamespace(), Name: g.Spec.ProviderRef.Name}
	if err := c.kube.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := c.newClient(s.Data[p.Spec.Secret.Key], p.Spec.Region)
	return &elastiCache{client: client}, errors.Wrap(err, "cannot create new AWS Replication Group client")
}

// Reconciler reconciles ReplicationGroups read from the Kubernetes API
// with an external store, typically the AWS API.
type Reconciler struct {
	connecter
	kube client.Client
}

// Add creates a new ReplicationGroup Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		connecter: &providerConnecter{kube: mgr.GetClient(), newClient: elasticache.NewClient},
		kube:      mgr.GetClient(),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	return c.Watch(&source.Kind{Type: &v1alpha1.ReplicationGroup{}}, &handler.EnqueueRequestForObject{})
}

// Reconcile Google AWS Replication Group resources with the AWS API.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.ReplicationGroupKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	rd := &v1alpha1.ReplicationGroup{}
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

	// The resource has been deleted from the API server. Delete from AWS.
	if rd.DeletionTimestamp != nil {
		return reconcile.Result{Requeue: client.Delete(ctx, rd)}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	// The group is unnamed. Assume it has not been created in AWS.
	if rd.Status.GroupName == "" {
		requeue, authToken := client.Create(ctx, rd)
		if err := r.upsertSecret(ctx, connectionSecretWithPassword(rd, authToken)); err != nil {
			rd.Status.SetFailed(reasonSyncingSecret, err.Error())
			requeue = true
		}
		return reconcile.Result{Requeue: requeue}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	if err := r.upsertSecret(ctx, connectionSecret(rd)); err != nil {
		rd.Status.SetFailed(reasonSyncingSecret, err.Error())
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
	}

	// The resource exists in the API server and AWS. Sync it.
	return reconcile.Result{Requeue: client.Sync(ctx, rd)}, errors.Wrapf(r.kube.Update(ctx, rd), "cannot update resource %s", req.NamespacedName)
}

func (r *Reconciler) upsertSecret(ctx context.Context, new *corev1.Secret) error {
	n := types.NamespacedName{Namespace: new.GetNamespace(), Name: new.GetName()}
	existing := &corev1.Secret{}
	if err := r.kube.Get(ctx, n, existing); err != nil {
		if kerrors.IsNotFound(err) {
			return errors.Wrapf(r.kube.Create(ctx, new), "cannot create secret %s", n)
		}
		return errors.Wrapf(err, "cannot get secret %s", n)
	}

	// If a key is set in both the existing and new secrets the new key wins,
	// but we preserve any keys set in the existing secret that aren't set in
	// the new secret.
	for k, v := range existing.Data {
		if _, ok := new.Data[k]; !ok {
			new.Data[k] = v
		}
	}

	return errors.Wrapf(r.kube.Update(ctx, new), "cannot update secret %s", n)
}

func connectionSecret(g *v1alpha1.ReplicationGroup) *corev1.Secret {
	ref := meta.AsOwner(meta.ReferenceTo(g, v1alpha1.ReplicationGroupGroupVersionKind))
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            g.ConnectionSecretName(),
			Namespace:       g.Namespace,
			OwnerReferences: []metav1.OwnerReference{ref},
		},

		// TODO(negz): Include the ports here too?
		Data: map[string][]byte{
			corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(g.Status.Endpoint),
		},
	}
}

func connectionSecretWithPassword(g *v1alpha1.ReplicationGroup, password string) *corev1.Secret {
	s := connectionSecret(g)
	if password != "" {
		s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = []byte(password)
	}
	return s
}
