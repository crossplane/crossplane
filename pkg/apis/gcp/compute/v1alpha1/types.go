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

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
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

// GKEClusterSpec specifies the configuration of a GKE cluster.
type GKEClusterSpec struct {
	Addons                    []string          `json:"addons,omitempty"`                    //--adons
	Async                     bool              `json:"async,omitempty"`                     //--async
	ClusterIPV4CIDR           string            `json:"clusterIPV4CIDR,omitempty"`           //--cluster-ipv4-cidr
	ClusterSecondaryRangeName string            `json:"clusterSecondaryRangeName,omitempty"` //--cluster-secondary-range-name
	ClusterVersion            string            `json:"clusterVersion,omitempty"`            //--cluster-version
	CreateSubnetwork          bool              `json:"createSubnetwork,omitempty"`          //--create-subnetwork
	DiskSize                  string            `json:"diskSize,omitempty"`                  //--disk-size
	EnableAutorepair          bool              `json:"enableAutorepair,omitempty"`          //--enable-autorepair
	EnableAutoupgrade         bool              `json:"enableAutoupgrade,omitempty"`         //--enable-autoupgrade
	EnableCloudLogging        bool              `json:"enableCloudLogging,omitempty"`        //--no-enable-cloud-logging]
	EnableCloudMonitoring     bool              `json:"enableCloudMonitoring,omitempty"`     //--no-enable-cloud-monitoring
	EnableIPAlias             bool              `json:"enableIPAlias,omitempty"`             //--enable-ip-alias
	EnableKubernetesAlpha     bool              `json:"enableKubernetesAlpha,omitempty"`     //--enable-kubernetes-alpha
	EnableLegacyAuthorization bool              `json:"enableLegacyAuthorization,omitempty"` //--enable-legacy-authorization
	EnableNetworkPolicy       bool              `json:"enableNetworkPolicy,omitempty"`       //--enable-network-policy
	ImageType                 string            `json:"imageType,omitempty"`                 //--image-type
	NoIssueClientCertificates bool              `json:"noIssueClientCertificates,omitempty"` //--no-issue-client-certificate
	Labels                    map[string]string `json:"labels,omitempty"`                    //--labels
	LocalSSDCount             int64             `json:"localSSDCount,omitempty"`             //--local-ssd-count
	MachineType               string            `json:"machineType,omitempty"`               //--machine-types
	MaintenanceWindow         string            `json:"maintenanceWindow,omitempty"`         //--maintenance-window, example: '12:43'
	MaxNodesPerPool           int64             `json:"maxNodesPerPool,omitempty"`           //--max-nodes-per-pool
	MinCPUPlatform            string            `json:"minCPUPlatform,omitempty"`            //--min-cpu-platform
	Network                   string            `json:"network,omitempty"`                   //--network
	NodeIPV4CIDR              string            `json:"nodeIPV4CIDR,omitempty"`
	NodeLabels                []string          `json:"nodeLabels,omitempty"`                //--node-labels [NODE_LABEL,…]
	NodeLocations             []string          `json:"nodeLocations,omitempty"`             //--node-locations=ZONE,[ZONE,…]
	NodeTaints                []string          `json:"nodeTaints,omitempty"`                //--node-taints=[NODE_TAINT,…]
	NodeVersion               []string          `json:"nodeVersion,omitempty"`               //--node-version=NODE_VERSION
	NumNodes                  int64             `json:"numNodes,omitempty"`                  //--num-nodes=NUM_NODES; default=3
	Preemtible                bool              `json:"preemtible,omitempty"`                //--preemptible
	ServiceIPV4CIDR           string            `json:"serviceIPV4CIDR,omitempty"`           //--services-ipv4-cidr=CIDR
	ServiceSecondaryRangeName string            `json:"serviceSecondaryRangeName,omitempty"` //--services-secondary-range-name=NAME
	Subnetwork                string            `json:"subnetwork,omitempty"`                //--subnetwork=SUBNETWORK
	Tags                      []string          `json:"tags,omitempty"`                      //--tags=TAG,[TAG,…]
	Zone                      string            `json:"zone,omitempty"`                      //--zone
	// Cluster Autoscaling
	EnableAutoscaling bool  `json:"enableAutoscaling,omitempty"` //--enable-autoscaling
	MaxNodes          int64 `json:"maxNodes,omitempty"`          //--max-nodes
	MinNodes          int64 `json:"minNodes,omitempty"`          //--min-nodes
	// Basic Auth
	Password        string `json:"password,omitempty"`        //--password
	EnableBasicAuth bool   `json:"enableBasicAuth,omitempty"` //--enable-basic-auth
	Username        string `json:"username,omitempty"`        //--username (-u) default:"admin"
	// Node Identity
	ServiceAccount       string   `json:"serviceAccount,omitempty,omitempty"` //--service-account
	EnableCloudEndpoints bool     `json:"enableCloudEndpoints,omitempty"`     //--enable-cloud-endpoints
	Scopes               []string `json:"scopes,omitempty"`                   //--scopes=[SCOPE,…]; default="gke-default"

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference      `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference      `json:"classRef,omitempty"`
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference  `json:"providerRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy v1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// GKEClusterStatus represents the status of a GKE cluster.
type GKEClusterStatus struct {
	v1alpha1.ConditionedStatus
	v1alpha1.BindingStatusPhase
	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	State       string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKECluster is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=compute.gcp
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
		ReclaimPolicy:    DefaultReclaimPolicy,
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

// ObjectReference to this RDSInstance
func (g *GKECluster) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(g.ObjectMeta, util.IfEmptyString(g.APIVersion, APIVersion), util.IfEmptyString(g.Kind, GKEClusterKind))
}

// OwnerReference to use this instance as an owner
func (g *GKECluster) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(g.ObjectReference())
}

// ConnectionSecret returns the connection secret for this GKE cluster.
func (g *GKECluster) ConnectionSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       g.Namespace,
			Name:            g.ConnectionSecretName(),
			OwnerReferences: []metav1.OwnerReference{g.OwnerReference()},
		},
	}
}

// ConnectionSecretName returns a secret name from the reference
func (g *GKECluster) ConnectionSecretName() string {
	if g.Spec.ConnectionSecretRef == nil {
		g.Spec.ConnectionSecretRef = &corev1.LocalObjectReference{
			Name: g.Name,
		}
	} else if g.Spec.ConnectionSecretRef.Name == "" {
		g.Spec.ConnectionSecretRef.Name = g.Name
	}

	return g.Spec.ConnectionSecretRef.Name
}

// State returns rds instance state value saved in the status (could be empty)
func (g *GKECluster) State() string {
	return g.Status.State
}

// IsAvailable for usage/binding
func (g *GKECluster) IsAvailable() bool {
	return g.State() == ClusterStateRunning
}

// IsBound returns true if this GKE cluster is bound to a resource claim.
func (g *GKECluster) IsBound() bool {
	return g.Status.IsBound()
}

// SetBound specifies whether this GKE cluster is bound to a resource claim.
func (g *GKECluster) SetBound(bound bool) {
	g.Status.SetBound(bound)
}
