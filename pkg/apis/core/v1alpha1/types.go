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
	"github.com/crossplaneio/crossplane/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceName is the name identifying various resources in a ResourceList.
type ResourceName string

// Resource names must be not more than 63 characters, consisting of upper- or lower-case alphanumeric characters,
// with the -, _, and . characters allowed anywhere, except the first or last character.
// The default convention, matching that for annotations, is to use lower-case names, with dashes, rather than
// camel case, separating compound words.
// Fully-qualified resource typenames are constructed from a DNS-style subdomain, followed by a slash `/` and a name.
const (
	// CPU, in cores. (500m = .5 cores)
	ResourceCPU ResourceName = "cpu"
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	ResourceMemory ResourceName = "memory"
	// Volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024)
	ResourceStorage ResourceName = "storage"
)

// ResourceList is a set of (resource name, quantity) pairs.
type ResourceList map[ResourceName]resource.Quantity

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceClass is the Schema for the instances API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="PROVISIONER",type="string",JSONPath=".provisioner"
// +kubebuilder:printcolumn:name="PROVIDER-REF",type="string",JSONPath=".providerRef.name"
// +kubebuilder:printcolumn:name="RECLAIM-POLICY",type="string",JSONPath=".reclaimPolicy"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ResourceClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Parameters holds parameters for the provisioner.
	// These values are opaque to the  system and are passed directly
	// to the provisioner.  The only validation done on keys is that they are
	// not empty.  The maximum number of parameters is
	// 512, with a cumulative max size of 256K
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// Provisioner is the driver expected to handle this ResourceClass.
	// This is an optionally-prefixed name, like a label key.
	// For example: "RDSInstance.database.aws.crossplane.io/v1alpha1" or "CloudSQLInstance.database.gcp.crossplane.io/v1alpha1".
	// This value may not be empty.
	Provisioner string `json:"provisioner"`

	// ProvierRef is the reference to cloud provider that will be used
	// to provision the concrete cloud resource
	ProviderRef corev1.LocalObjectReference `json:"providerRef"`

	// reclaimPolicy is the reclaim policy that dynamically provisioned
	// ResourceInstances of this resource class are created with
	// +optional
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceClassList contains a list of RDSInstance
type ResourceClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceClass `json:"items"`
}

// ObjectReference to this mysql instance
func (r *ResourceClass) ObjectReference() *corev1.ObjectReference {
	return util.ObjectReference(r.ObjectMeta, util.IfEmptyString(r.APIVersion, APIVersion), util.IfEmptyString(r.Kind, ResourceClassKind))
}
