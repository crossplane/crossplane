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

package v1alpha1

import (
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// ReplicationGroup states.
const (
	StatusCreating     = "creating"
	StatusAvailable    = "available"
	StatusModifying    = "modifying"
	StatusDeleting     = "deleting"
	StatusCreateFailed = "create-failed"
	StatusSnapshotting = "snapshotting"
)

// Supported cache engines.
const (
	CacheEngineRedis = "redis"
)

// TODO(negz): Lookup supported patch versions in the ElastiCache API?
// AWS requires we specify desired Redis versions down to the patch version,
// but the RedisCluster resource claim supports only minor versions (which are
// the lowest common denominator between supported clouds). We perform this
// lookup in the claim provisioning code, which does not have an AWS client
// plumbed in to perform such a lookup.
// https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_DescribeCacheEngineVersions.html

// MinorVersion represents a supported minor version of Redis.
type MinorVersion string

// PatchVersion represents a supported patch version of Redis.
type PatchVersion string

// UnsupportedVersion indicates the requested MinorVersion is unsupported.
const UnsupportedVersion PatchVersion = ""

// LatestSupportedPatchVersion returns the latest supported patch version
// for a given minor version.
var LatestSupportedPatchVersion = map[MinorVersion]PatchVersion{
	MinorVersion("5.0"): PatchVersion("5.0.0"),
	MinorVersion("4.0"): PatchVersion("4.0.10"),
	MinorVersion("3.2"): PatchVersion("3.2.10"),
	MinorVersion("2.8"): PatchVersion("2.8.24"),
}

// ReplicationGroupSpec defines the desired state of ReplicationGroup
// Most fields map directly to an AWS ReplicationGroup resource.
// https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_CreateReplicationGroup.html#API_CreateReplicationGroup_RequestParameters
type ReplicationGroupSpec struct {

	// AtRestEncryptionEnabled enables encryption at rest when set to true.
	//
	// You cannot modify the value of AtRestEncryptionEnabled after the replication
	// group is created. To enable encryption at rest on a replication group you
	// must set AtRestEncryptionEnabled to true when you create the replication
	// group.
	AtRestEncryptionEnabled bool `json:"atRestEncryptionEnabled,omitempty"`

	// AuthEnabled enables mandatory authentication when connecting to the
	// managed replication group. AuthEnabled requires TransitEncryptionEnabled
	// to be true.
	//
	// While ReplicationGroupSpec mirrors the fields of the upstream replication
	// group object as closely as possible, we expose a boolean here rather than
	// requiring the operator pass in a string authentication token. Crossplane
	// will generate a token automatically and expose it via a Secret.
	AuthEnabled bool `json:"authEnabled,omitempty"`

	// AutomaticFailoverEnabled specifies whether a read-only replica is
	// automatically promoted to read/write primary if the existing primary
	// fails. If true, Multi-AZ is enabled for this replication group. If false,
	// Multi-AZ is disabled for this replication group.
	//
	// AutomaticFailoverEnabled must be enabled for Redis (cluster mode enabled)
	// replication groups.
	AutomaticFailoverEnabled bool `json:"automaticFailoverEnabled,omitempty"`

	// CacheNodeType specifies the compute and memory capacity of the nodes in
	// the node group (shard).
	CacheNodeType string `json:"cacheNodeType"`

	// CacheParameterGroupName specifies the name of the parameter group to
	// associate with this replication group. If this argument is omitted, the
	// default cache parameter group for the specified engine is used.
	CacheParameterGroupName string `json:"cacheParameterGroupName,omitempty"`

	// CacheSecurityGroupNames specifies a list of cache security group names to
	// associate with this replication group.
	CacheSecurityGroupNames []string `json:"cacheSecurityGroupNames,omitempty"`

	// CacheSubnetGroupName specifies the name of the cache subnet group to be
	// used for the replication group. If you're going to launch your cluster in
	// an Amazon VPC, you need to create a subnet group before you start
	// creating a cluster.
	CacheSubnetGroupName string `json:"cacheSubnetGroupName,omitempty"`

	// EngineVersion specifies the version number of the cache engine to be
	// used for the clusters in this replication group. To view the supported
	// cache engine versions, use the DescribeCacheEngineVersions operation.
	EngineVersion string `json:"engineVersion,omitempty"`

	// NodeGroupConfiguration specifies a list of node group (shard)
	// configuration options.
	NodeGroupConfiguration []NodeGroupConfigurationSpec `json:"nodeGroupConfiguration,omitempty"`

	// NotificationTopicARN specifies the Amazon Resource Name (ARN) of the
	// Amazon Simple Notification Service (SNS) topic to which notifications are
	// sent. The Amazon SNS topic owner must be the same as the cluster owner.
	NotificationTopicARN string `json:"notificationTopicArn,omitempty"`

	// NumCacheClusters specifies the number of clusters this replication group
	// initially has. This parameter is not used if there is more than one node
	// group (shard). You should use ReplicasPerNodeGroup instead.
	//
	// If AutomaticFailoverEnabled is true, the value of this parameter must be
	// at least 2. If AutomaticFailoverEnabled is false you can omit this
	// parameter (it will default to 1), or you can explicitly set it to a value
	// between 2 and 6.
	NumCacheClusters int `json:"numCacheClusters,omitempty"`

	// NumNodeGroups specifies the number of node groups (shards) for this Redis
	// (cluster mode enabled) replication group. For Redis (cluster mode
	// disabled) either omit this parameter or set it to 1.
	NumNodeGroups int `json:"numNodeGroups,omitempty"`

	// Port number on which each member of the replication group accepts
	// connections.
	Port int `json:"port,omitempty"`

	// PreferredCacheClusterAZs specifies a list of EC2 Availability Zones in
	// which the replication group's clusters are created. The order of the
	// Availability Zones in the list is the order in which clusters are
	// allocated. The primary cluster is created in the first AZ in the list.
	//
	// This parameter is not used if there is more than one node group (shard).
	// You should use NodeGroupConfiguration instead.
	//
	// The number of Availability Zones listed must equal the value of
	// NumCacheClusters.
	PreferredCacheClusterAZs []string `json:"preferredCacheClusterAzs,omitempty"`

	// PreferredMaintenanceWindow specifies the weekly time range during which
	// maintenance on the cluster is performed. It is specified as a range in
	// the format ddd:hh24:mi-ddd:hh24:mi (24H Clock UTC). The minimum
	// maintenance window is a 60 minute period.
	//
	// Example: sun:23:00-mon:01:30
	PreferredMaintenanceWindow string `json:"preferredMaintenanceWindow,omitempty"`

	// ReplicasPerNodeGroup specifies the number of replica nodes in each node
	// group (shard). Valid values are 0 to 5.
	ReplicasPerNodeGroup int `json:"replicasPerNodeGroup,omitempty"`

	// SecurityGroupIDs specifies one or more Amazon VPC security groups
	// associated with this replication group. Use this parameter only when you
	// are creating a replication group in an Amazon VPC.
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty"`

	// SnapshotARNs specifies a list of Amazon Resource Names (ARN) that
	// uniquely identify the Redis RDB snapshot files stored in Amazon S3. The
	// snapshot files are used to populate the new replication group. The Amazon
	// S3 object name in the ARN cannot contain any commas. The new replication
	// group will have the number of node groups (console: shards) specified by
	// the parameter NumNodeGroups or the number of node groups configured by
	// NodeGroupConfiguration regardless of the number of ARNs specified here.
	SnapshotARNs []string `json:"snapshotArns,omitempty"`

	// SnapshotName specifies the name of a snapshot from which to restore data
	// into the new replication group. The snapshot status changes to restoring
	// while the new replication group is being created.
	SnapshotName string `json:"snapshotName,omitempty"`

	// SnapshotRetentionLimit specifies the number of days for which ElastiCache
	// retains automatic snapshots before deleting them. For example, if you set
	// SnapshotRetentionLimit to 5, a snapshot that was taken today is retained
	// for 5 days before being deleted.
	SnapshotRetentionLimit int `json:"snapshotRetentionLimit,omitempty"`

	// SnapshotWindow specifies the daily time range (in UTC) during which
	// ElastiCache begins taking a daily snapshot of your node group (shard).
	//
	// Example: 05:00-09:00
	//
	// If you do not specify this parameter, ElastiCache automatically chooses an
	// appropriate time range.
	SnapshotWindow string `json:"snapshotWindow,omitempty"`

	// TransitEncryptionEnabled enables in-transit encryption when set to true.
	//
	// You cannot modify the value of TransitEncryptionEnabled after the cluster
	// is created. To enable in-transit encryption on a cluster you must
	// TransitEncryptionEnabled to true when you create a cluster.
	TransitEncryptionEnabled bool `json:"transitEncryptionEnabled,omitempty"`

	// Kubernetes object references
	ClaimRef            *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef            *v1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// NodeGroupConfigurationSpec specifies the configuration of a node group within
// a replication group.
type NodeGroupConfigurationSpec struct {
	// PrimaryAvailabilityZone specifies the Availability Zone where the primary
	// node of this node group (shard) is launched.
	PrimaryAvailabilityZone string `json:"primaryAvailabilityZone,omitempty"`

	// ReplicaAvailabilityZones specifies a list of Availability Zones to be
	// used for the read replicas. The number of Availability Zones in this list
	// must match the value of ReplicaCount or ReplicasPerNodeGroup if not
	// specified.
	ReplicaAvailabilityZones []string `json:"replicaAvailabilityZones,omitempty"`

	// ReplicaCount specifies the number of read replica nodes in this node
	// group (shard).
	ReplicaCount int `json:"replicaCount,omitempty"`

	// Slots specifies the keyspace for a particular node group. Keyspaces range
	// from 0 to 16,383. The string is in the format startkey-endkey.
	//
	// Example: "0-3999"
	Slots string `json:"slots,omitempty"`
}

// ReplicationGroupStatus defines the observed state of ReplicationGroup
type ReplicationGroupStatus struct {
	corev1alpha1.DeprecatedConditionedStatus
	corev1alpha1.BindingStatusPhase
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// ProviderID is the external ID to identify this resource in the cloud
	// provider
	ProviderID string `json:"providerID,omitempty"`

	// Endpoint of the Replication Group used in connection strings.
	Endpoint string `json:"endpoint,omitempty"`

	// Port at which the Replication Group endpoint is listening.
	Port int `json:"port,omitempty"`

	// ClusterEnabled indicates whether cluster mode is enabled, i.e. whether
	// this replication group's data can be partitioned across multiple shards.
	ClusterEnabled bool `json:"clusterEnabled,omitempty"`

	// MemberClusters that are part of this replication group.
	MemberClusters []string `json:"memberClusters,omitempty"`

	// Groupname of the Replication Group.
	GroupName string `json:"groupName,omitempty"`

	// TODO(negz): Support PendingModifiedValues?
	// https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroupPendingModifiedValues.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReplicationGroup is the Schema for the instances API
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.engineVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ReplicationGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicationGroupSpec   `json:"spec,omitempty"`
	Status ReplicationGroupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReplicationGroupList contains a list of ReplicationGroup
type ReplicationGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicationGroup `json:"items"`
}

// NewReplicationGroupSpec creates a new ReplicationGroupSpec
// from the given properties map.
func NewReplicationGroupSpec(properties map[string]string) *ReplicationGroupSpec {
	spec := &ReplicationGroupSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,

		// Note that these keys should match the JSON tags of their respective
		// ReplicationGroupSpec fields.
		CacheNodeType:              properties["cacheNodeType"],
		CacheParameterGroupName:    properties["cacheParameterGroupName"],
		CacheSubnetGroupName:       properties["cacheSubnetGroupName"],
		EngineVersion:              properties["engineVersion"],
		NotificationTopicARN:       properties["notificationTopicArn"],
		PreferredMaintenanceWindow: properties["preferredMaintenanceWindow"],
		SnapshotName:               properties["snapshotName"],
		SnapshotWindow:             properties["snapshotWindow"],
		CacheSecurityGroupNames:    parseSlice(properties["cacheSecurityGroupNames"]),
		PreferredCacheClusterAZs:   parseSlice(properties["preferredCacheClusterAzs"]),
		SecurityGroupIDs:           parseSlice(properties["securityGroupIds"]),

		// TODO(negz): Support NodeGroupConfiguration? It's awkward to extract a
		// slice of structs from a one dimensional map - we might use Terraform
		// style nodeGroupConfiguration.0.slots style notation. Without
		// NodeGroupConfiguration we cannot support SnapshotARNs.
	}

	if b, err := strconv.ParseBool(properties["atRestEncryptionEnabled"]); err == nil {
		spec.AtRestEncryptionEnabled = b
	}
	if b, err := strconv.ParseBool(properties["authEnabled"]); err == nil {
		spec.AuthEnabled = b
	}
	if b, err := strconv.ParseBool(properties["automaticFailoverEnabled"]); err == nil {
		spec.AutomaticFailoverEnabled = b
	}
	if b, err := strconv.ParseBool(properties["transitEncryptionEnabled"]); err == nil {
		spec.TransitEncryptionEnabled = b
	}
	if i, err := strconv.Atoi(properties["numCacheClusters"]); err == nil {
		spec.NumCacheClusters = i
	}
	if i, err := strconv.Atoi(properties["numNodeGroups"]); err == nil {
		spec.NumNodeGroups = i
	}
	if i, err := strconv.Atoi(properties["port"]); err == nil {
		spec.Port = i
	}
	if i, err := strconv.Atoi(properties["replicasPerNodeGroup"]); err == nil {
		spec.ReplicasPerNodeGroup = i
	}
	if i, err := strconv.Atoi(properties["snapshotRetentionLimit"]); err == nil {
		spec.SnapshotRetentionLimit = i
	}

	return spec
}

// parseSlice parses a string of comma separated strings, for example
// "value1, value2", into a string slice.
func parseSlice(s string) []string {
	if s == "" {
		return nil
	}
	sl := make([]string, 0, strings.Count(s, ",")+1)
	for _, sub := range strings.Split(s, ",") {
		sl = append(sl, strings.TrimSpace(sub))
	}
	return sl
}

// ConnectionSecretName returns a secret name from the reference
func (c *ReplicationGroup) ConnectionSecretName() string {
	if c.Spec.ConnectionSecretRef.Name == "" {
		c.Spec.ConnectionSecretRef.Name = c.Name
	}

	return c.Spec.ConnectionSecretRef.Name
}

// IsAvailable for usage/binding
func (c *ReplicationGroup) IsAvailable() bool {
	return c.Status.State == StatusAvailable
}

// IsBound determines if the resource is in a bound binding state
func (c *ReplicationGroup) IsBound() bool {
	return c.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound sets the binding state of this resource
func (c *ReplicationGroup) SetBound(state bool) {
	if state {
		c.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		c.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
