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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterProvisioningStateSucceeded = "Succeeded"

	DefaultReclaimPolicy = corev1alpha1.ReclaimRetain
	DefaultNodeCount     = 1
)

// AKSClusterSpec
type AKSClusterSpec struct {
	ResourceGroupName string `json:"resourceGroupName"` //--resource-group
	Location          string `json:"location"`          //--location
	Version           string `json:"version"`           //--kubernetes-version
	NodeCount         int    `json:"nodeCount"`         //--node-count
	NodeVMSize        string `json:"nodeVMSize"`        //--node-vm-size
	DNSNamePrefix     string `json:"dnsNamePrefix"`     //--dns-name-prefix
	DisableRBAC       bool   `json:"disableRBAC"`       //--disable-rbac

	// Kubernetes object references
	ClaimRef            *corev1.ObjectReference      `json:"claimRef,omitempty"`
	ClassRef            *corev1.ObjectReference      `json:"classRef,omitempty"`
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`
	ProviderRef         corev1.LocalObjectReference  `json:"providerRef,omitempty"`

	// ReclaimPolicy identifies how to handle the cloud resource after the deletion of this type
	ReclaimPolicy corev1alpha1.ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// AKSClusterStatus
type AKSClusterStatus struct {
	corev1alpha1.ConditionedStatus
	corev1alpha1.BindingStatusPhase
	// ClusterName is the name of the cluster as registered with the cloud provider
	ClusterName string `json:"clusterName,omitempty"`
	// State is the current state of the cluster
	State string `json:"state,omitempty"`
	// the external ID to identify this resource in the cloud provider
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
// +k8s:openapi-gen=true
// +groupName=compute.azure
type AKSCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AKSClusterSpec   `json:"spec,omitempty"`
	Status AKSClusterStatus `json:"status,omitempty"`
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
		ReclaimPolicy: corev1alpha1.ReclaimRetain,
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
			spec.NodeCount = nodeCount
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

// ObjectReference to this instance
func (a *AKSCluster) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(a.ObjectMeta, util.IfEmptyString(a.APIVersion, APIVersion), util.IfEmptyString(a.Kind, AKSClusterKind))
}

// OwnerReference to use this instance as an owner
func (a *AKSCluster) OwnerReference() metav1.OwnerReference {
	return *util.ObjectToOwnerReference(a.ObjectReference())
}

func (a *AKSCluster) ConnectionSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       a.Namespace,
			Name:            a.ConnectionSecretName(),
			OwnerReferences: []metav1.OwnerReference{a.OwnerReference()},
		},
	}
}

// ConnectionSecretName returns a secret name from the reference
func (a *AKSCluster) ConnectionSecretName() string {
	if a.Spec.ConnectionSecretRef == nil {
		a.Spec.ConnectionSecretRef = &corev1.LocalObjectReference{
			Name: a.Name,
		}
	} else if a.Spec.ConnectionSecretRef.Name == "" {
		a.Spec.ConnectionSecretRef.Name = a.Name
	}

	return a.Spec.ConnectionSecretRef.Name
}

func (a *AKSCluster) Endpoint() string {
	return a.Status.Endpoint
}

// State returns instance state value saved in the status (could be empty)
func (a *AKSCluster) State() string {
	return a.Status.State
}

// IsAvailable for usage/binding
func (a *AKSCluster) IsAvailable() bool {
	return a.State() == ClusterProvisioningStateSucceeded
}

// IsBound
func (a *AKSCluster) IsBound() bool {
	return a.Status.Phase == corev1alpha1.BindingStateBound
}

// SetBound
func (a *AKSCluster) SetBound(state bool) {
	if state {
		a.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		a.Status.Phase = corev1alpha1.BindingStateUnbound
	}
}
