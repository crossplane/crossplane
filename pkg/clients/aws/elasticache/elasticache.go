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

package elasticache

import (
	"fmt"
	"hash/fnv"

	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/elasticacheiface"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

// NamePrefix is the prefix for all created ElastiCache replication groups.
const NamePrefix = "ec"

// A Client handles CRUD operations for ElastiCache resources. This interface is
// compatible with the upstream AWS redis client.
type Client elasticacheiface.ElastiCacheAPI

// NewClient returns a new ElastiCache client. Credentials must be passed as
// JSON encoded data.
func NewClient(credentials []byte, region string) (Client, error) {
	cfg, err := aws.LoadConfig(credentials, aws.DefaultSection, region)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new AWS configuration")
	}
	return elasticache.New(*cfg), nil
}

// NewReplicationGroupID returns an identifier used to identify a Replication
// Group in the AWS API.
func NewReplicationGroupID(o metav1.Object) string {
	/*
		We want this ID to be deterministic and unique across time and space. We
		should always return the same ID for a given Kubernetes ReplicationGroup
		resource, but we should _not_ return the same ID for two identical
		resources that existed at different points in time.

		Assume a user creates a ReplicationGroup in the Kubernetes API, deletes
		it, then immediately creates a identical one. We want our controller to
		delete the first AWS Replication Group then create an identical
		replacement, not try to sync the lingering first AWS Replication Group
		with the second Kubernetes ReplicationGroup.

		Kubernetes resources have a UID for this reason. Crossplane (often) uses
		this UID to identify the cloud provider resources it manages. UIDs are
		in practice 36 character V1 UUID strings, but Kubernetes requires we
		treat them as opaque.

		ElastiCache requires that Replication Groups be identified by a string
		consisting of no more than 20 characters from the set [-a-z0-9], so we
		can't use the Kubernetes UID. Instead we hash it, use the 64 bit hash's
		16 character hex string, and hope we don't get a collision. ¯\_(ツ)_/¯
	*/

	// Hashes never error on write.
	h := fnv.New64a()
	h.Write([]byte(o.GetUID())) // nolint:errcheck
	return fmt.Sprintf("%s-%x", NamePrefix, h.Sum64())
}

// NewReplicationGroupDescription returns a description suitable for use with
// the AWS API.
func NewReplicationGroupDescription(g *v1alpha1.ReplicationGroup) string {
	return fmt.Sprintf("Crossplane managed %s %s/%s", v1alpha1.ReplicationGroupKindAPIVersion, g.GetNamespace(), g.GetName())
}

// TODO(negz): Determine whether we have to handle converting zero values to
// nil for the below types.

// NewCreateReplicationGroupInput returns ElastiCache replication group creation
// input suitable for use with the AWS API.
func NewCreateReplicationGroupInput(g *v1alpha1.ReplicationGroup, authToken string) *elasticache.CreateReplicationGroupInput {
	return &elasticache.CreateReplicationGroupInput{
		ReplicationGroupId:          aws.String(NewReplicationGroupID(g), aws.FieldRequired),
		ReplicationGroupDescription: aws.String(NewReplicationGroupDescription(g), aws.FieldRequired),

		// The AWS API docs state these fields are not required, but they are.
		// The APi returns an error if they're omitted.
		Engine:        aws.String(v1alpha1.CacheEngineRedis, aws.FieldRequired),
		CacheNodeType: aws.String(g.Spec.CacheNodeType, aws.FieldRequired),

		AtRestEncryptionEnabled:    aws.Bool(g.Spec.AtRestEncryptionEnabled),
		AuthToken:                  aws.String(authToken),
		AutomaticFailoverEnabled:   aws.Bool(g.Spec.AutomaticFailoverEnabled),
		CacheParameterGroupName:    aws.String(g.Spec.CacheParameterGroupName),
		CacheSecurityGroupNames:    g.Spec.CacheSecurityGroupNames,
		CacheSubnetGroupName:       aws.String(g.Spec.CacheSubnetGroupName),
		EngineVersion:              aws.String(g.Spec.EngineVersion),
		NodeGroupConfiguration:     newNodeGroupConfigurations(g),
		NotificationTopicArn:       aws.String(g.Spec.NotificationTopicARN),
		NumCacheClusters:           aws.Int64(g.Spec.NumCacheClusters),
		NumNodeGroups:              aws.Int64(g.Spec.NumNodeGroups),
		Port:                       aws.Int64(g.Spec.Port),
		PreferredCacheClusterAZs:   g.Spec.PreferredCacheClusterAZs,
		PreferredMaintenanceWindow: aws.String(g.Spec.PreferredMaintenanceWindow),
		ReplicasPerNodeGroup:       aws.Int64(g.Spec.ReplicasPerNodeGroup),
		SecurityGroupIds:           g.Spec.SecurityGroupIDs,
		SnapshotArns:               g.Spec.SnapshotARNs,
		SnapshotName:               aws.String(g.Spec.SnapshotName),
		SnapshotRetentionLimit:     aws.Int64(g.Spec.SnapshotRetentionLimit),
		SnapshotWindow:             aws.String(g.Spec.SnapshotWindow),
		TransitEncryptionEnabled:   aws.Bool(g.Spec.TransitEncryptionEnabled),
	}
}

