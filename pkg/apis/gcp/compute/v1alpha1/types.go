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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// Cluster states.
const (
	ClusterStateProvisioning = "PROVISIONING"
	ClusterStateRunning      = "RUNNING"
)

// Defaults for GKE resources.
const (
	DefaultReclaimPolicy = corev1alpha1.ReclaimRetain
	DefaultNumberOfNodes = int64(1)
)

// GKEClusterSpec specifies the configuration of a GKE cluster.
type GKEClusterSpec struct {
	corev1alpha1.ResourceSpec

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

	ServiceAccount       string   `json:"serviceAccount,omitempty,omitempty"`
	EnableCloudEndpoints bool     `json:"enableCloudEndpoints,omitempty"`
	Scopes               []string `json:"scopes,omitempty"`
}

// GKEClusterStatus represents the status of a GKE cluster.
type GKEClusterStatus struct {
	corev1alpha1.ResourceStatus

	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	State       string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
func (c *GKECluster) SetBindingPhase(p corev1alpha1.BindingPhase) {
	c.Status.SetBindingPhase(p)
}

// GetBindingPhase of this GKECluster.
func (c *GKECluster) GetBindingPhase() corev1alpha1.BindingPhase {
	return c.Status.GetBindingPhase()
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

// SetWriteConnectionSecretTo of this GKECluster.
func (c *GKECluster) SetWriteConnectionSecretTo(r corev1.LocalObjectReference) {
	c.Spec.WriteConnectionSecretTo = r
}

// GetWriteConnectionSecretTo of this GKECluster.
func (c *GKECluster) GetWriteConnectionSecretTo() corev1.LocalObjectReference {
	return c.Spec.WriteConnectionSecretTo
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKEClusterList contains a list of GKECluster items
type GKEClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GKECluster `json:"items"`
}

// ParseClusterSpec from properties map
func ParseClusterSpec(properties map[string]string) *GKEClusterSpec {
	return &GKEClusterSpec{
		ResourceSpec: corev1alpha1.ResourceSpec{
			ReclaimPolicy: DefaultReclaimPolicy,
		},
		ClusterVersion:   properties["clusterVersion"],
		Labels:           util.ParseMap(properties["labels"]),
		MachineType:      properties["machineType"],
		NumNodes:         parseNodesNumber(properties["numNodes"]),
		Scopes:           util.Split(properties["scopes"], ","),
		Zone:             properties["zone"],
		EnableIPAlias:    util.ParseBool(properties["enableIPAlias"]),
		CreateSubnetwork: util.ParseBool(properties["createSubnetwork"]),
		ClusterIPV4CIDR:  properties["clusterIPV4CIDR"],
		ServiceIPV4CIDR:  properties["serviceIPV4CIDR"],
		NodeIPV4CIDR:     properties["nodeIPV4CIDR"],
	}
}

// parseNodesNumber from the input string value
// If value is empty or invalid integer >= 0: return DefaultNumberOfNodes
func parseNodesNumber(s string) int64 {
	if s == "" {
		return DefaultNumberOfNodes
	}

	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return DefaultNumberOfNodes
	}
	return int64(n)
}
