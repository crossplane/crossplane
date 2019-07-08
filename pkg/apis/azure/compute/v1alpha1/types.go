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

	"github.com/Azure/go-autorest/autorest/to"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

const (
	// ClusterProvisioningStateSucceeded is the state for a cluster that has succeeded provisioning
	ClusterProvisioningStateSucceeded = "Succeeded"
	// DefaultReclaimPolicy is the default reclaim policy to use
	DefaultReclaimPolicy = corev1alpha1.ReclaimRetain
	// DefaultNodeCount is the default node count for a cluster
	DefaultNodeCount = 1
)

// AKSClusterSpec is the spec for AKS cluster resources
type AKSClusterSpec struct {
	corev1alpha1.ResourceSpec `json:",inline"`

	// ResourceGroupName is the name of the resource group that the cluster will be created in
	ResourceGroupName string `json:"resourceGroupName"` //--resource-group

	// Location is the Azure location that the cluster will be created in
	Location string `json:"location"` //--location

	// Version is the Kubernetes version that will be deployed to the cluster
	Version string `json:"version"` //--kubernetes-version

	// NodeCount is the number of nodes that the cluster will initially be created with.  This can
	// be scaled over time and defaults to 1.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	NodeCount *int `json:"nodeCount,omitempty"` //--node-count

	// NodeVMSize is the name of the worker node VM size, e.g., Standard_B2s, Standard_F2s_v2, etc.
	// This value cannot be changed after cluster creation.
	NodeVMSize string `json:"nodeVMSize"` //--node-vm-size

	// DNSNamePrefix is the DNS name prefix to use with the hosted Kubernetes API server FQDN. You
	// will use this to connect to the Kubernetes API when managing containers after creating the cluster.
	DNSNamePrefix string `json:"dnsNamePrefix"` //--dns-name-prefix

	// DisableRBAC determines whether RBAC will be disabled or enabled in the cluster.
	DisableRBAC bool `json:"disableRBAC,omitempty"` //--disable-rbac

	// WriteServicePrincipalSecretTo the specified Secret. The service principal
	// is automatically generated and used by the AKS cluster to interact with
	// other Azure resources.
	WriteServicePrincipalSecretTo corev1.LocalObjectReference `json:"writeServicePrincipalTo"`
}

// AKSClusterStatus is the status for AKS cluster resources
type AKSClusterStatus struct {
	corev1alpha1.ResourceStatus `json:",inline"`

	// ClusterName is the name of the cluster as registered with the cloud provider
	ClusterName string `json:"clusterName,omitempty"`
	// State is the current state of the cluster
	State string `json:"state,omitempty"`
	// ProviderID is the external ID to identify this resource in the cloud provider
	ProviderID string `json:"providerID,omitempty"`
	// Endpoint is the endpoint where the cluster can be reached
	Endpoint string `json:"endpoint"`
	// ApplicationObjectID is the object ID of the AD application the cluster uses for Azure APIs
	ApplicationObjectID string `json:"appObjectID,omitempty"`
	// ServicePrincipalID is the ID of the service principal the AD application uses
	ServicePrincipalID string `json:"servicePrincipalID,omitempty"`

	// RunningOperation stores any current long running operation for this instance across
	// reconciliation attempts.  This will be a serialized Azure AKS cluster API object that will
	// be used to check the status and completion of the operation during each reconciliation.
	// Once the operation has completed, this field will be cleared out.
	RunningOperation string `json:"runningOperation,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AKSCluster is the Schema for the instances API
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLUSTER-NAME",type="string",JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type AKSCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AKSClusterSpec   `json:"spec,omitempty"`
	Status AKSClusterStatus `json:"status,omitempty"`
}

// SetBindingPhase of this AKSCluster.
func (c *AKSCluster) SetBindingPhase(p corev1alpha1.BindingPhase) {
	c.Status.SetBindingPhase(p)
}

// GetBindingPhase of this AKSCluster.
func (c *AKSCluster) GetBindingPhase() corev1alpha1.BindingPhase {
	return c.Status.GetBindingPhase()
}

// SetClaimReference of this AKSCluster.
func (c *AKSCluster) SetClaimReference(r *corev1.ObjectReference) {
	c.Spec.ClaimReference = r
}

// GetClaimReference of this AKSCluster.
func (c *AKSCluster) GetClaimReference() *corev1.ObjectReference {
	return c.Spec.ClaimReference
}

// SetClassReference of this AKSCluster.
func (c *AKSCluster) SetClassReference(r *corev1.ObjectReference) {
	c.Spec.ClassReference = r
}

// GetClassReference of this AKSCluster.
func (c *AKSCluster) GetClassReference() *corev1.ObjectReference {
	return c.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this AKSCluster.
func (c *AKSCluster) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	c.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this AKSCluster.
func (c *AKSCluster) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return c.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this AKSCluster.
func (c *AKSCluster) GetReclaimPolicy() corev1alpha1.ReclaimPolicy {
	return c.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this AKSCluster.
func (c *AKSCluster) SetReclaimPolicy(p corev1alpha1.ReclaimPolicy) {
	c.Spec.ReclaimPolicy = p
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AKSClusterList contains a list of AKSCluster items
type AKSClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AKSCluster `json:"items"`
}

// NewAKSClusterSpec creates a new AKSClusterSpec based on the given properties map
func NewAKSClusterSpec(properties map[string]string) *AKSClusterSpec {
	spec := &AKSClusterSpec{
		ResourceSpec: corev1alpha1.ResourceSpec{
			ReclaimPolicy: DefaultReclaimPolicy,
		},
		NodeCount: to.IntPtr(DefaultNodeCount),
	}

	val, ok := properties["resourceGroupName"]
	if ok {
		spec.ResourceGroupName = val
	}

	val, ok = properties["location"]
	if ok {
		spec.Location = val
	}

	val, ok = properties["version"]
	if ok {
		spec.Version = val
	}

	val, ok = properties["nodeCount"]
	if ok {
		if nodeCount, err := strconv.Atoi(val); err == nil {
			spec.NodeCount = to.IntPtr(nodeCount)
		}
	}

	val, ok = properties["nodeVMSize"]
	if ok {
		spec.NodeVMSize = val
	}

	val, ok = properties["dnsNamePrefix"]
	if ok {
		spec.DNSNamePrefix = val
	}

	val, ok = properties["disableRBAC"]
	if ok {
		if disableRBAC, err := strconv.ParseBool(val); err == nil {
			spec.DisableRBAC = disableRBAC
		}
	}

	return spec
}