func newNodeGroupConfigurations(g *v1alpha1.ReplicationGroup) []elasticache.NodeGroupConfiguration {
	if len(g.Spec.NodeGroupConfiguration) == 0 {
		return nil
	}
	nc := make([]elasticache.NodeGroupConfiguration, len(g.Spec.NodeGroupConfiguration))
	for i, cfg := range g.Spec.NodeGroupConfiguration {
		nc[i] = elasticache.NodeGroupConfiguration{
			PrimaryAvailabilityZone:  aws.String(cfg.PrimaryAvailabilityZone),
			ReplicaAvailabilityZones: cfg.ReplicaAvailabilityZones,
			ReplicaCount:             aws.Int64(cfg.ReplicaCount),
			Slots:                    aws.String(cfg.Slots),
		}
	}
	return nc
}

// NewModifyReplicationGroupInput returns ElastiCache replication group
// modification input suitable for use with the AWS API.
func NewModifyReplicationGroupInput(g *v1alpha1.ReplicationGroup) *elasticache.ModifyReplicationGroupInput {
	return &elasticache.ModifyReplicationGroupInput{
		ReplicationGroupId: aws.String(NewReplicationGroupID(g), aws.FieldRequired),

		// TODO(negz): Should this be a configurable part of the replication
		// group spec? If we did wait until the next maintenance window to apply
		// changes we'd need some way to account for the pending changes during
		// our sync logic.
		ApplyImmediately: aws.Bool(true),

		AutomaticFailoverEnabled:   aws.Bool(g.Spec.AutomaticFailoverEnabled),
		CacheNodeType:              aws.String(g.Spec.CacheNodeType),
		CacheParameterGroupName:    aws.String(g.Spec.CacheParameterGroupName),
		CacheSecurityGroupNames:    g.Spec.CacheSecurityGroupNames,
		EngineVersion:              aws.String(g.Spec.EngineVersion),
		NotificationTopicArn:       aws.String(g.Spec.NotificationTopicARN),
		PreferredMaintenanceWindow: aws.String(g.Spec.PreferredMaintenanceWindow),
		SecurityGroupIds:           g.Spec.SecurityGroupIDs,
		SnapshotRetentionLimit:     aws.Int64(g.Spec.SnapshotRetentionLimit),
		SnapshotWindow:             aws.String(g.Spec.SnapshotWindow),
	}
}

// NewDeleteReplicationGroupInput returns ElastiCache replication group deletion
// input suitable for use with the AWS API.
func NewDeleteReplicationGroupInput(g *v1alpha1.ReplicationGroup) *elasticache.DeleteReplicationGroupInput {
	return &elasticache.DeleteReplicationGroupInput{ReplicationGroupId: aws.String(NewReplicationGroupID(g), aws.FieldRequired)}
}

// NewDescribeReplicationGroupsInput returns ElastiCache replication group describe
// input suitable for use with the AWS API.
func NewDescribeReplicationGroupsInput(g *v1alpha1.ReplicationGroup) *elasticache.DescribeReplicationGroupsInput {
	return &elasticache.DescribeReplicationGroupsInput{ReplicationGroupId: aws.String(NewReplicationGroupID(g))}
}

// NewDescribeCacheClustersInput returns ElastiCache cache cluster describe
// input suitable for use with the AWS API.
func NewDescribeCacheClustersInput(cluster string) *elasticache.DescribeCacheClustersInput {
	return &elasticache.DescribeCacheClustersInput{CacheClusterId: aws.String(cluster)}
}

