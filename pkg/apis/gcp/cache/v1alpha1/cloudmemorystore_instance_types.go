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

	"google.golang.org/genproto/googleapis/cloud/redis/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// Cloud Memorystore instance states.
var (
	StateUnspecified = redis.Instance_STATE_UNSPECIFIED.String()
	StateCreating    = redis.Instance_CREATING.String()
	StateReady       = redis.Instance_READY.String()
	StateUpdating    = redis.Instance_UPDATING.String()
	StateDeleting    = redis.Instance_DELETING.String()
	StateRepairing   = redis.Instance_REPAIRING.String()
	StateMaintenance = redis.Instance_MAINTENANCE.String()
)

// Cloud Memorystore instance tiers.
var (
	TierBasic      = redis.Instance_BASIC.String()
	TierStandardHA = redis.Instance_STANDARD_HA.String()
)

// CloudMemorystoreInstanceSpec defines the desired state of CloudMemorystoreInstance
// Most fields map directly to a GCP Instance resource.
// https://cloud.google.com/memorystore/docs/redis/reference/rest/v1/projects.locations.instances#Instance
type CloudMemorystoreInstanceSpec struct {
	// Region in which to create this CloudMemorystore cluster.
	Region string `json:"region"`

	// Tier specifies the replication level of the Redis cluster. BASIC provides
	// a single Redis instance with no high availability. STANDARD_HA provides a
	// cluster of two Redis instances in distinct availability zones.
	// https://cloud.google.com/memorystore/docs/redis/redis-tiers
	// +kubebuilder:validation:Enum=BASIC,STANDARD_HA
	Tier string `json:"tier"`

	// LocationID specifies the zone where the instance will be provisioned. If
	// not provided, the service will choose a zone for the instance. For
	// STANDARD_HA tier, instances will be created across two zones for
	// protection against zonal failures.
	LocationID string `json:"locationId,omitempty"`

	// AlternativeLocationID is only applicable to STANDARD_HA tier, which
	// protects the instance against zonal failures by provisioning it across
	// two zones. If provided, it must be a different zone from the one provided
	// in locationId.
	AlternativeLocationID string `json:"alternativeLocationId,omitempty"`

	// MemorySizeGB specifies the Redis memory size in GiB.
	MemorySizeGB int `json:"memorySizeGb"`

	// ReservedIPRange specifies the CIDR range of internal addresses that are
	// reserved for this instance. If not provided, the service will choose an
	// unused /29 block, for example, 10.0.0.0/29 or 192.168.0.0/29. Ranges must
	// be unique and non-overlapping with existing subnets in an authorized
	// network.
	ReservedIPRange string `json:"reservedIpRange,omitempty"`

	// AuthorizedNetwork specifies the full name of the Google Compute Engine
	// network to which the instance is connected. If left unspecified, the
	// default network will be used.
	AuthorizedNetwork string `json:"authorizedNetwork,omitempty"`

	// RedisVersion specifies the version of Redis software. If not provided,
	// latest supported version will be used. Updating the version will perform
	// an upgrade/downgrade to the new version. Currently, the supported values
	// are REDIS_3_2 for Redis 3.2.
	// +kubebuilder:validation:Enum=REDIS_3_2
	RedisVersion string `json:"redisVersion,omitempty"`

	// RedisConfigs specifies Redis configuration parameters, according to
	// http://redis.io/topics/config. Currently, the only supported parameters
	// are:
	// * maxmemory-policy
	// * notify-keyspace-events
	RedisConfigs map[string]string `json:"redisConfigs,omitempty"`

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// CloudMemorystoreInstanceStatus defines the observed state of CloudMemorystoreInstance
type CloudMemorystoreInstanceStatus struct {
	corev1alpha1.DeprecatedConditionedStatus
	corev1alpha1.BindingStatusPhase
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// ProviderID is the external ID to identify this resource in the cloud
	// provider
	ProviderID string `json:"providerID,omitempty"`

	// CurrentLocationID is the current zone where the Redis endpoint is placed.
	// For Basic Tier instances, this will always be the same as the locationId
	// provided by the user at creation time. For Standard Tier instances, this
	// can be either locationId or alternativeLocationId and can change after a
	// failover event.
	CurrentLocationID string `json:"currentLocationId,omitempty"`

	// Endpoint of the Cloud Memorystore instance used in connection strings.
	Endpoint string `json:"endpoint,omitempty"`

	// Port at which the Cloud Memorystore instance endpoint is listening.
	Port int `json:"port,omitempty"`

	// InstanceName of the Cloud Memorystore instance. Does not include the
	// project and location (region) IDs. e.g. 'foo', not
	// 'projects/fooproj/locations/us-foo1/instances/foo'
	InstanceName string `json:"instanceName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudMemorystoreInstance is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=cache.gcp
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.redisVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type CloudMemorystoreInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudMemorystoreInstanceSpec   `json:"spec,omitempty"`
	Status CloudMemorystoreInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudMemorystoreInstanceList contains a list of CloudMemorystoreInstance
type CloudMemorystoreInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudMemorystoreInstance `json:"items"`
}

// NewCloudMemorystoreInstanceSpec creates a new CloudMemorystoreInstanceSpec
// from the given properties map.
func NewCloudMemorystoreInstanceSpec(properties map[string]string) *CloudMemorystoreInstanceSpec {
	spec := &CloudMemorystoreInstanceSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,

		// Note that these keys should match the JSON tags of their respective
		// CloudMemorystoreInstanceSpec fields.
		Region:                properties["region"],
		Tier:                  properties["tier"],
		LocationID:            properties["locationId"],
		AlternativeLocationID: properties["alternativeLocationId"],
		ReservedIPRange:       properties["reservedIpRange"],
		AuthorizedNetwork:     properties["authorizedNetwork"],
		RedisVersion:          properties["redisVersion"],
		RedisConfigs:          util.ParseMap(properties["redisConfigs"]),
	}

	if i, err := strconv.Atoi(properties["memorySizeGb"]); err == nil {
		spec.MemorySizeGB = i
	}

	return spec
}

// ConnectionSecretName returns a secret name from the reference
func (c *CloudMemorystoreInstance) ConnectionSecretName() string {
	if c.Spec.ConnectionSecretRef.Name == "" {
		c.Spec.ConnectionSecretRef.Name = c.Name
	}

	return c.Spec.ConnectionSecretRef.Name
}

// IsAvailable for usage/binding
func (c *CloudMemorystoreInstance) IsAvailable() bool {
	return c.Status.State == StateReady
}

// IsBound determines if the resource is in a bound binding state
func (c *CloudMemorystoreInstance) IsBound() bool {
	return c.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound sets the binding state of this resource
func (c *CloudMemorystoreInstance) SetBound(state bool) {
	if state {
		c.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		c.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
