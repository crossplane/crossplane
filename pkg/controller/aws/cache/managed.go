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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/elasticache"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// Error strings.
const (
	errNewClient                = "cannot create new ElastiCache client"
	errNotReplicationGroup      = "managed resource is not an ElastiCache replication group"
	errDescribeReplicationGroup = "cannot describe ElastiCache replication group"
	errGenerateAuthToken        = "cannot generate ElastiCache auth token"
	errCreateReplicationGroup   = "cannot create ElastiCache replication group"
	errModifyReplicationGroup   = "cannot modify ElastiCache replication group"
	errDescribeCacheCluster     = "cannot describe ElastiCache cache cluster"
	errDeleteReplicationGroup   = "cannot delete ElastiCache replication group"
)

// Note this is the length of the generated random byte slice before base64
// encoding, which adds ~33% overhead. ElastiCache allows auth tokens to be
const maxAuthTokenData = 32

// AddManaged adds a controller that reconciles ReplicationGroup managed
// resources claims by creating ElastiCache replication groups in AWS.
func AddManaged(mgr manager.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(v1alpha1.ReplicationGroupGroupVersionKind),
		resource.WithExternalConnecter(&connecter{client: mgr.GetClient()}))

	name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.ReplicationGroupKind, v1alpha1.Group))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	return c.Watch(&source.Kind{Type: &v1alpha1.ReplicationGroup{}}, &handler.EnqueueRequestForObject{})
}

type connecter struct {
	client client.Client
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	g, ok := mg.(*v1alpha1.ReplicationGroup)
	if !ok {
		return nil, errors.New(errNotReplicationGroup)
	}

	p := &awsv1alpha1.Provider{}
	n := meta.NamespacedNameOf(g.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}

	client, err := elasticache.NewClient(s.Data[p.Spec.Secret.Key], p.Spec.Region)
	return &external{client: client}, errors.Wrap(err, errNewClient)
}

type external struct{ client elasticache.Client }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	g, ok := mg.(*v1alpha1.ReplicationGroup)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotReplicationGroup)
	}

	dr := e.client.DescribeReplicationGroupsRequest(elasticache.NewDescribeReplicationGroupsInput(g))
	dr.SetContext(ctx)
	rsp, err := dr.Send()
	if elasticache.IsNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errDescribeReplicationGroup)
	}

	// DescribeReplicationGroups can return one or many replication groups. We
	// ask for one group by name, so we should get either a single element list
	// or an error.
	existing := rsp.ReplicationGroups[0]
	g.Status.State = aws.StringValue(existing.Status)
	g.Status.Endpoint = elasticache.ConnectionEndpoint(existing).Address
	g.Status.Port = elasticache.ConnectionEndpoint(existing).Port
	g.Status.ProviderID = aws.StringValue(existing.ReplicationGroupId)
	g.Status.ClusterEnabled = aws.BoolValue(existing.ClusterEnabled)
	g.Status.MemberClusters = existing.MemberClusters

	switch g.Status.State {
	case v1alpha1.StatusAvailable:
		g.Status.SetConditions(corev1alpha1.Available())
		resource.SetBindable(g)
	case v1alpha1.StatusCreating:
		g.Status.SetConditions(corev1alpha1.Creating())
	case v1alpha1.StatusDeleting:
		g.Status.SetConditions(corev1alpha1.Deleting())
	}

	o := resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}

	if g.Status.Endpoint != "" {
		o.ConnectionDetails[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(g.Status.Endpoint)
	}

	return o, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	g, ok := mg.(*v1alpha1.ReplicationGroup)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotReplicationGroup)
	}

	g.Status.SetConditions(corev1alpha1.Creating())
	g.Status.GroupName = elasticache.NewReplicationGroupID(g)

	// Our create request will fail if auth is enabled but transit encryption is
	// not. We don't check for the latter here because it's less surprising to
	// submit the request as the operator intended and let the reconcile fail
	// with an explanatory message from AWS explaining that transit encryption
	// is required.
	if !g.Spec.AuthEnabled {
		token := ""
		r := e.client.CreateReplicationGroupRequest(elasticache.NewCreateReplicationGroupInput(g, token))
		r.SetContext(ctx)
		_, err := r.Send()
		return resource.ExternalCreation{}, errors.Wrap(err, errCreateReplicationGroup)
	}

	token, err := util.GeneratePassword(maxAuthTokenData)
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errGenerateAuthToken)
	}

	r := e.client.CreateReplicationGroupRequest(elasticache.NewCreateReplicationGroupInput(g, token))
	r.SetContext(ctx)
	if _, err := r.Send(); err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errCreateReplicationGroup)
	}

	c := resource.ExternalCreation{
		ConnectionDetails: resource.ConnectionDetails{
			corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(token),
		},
	}

	return c, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	g, ok := mg.(*v1alpha1.ReplicationGroup)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotReplicationGroup)
	}

	dr := e.client.DescribeReplicationGroupsRequest(elasticache.NewDescribeReplicationGroupsInput(g))
	dr.SetContext(ctx)
	rsp, err := dr.Send()
	if err != nil {
		return resource.ExternalUpdate{}, errors.Wrap(err, errDescribeReplicationGroup)
	}

	// DescribeReplicationGroups can return one or many replication groups. We
	// ask for one group by name, so we should get either a single element list
	//  or an error.
	if elasticache.ReplicationGroupNeedsUpdate(g, rsp.ReplicationGroups[0]) {
		mr := e.client.ModifyReplicationGroupRequest(elasticache.NewModifyReplicationGroupInput(g))
		mr.SetContext(ctx)
		_, err = mr.Send()
		return resource.ExternalUpdate{}, errors.Wrap(err, errModifyReplicationGroup)
	}

	for _, cc := range g.Status.MemberClusters {
		dcc := e.client.DescribeCacheClustersRequest(elasticache.NewDescribeCacheClustersInput(cc))
		dcc.SetContext(ctx)
		rsp, err := dcc.Send()
		if err != nil {
			return resource.ExternalUpdate{}, errors.Wrapf(err, errDescribeCacheCluster)
		}

		// DescribeCacheClusters can return one or many cache clusters. We ask
		// for one cluster by name, so we should get either a single element
		// list or an error.
		if elasticache.CacheClusterNeedsUpdate(g, rsp.CacheClusters[0]) {
			mr := e.client.ModifyReplicationGroupRequest(elasticache.NewModifyReplicationGroupInput(g))
			mr.SetContext(ctx)
			_, err = mr.Send()
			return resource.ExternalUpdate{}, errors.Wrap(err, errModifyReplicationGroup)
		}
	}

	return resource.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	g, ok := mg.(*v1alpha1.ReplicationGroup)
	if !ok {
		return errors.New(errNotReplicationGroup)
	}

	req := e.client.DeleteReplicationGroupRequest(elasticache.NewDeleteReplicationGroupInput(g))
	req.SetContext(ctx)
	if _, err := req.Send(); resource.Ignore(elasticache.IsNotFound, err) != nil {
		return errors.Wrap(err, errDeleteReplicationGroup)
	}

	return nil
}
