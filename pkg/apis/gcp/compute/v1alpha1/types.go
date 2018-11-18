/*
Copyright 2018 The Conductor Authors.

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

	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterStateProvisioning = "PROVISIONING"
	ClusterStateRunning      = "RUNNING"

	DefaultReclaimPolicy = corev1alpha1.ReclaimRetain
	DefaultNumberOfNodes = int64(1)
)

// GKEClusterSpec
type GKEClusterSpec struct {
	Addons                    []string          `json:"addons,omitempty"`                    //--adons
	Async                     bool              `json:"async,omitempty"`                     //--async
	ClusterIPV4CIDR           string            `json:"clusterIPV4CIDR,omitempty"`           //--cluster-ipv4-cidr
	ClusterSecondaryRangeName string            `json:"clusterSecondaryRangeName,omitempty"` //--cluster-secondary-range-name
	ClusterVersion            string            `json:"clusterVersion,omitempty"`            //--cluster-version
	ClusterSubnetwork         map[string]string `json:"createSubnetwork,omitempty"`          //--create-subnetwork
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
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// GKEClusterStatus
type GKEClusterStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	State       string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKECluster is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=compute.gcp
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

// NewGKEClusterSpec from properties map
func NewGKEClusterSpec(properties map[string]string) *GKEClusterSpec {
	spec := &GKEClusterSpec{
		ReclaimPolicy: DefaultReclaimPolicy,
		Zone:          properties["zone"],
		MachineType:   properties["machineType"],
		NumNodes:      DefaultNumberOfNodes,
	}

	// assign nodes from properties
	n, err := strconv.Atoi(properties["numNodes"])
	if err == nil {
		spec.NumNodes = int64(n)
	}

	return spec
}

// ObjectReference to this RDSInstance
func (g *GKECluster) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(g.ObjectMeta, util.IfEmptyString(g.APIVersion, APIVersion), util.IfEmptyString(g.Kind, GKEClusterKind))
}

// OwnerReference to use this instance as an owner
func (g *GKECluster) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(g.ObjectReference())
}

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

func (g *GKECluster) Endpoint() string {
	return g.Status.Endpoint
}

// State returns rds instance state value saved in the status (could be empty)
func (g *GKECluster) State() string {
	return g.Status.State
}

// IsAvailable for usage/binding
func (g *GKECluster) IsAvailable() bool {
	return g.State() == ClusterStateRunning
}

// IsBound
func (g *GKECluster) IsBound() bool {
	return g.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound
func (g *GKECluster) SetBound(state bool) {
	if state {
		g.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		g.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
