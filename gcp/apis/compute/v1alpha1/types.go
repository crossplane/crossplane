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

package v1alpha1

import (
	"github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster states.
const (
	ClusterStateProvisioning = "PROVISIONING"
	ClusterStateRunning      = "RUNNING"
)

// Defaults for GKE resources.
const (
	DefaultReclaimPolicy = v1alpha1.ReclaimRetain
	DefaultNumberOfNodes = int64(1)
)

// GKEClusterParameters specifies the configuration of a GKE cluster.
type GKEClusterParameters struct {
	Addons                    []string          `json:"addons,omitempty"`
	Async                     bool              `json:"async,omitempty"`
	ClusterIPV4CIDR           string            `json:"clusterIPV4CIDR,omitempty"`
	ClusterSecondaryRangeName string            `json:"clusterSecondaryRangeName,omitempty"`
	ClusterVersion            string            `json:"clusterVersion,omitempty"`
	CreateSubnetwork          bool              `json:"createSubnetwork,omitempty"`
	DiskSize                  string            `json:"diskSize,omitempty"`
	EnableAutorepair          bool              `json:"enableAutorepair,omitempty"`
	EnableAutoupgrade         bool              `json:"enableAutoupgrade,omitempty"`
	EnableCloudLogging        bool              `json:"enableCloudLogging,omitempty"`
	EnableCloudMonitoring     bool              `json:"enableCloudMonitoring,omitempty"`
	EnableIPAlias             bool              `json:"enableIPAlias,omitempty"`
	EnableKubernetesAlpha     bool              `json:"enableKubernetesAlpha,omitempty"`
	EnableLegacyAuthorization bool              `json:"enableLegacyAuthorization,omitempty"`
	EnableNetworkPolicy       bool              `json:"enableNetworkPolicy,omitempty"`
	ImageType                 string            `json:"imageType,omitempty"`
	NoIssueClientCertificates bool              `json:"noIssueClientCertificates,omitempty"`
	Labels                    map[string]string `json:"labels,omitempty"`
	LocalSSDCount             int64             `json:"localSSDCount,omitempty"`
	MachineType               string            `json:"machineType,omitempty"`
	MaintenanceWindow         string            `json:"maintenanceWindow,omitempty"`
	MaxNodesPerPool           int64             `json:"maxNodesPerPool,omitempty"`
	MinCPUPlatform            string            `json:"minCPUPlatform,omitempty"`
	Network                   string            `json:"network,omitempty"`
	NodeIPV4CIDR              string            `json:"nodeIPV4CIDR,omitempty"`
	NodeLabels                []string          `json:"nodeLabels,omitempty"`
	NodeLocations             []string          `json:"nodeLocations,omitempty"`
	NodeTaints                []string          `json:"nodeTaints,omitempty"`
	NodeVersion               []string          `json:"nodeVersion,omitempty"`
	NumNodes                  int64             `json:"numNodes,omitempty"`
	Preemtible                bool              `json:"preemtible,omitempty"`
	ServiceIPV4CIDR           string            `json:"serviceIPV4CIDR,omitempty"`
	ServiceSecondaryRangeName string            `json:"serviceSecondaryRangeName,omitempty"`
	Subnetwork                string            `json:"subnetwork,omitempty"`
	Tags                      []string          `json:"tags,omitempty"`
	Zone                      string            `json:"zone,omitempty"`

	EnableAutoscaling bool  `json:"enableAutoscaling,omitempty"`
	MaxNodes          int64 `json:"maxNodes,omitempty"`
	MinNodes          int64 `json:"minNodes,omitempty"`

	Password        string `json:"password,omitempty"`
	EnableBasicAuth bool   `json:"enableBasicAuth,omitempty"`
	Username        string `json:"username,omitempty"`

	ServiceAccount       string   `json:"serviceAccount,omitempty"`
	EnableCloudEndpoints bool     `json:"enableCloudEndpoints,omitempty"`
	Scopes               []string `json:"scopes,omitempty"`
}

// GKEClusterSpec specifies the configuration of a GKE cluster.
type GKEClusterSpec struct {
	v1alpha1.ResourceSpec `json:",inline"`
	GKEClusterParameters  `json:",inline"`
}

// GKEClusterStatus represents the status of a GKE cluster.
type GKEClusterStatus struct {
	v1alpha1.ResourceStatus `json:",inline"`

	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	State       string `json:"state,omitempty"`
}

// +kubebuilder:object:root=true

// GKECluster is the Schema for the instances API
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.bindingPhase"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="CLUSTER-NAME",type="string",JSONPath=".status.clusterName"
// +kubebuilder:printcolumn:name="ENDPOINT",type="string",JSONPath=".status.endpoint"
// +kubebuilder:printcolumn:name="CLUSTER-CLASS",type="string",JSONPath=".spec.classRef.name"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.zone"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".spec.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type GKECluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKEClusterSpec   `json:"spec,omitempty"`
	Status GKEClusterStatus `json:"status,omitempty"`
}