// ReplicationGroupNeedsUpdate returns true if the supplied Kubernetes resource
// differs from the supplied AWS resource. It considers only fields that can be
// modified in place without deleting and recreating the group, and only fields
// that are first class properties of the AWS replication group.
func ReplicationGroupNeedsUpdate(kube *v1alpha1.ReplicationGroup, rg elasticache.ReplicationGroup) bool {
	switch {
	case kube.Spec.AutomaticFailoverEnabled != automaticFailoverEnabled(rg):
		return true
	case kube.Spec.CacheNodeType != aws.StringValue(rg.CacheNodeType):
		return true
	case kube.Spec.SnapshotRetentionLimit != aws.Int64Value(rg.SnapshotRetentionLimit):
		return true
	// AWS will return a snapshot window if we don't specify one.
	case kube.Spec.SnapshotWindow != "" && kube.Spec.SnapshotWindow != aws.StringValue(rg.SnapshotWindow):
		return true
	}
	return false
}

func automaticFailoverEnabled(rg elasticache.ReplicationGroup) bool {
	return rg.AutomaticFailover == elasticache.AutomaticFailoverStatusEnabled || rg.AutomaticFailover == elasticache.AutomaticFailoverStatusEnabling
}

// CacheClusterNeedsUpdate returns true if the supplied Kubernetes resource
// differs from the supplied AWS resource. It considers only fields that can be
// modified in place without deleting and recreating the group, and only fields
// that are first class properties of the AWS replication group.
func CacheClusterNeedsUpdate(kube *v1alpha1.ReplicationGroup, cc elasticache.CacheCluster) bool { // nolint:gocyclo
	// AWS will set and return a default version if we don't specify one.
	if v := kube.Spec.EngineVersion; v != "" && v != aws.StringValue(cc.EngineVersion) {
		return true
	}

	// AWS will set and return a default parameter group if we don't specify one.
	// TODO(negz): Do we care about CacheNodeIdsToReboot or ParameterApplyStatus?
	if pg, name := cc.CacheParameterGroup, kube.Spec.CacheParameterGroupName; pg != nil && name != "" && name != aws.StringValue(pg.CacheParameterGroupName) {
		return true
	}

	// TODO(negz): Do we care about TopicStatus?
	if nc := cc.NotificationConfiguration; nc != nil && kube.Spec.NotificationTopicARN != aws.StringValue(nc.TopicArn) {
		return true
	}

	// AWS will set and return a maintenance window if we don't specify one.
	if w := kube.Spec.PreferredMaintenanceWindow; w != "" && w != aws.StringValue(cc.PreferredMaintenanceWindow) {
		return true
	}

	return sgIDsNeedUpdate(kube.Spec.SecurityGroupIDs, cc.SecurityGroups) || sgNamesNeedUpdate(kube.Spec.CacheSecurityGroupNames, cc.CacheSecurityGroups)
}

func sgIDsNeedUpdate(kube []string, cc []elasticache.SecurityGroupMembership) bool {
	if len(kube) != len(cc) {
		return true
	}

	// TODO(negz): Do we care about sg.Status?
	csgs := map[string]bool{}
	for _, sg := range cc {
		csgs[aws.StringValue(sg.SecurityGroupId)] = true
	}

	for _, sg := range kube {
		if !csgs[sg] {
			return true
		}
	}

	return false
}

func sgNamesNeedUpdate(kube []string, cc []elasticache.CacheSecurityGroupMembership) bool {
	if len(kube) != len(cc) {
		return true
	}

	// TODO(negz): Do we care about sg.Status?
	csgs := map[string]bool{}
	for _, sg := range cc {
		csgs[aws.StringValue(sg.CacheSecurityGroupName)] = true
	}

	for _, sg := range kube {
		if !csgs[sg] {
			return true
		}
	}

	return false
}

// Endpoint represents the address and port used to connect to an ElastiCache
// Replication Group.
type Endpoint struct {
	Address string
	Port    int
}

func newEndpoint(e *elasticache.Endpoint) Endpoint {
	if e == nil {
		return Endpoint{}
	}

	return Endpoint{Address: aws.StringValue(e.Address), Port: aws.Int64Value(e.Port)}
}

// ConnectionEndpoint returns the connection endpoint for a Replication Group.
// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/Endpoints.html
func ConnectionEndpoint(rg elasticache.ReplicationGroup) Endpoint {
	// "Cluster enabled" Replication Groups have multiple node groups, and an
	// explicit configuration endpoint that should be used for read and write.
	if aws.BoolValue(rg.ClusterEnabled) {
		return newEndpoint(rg.ConfigurationEndpoint)
	}

	// "Cluster disabled" Replication Groups have a single node group, with a
	// primary endpoint that should be used for write. Any node's endpoint can
	// be used for read, but we support only a single endpoint so we return the
	// primary's.
	if len(rg.NodeGroups) == 1 {
		return newEndpoint(rg.NodeGroups[0].PrimaryEndpoint)
	}

	// If the AWS API docs are to be believed we should never get here.
	return Endpoint{}
}
