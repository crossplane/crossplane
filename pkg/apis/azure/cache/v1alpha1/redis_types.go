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

	"github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// SKU options.
const (
	SKUNameBasic    = string(redis.Basic)
	SKUNamePremium  = string(redis.Premium)
	SKUNameStandard = string(redis.Standard)

	SKUFamilyC = string(redis.C)
	SKUFamilyP = string(redis.P)
)

// Resource states
const (
	ProvisioningStateCreating               = string(redis.Creating)
	ProvisioningStateDeleting               = string(redis.Deleting)
	ProvisioningStateDisabled               = string(redis.Disabled)
	ProvisioningStateFailed                 = string(redis.Failed)
	ProvisioningStateLinking                = string(redis.Linking)
	ProvisioningStateProvisioning           = string(redis.Provisioning)
	ProvisioningStateRecoveringScaleFailure = string(redis.RecoveringScaleFailure)
	ProvisioningStateScaling                = string(redis.Scaling)
	ProvisioningStateSucceeded              = string(redis.Succeeded)
	ProvisioningStateUnlinking              = string(redis.Unlinking)
	ProvisioningStateUnprovisioning         = string(redis.Unprovisioning)
	ProvisioningStateUpdating               = string(redis.Updating)
)

const (
	// SupportedRedisVersion is the only minor version of Redis currently
	// supported by Azure Cache for Redis. The version cannot be specified at
	// creation time.
	SupportedRedisVersion = "3.2"
)

// RedisSpec defines the desired state of Redis
// Most fields map directly to an Azure Redis resource.
// https://docs.microsoft.com/en-us/rest/api/redis/redis/get#redisresource
type RedisSpec struct {
	// ResourceGroupName in which to create this resource.
	ResourceGroupName string `json:"resourceGroupName"`

	// Location in which to create this resource.
	Location string `json:"location"`

	// SKU of the Redis cache to deploy.
	SKU SKUSpec `json:"sku"`

	// EnableNonSSLPort specifies whether the non-ssl Redis server port (6379)
	// is enabled.
	EnableNonSSLPort bool `json:"enableNonSslPort,omitempty"`

	// ShardCount specifies the number of shards to be created on a Premium
	// Cluster Cache.
	ShardCount int `json:"shardCount,omitempty"`

	// StaticIP address. Required when deploying a Redis cache inside an
	// existing Azure Virtual Network.
	StaticIP string `json:"staticIP,omitempty"`

	// SubnetID specifies the full resource ID of a subnet in a virtual network
	// to deploy the Redis cache in. Example format:
	// /subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/Microsoft.{Network|ClassicNetwork}/VirtualNetworks/vnet1/subnets/subnet1
	SubnetID string `json:"subnetId,omitempty"`

	// RedisConfiguration specifies Redis Settings.
	RedisConfiguration map[string]string `json:"redisConfiguration,omitempty"`

	// Kubernetes object references
	ClaimRef            *v1.ObjectReference     `json:"claimRef,omitempty"`
	ClassRef            *v1.ObjectReference     `json:"classRef,omitempty"`
	ProviderRef         v1.LocalObjectReference `json:"providerRef"`
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// TODO(negz): Rename SKU to PricingTier? Both SQL databases and Redis caches
// call this an 'SKU' in their API, but we call it a PricingTier in our Azure
// SQL database CRD.

// SKUSpec represents the performance and cost oriented properties of the server
type SKUSpec struct {
	// Name specifies what type of Redis cache to deploy. Valid values: (Basic,
	// Standard, Premium). Possible values include: 'Basic', 'Standard',
	// 'Premium'
	// +kubebuilder:validation:Enum=Basic,Standard,Premium
	Name string `json:"name"`

	// Family specifies which family to use. Valid values: (C, P). Possible
	// values include: 'C', 'P'
	// +kubebuilder:validation:Enum=C,P
	Family string `json:"family"`

	// Capacity specifies the size of Redis cache to deploy. Valid values: for C
	// family (0, 1, 2, 3, 4, 5, 6), for P family (1, 2, 3, 4).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=6
	Capacity int `json:"capacity"`
}

// RedisStatus defines the observed state of Redis
type RedisStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`

	// ProviderID is the external ID to identify this resource in the cloud
	// provider
	ProviderID string `json:"providerID,omitempty"`

	// Endpoint of the Redis resource used in connection strings.
	Endpoint string `json:"endpoint,omitempty"`

	// Port at which the Redis endpoint is listening.
	Port int `json:"port,omitempty"`

	// SSLPort at which the Redis endpoint is listening.
	SSLPort int `json:"sslPort,omitempty"`

	// RedisVersion the Redis endpoint is running.
	RedisVersion string `json:"redisVersion,omitempty"`

	// ResourceName of the Redis cache resource.
	ResourceName string `json:"resourceName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Redis is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=cache.azure
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".status.redisVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisList contains a list of Redis
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

// NewRedisSpec creates a new RedisSpec
// from the given properties map.
func NewRedisSpec(properties map[string]string) *RedisSpec {
	spec := &RedisSpec{
		ReclaimPolicy: corev1alpha1.ReclaimRetain,

		// Note that these keys should match the JSON tags of their respective
		// RedisSpec fields.
		ResourceGroupName:  properties["resourceGroupName"],
		Location:           properties["location"],
		StaticIP:           properties["staticIP"],
		SubnetID:           properties["subnetId"],
		RedisConfiguration: util.ParseMap(properties["redisConfiguration"]),

		// TODO(negz): Remove the sku prefix here? This feels clearer, but the
		// Azure SQL server equivalent uses bare "tier", "vcores", etc.
		SKU: SKUSpec{
			Name:   properties["skuName"],
			Family: properties["skuFamily"],
		},
	}

	if b, err := strconv.ParseBool(properties["enableNonSslPort"]); err == nil {
		spec.EnableNonSSLPort = b
	}
	if i, err := strconv.Atoi(properties["shardCount"]); err == nil {
		spec.ShardCount = i
	}
	if i, err := strconv.Atoi(properties["skuCapacity"]); err == nil {
		spec.SKU.Capacity = i
	}

	return spec
}

// ConnectionSecretName returns a secret name from the reference
func (c *Redis) ConnectionSecretName() string {
	if c.Spec.ConnectionSecretRef.Name == "" {
		c.Spec.ConnectionSecretRef.Name = c.Name
	}

	return c.Spec.ConnectionSecretRef.Name
}

// ObjectReference to this Redis instance
func (c *Redis) ObjectReference() *v1.ObjectReference {
	return util.ObjectReference(c.ObjectMeta, util.IfEmptyString(c.APIVersion, APIVersion), util.IfEmptyString(c.Kind, RedisKind))
}

// OwnerReference to use this instance as an owner
func (c *Redis) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(c.ObjectReference())
}

// IsAvailable for usage/binding
func (c *Redis) IsAvailable() bool {
	return c.Status.State == ProvisioningStateSucceeded
}

// IsBound determines if the resource is in a bound binding state
func (c *Redis) IsBound() bool {
	return c.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound sets the binding state of this resource
func (c *Redis) SetBound(state bool) {
	if state {
		c.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		c.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
