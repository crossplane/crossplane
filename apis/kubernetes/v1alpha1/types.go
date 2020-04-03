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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// A ProviderSpec defines the desired state of a Provider.
type ProviderSpec struct {
	// A Secret containing connection credentials for a Kubernetes cluster
	// client that will be used to authenticate to this Kubernetes Provider.
	// This will typically be the connection secret of a KubernetesCluster claim,
	// or the secret created by a Kubernetes service account, but could also be
	// manually configured to connect to a preexisting cluster.
	Secret v1alpha1.SecretReference `json:"credentialsSecretRef"`
}

// +kubebuilder:object:root=true

// A Provider configures a Kubernetes 'provider', i.e. a connection to a particular
// Kubernetes cluster using the referenced Secret.
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentialsSecretRef.name",priority=1
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Provider
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}