// SetBindingPhase of this GKECluster.
func (c *GKECluster) SetBindingPhase(p v1alpha1.BindingPhase) {
	c.Status.SetBindingPhase(p)
}

// GetBindingPhase of this GKECluster.
func (c *GKECluster) GetBindingPhase() v1alpha1.BindingPhase {
	return c.Status.GetBindingPhase()
}

// SetConditions of this GKECluster.
func (c *GKECluster) SetConditions(cd ...v1alpha1.Condition) {
	c.Status.SetConditions(cd...)
}

// SetClaimReference of this GKECluster.
func (c *GKECluster) SetClaimReference(r *corev1.ObjectReference) {
	c.Spec.ClaimReference = r
}

// GetClaimReference of this GKECluster.
func (c *GKECluster) GetClaimReference() *corev1.ObjectReference {
	return c.Spec.ClaimReference
}

// SetClassReference of this GKECluster.
func (c *GKECluster) SetClassReference(r *corev1.ObjectReference) {
	c.Spec.ClassReference = r
}

// GetClassReference of this GKECluster.
func (c *GKECluster) GetClassReference() *corev1.ObjectReference {
	return c.Spec.ClassReference
}

// SetWriteConnectionSecretToReference of this GKECluster.
func (c *GKECluster) SetWriteConnectionSecretToReference(r corev1.LocalObjectReference) {
	c.Spec.WriteConnectionSecretToReference = r
}

// GetWriteConnectionSecretToReference of this GKECluster.
func (c *GKECluster) GetWriteConnectionSecretToReference() corev1.LocalObjectReference {
	return c.Spec.WriteConnectionSecretToReference
}

// GetReclaimPolicy of this GKECluster.
func (c *GKECluster) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return c.Spec.ReclaimPolicy
}

// SetReclaimPolicy of this GKECluster.
func (c *GKECluster) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	c.Spec.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// GKEClusterList contains a list of GKECluster items
type GKEClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GKECluster `json:"items"`
}

// GKEClusterClassSpecTemplate is the Schema for the resource class
type GKEClusterClassSpecTemplate struct {
	v1alpha1.ResourceClassSpecTemplate `json:",inline"`
	GKEClusterParameters               `json:",inline"`
}

var _ resource.Class = &GKEClusterClass{}

// +kubebuilder:object:root=true

// GKEClusterClass is the Schema for the resource class
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".specTemplate.providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".specTemplate.reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type GKEClusterClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	SpecTemplate GKEClusterClassSpecTemplate `json:"specTemplate,omitempty"`
}

// GetReclaimPolicy of this GKEClusterClass.
func (i *GKEClusterClass) GetReclaimPolicy() v1alpha1.ReclaimPolicy {
	return i.SpecTemplate.ReclaimPolicy
}

// SetReclaimPolicy of this GKEClusterClass.
func (i *GKEClusterClass) SetReclaimPolicy(p v1alpha1.ReclaimPolicy) {
	i.SpecTemplate.ReclaimPolicy = p
}

// +kubebuilder:object:root=true

// GKEClusterClassList contains a list of cloud memorystore resource classes.
type GKEClusterClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GKEClusterClass `json:"items"`
}
