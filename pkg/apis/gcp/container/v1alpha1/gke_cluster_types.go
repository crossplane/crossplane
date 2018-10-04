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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type clusterAddon string

type GKEClusterSpec struct {
	Addons                    []clusterAddon    `json:"addons,omitempty"`                    //--adons
	Async                     bool              `json:"async,omitempty"`                     //--async
	ClusterIPV4CIDR           string            `json:"clusterIPV4CIDR,omitempty"`           //--cluster-ipv4-cidr
	ClusterSecondaryRangeName string            `json:"clusterSecondaryRangeName,omitempty"` //--cluster-secondary-range-name
	ClusterVersion            string            `json:"clusterVersion"`                      //--cluster-version
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
	MachineType               string            `json:"machineType"`                         //--machine-types
	MaintenanceWindow         string            `json:"maintenanceWindow,omitempty"`         //--maintenance-window, example: '12:43'
	MaxNodesPerPool           int64             `json:"maxNodesPerPool,omitempty"`           //--max-nodes-per-pool
	MinCPUPlatform            string            `json:"minCPUPlatform,omitempty"`            //--min-cpu-platform
	Network                   string            `json:"network,omitempty"`                   //--network
	NodeLabels                []string          `json:"nodeLabels,omitempty"`                //--node-labels [NODE_LABEL,…]
	NodeLocations             []string          `json:"nodeLocations,omitempty"`             //--node-locations=ZONE,[ZONE,…]
	NodeTaints                []string          `json:"nodeTaints,omitempty"`                //--node-taints=[NODE_TAINT,…]
	NodeVersion               []string          `json:"nodeVersion,omitempty"`               //--node-version=NODE_VERSION
	NumNodes                  int64             `json:"numNodes"`                            //--num-nodes=NUM_NODES; default=3
	Preemtible                bool              `json:"preemtible,omitempty"`                //--preemptible
	ServiceIPV4CIDR           string            `json:"serviceIPV4CIDR,omitempty"`           //--services-ipv4-cidr=CIDR
	ServiceSecondaryRangeName string            `json:"serviceSecondaryRangeName,omitempty"` //--services-secondary-range-name=NAME
	Subnetwork                string            `json:"subnetwork,omitempty"`                //--subnetwork=SUBNETWORK
	Tags                      []string          `json:"tags,omitempty"`                      //--tags=TAG,[TAG,…]
	Zone                      string            `json:"zone"`                                //--zone
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

	// ProviderRef - reference to GCP provider object
	ProviderRef v1.LocalObjectReference `json:"providerRef"`

	// ConnectionSecretRef - reference to GKE Cluster connection secret which will be created and contain connection related data
	ConnectionSecretRef v1.LocalObjectReference `json:"connectionSecretRef"`
}

type GKEClusterStatus struct {
	State   string `json:"state,omitempty"`
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKECluster is the Schema for the instances API
// +k8s:openapi-gen=true
// +groupName=container.gcp
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

func init() {
	SchemeBuilder.Register(&GKECluster{}, &GKEClusterList{})
}
