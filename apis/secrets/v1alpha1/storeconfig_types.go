/*
Copyright 2022 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// A StoreConfigSpec defines the desired state of a StoreConfig.
type StoreConfigSpec struct {
	xpv1.SecretStoreConfig `json:",inline"`
}

// +kubebuilder:object:root=true

// A StoreConfig configures how Crossplane controllers should store connection
// details in an external secret store.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="DEFAULT-SCOPE",type="string",JSONPath=".spec.defaultScope"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,store}
type StoreConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StoreConfigSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// StoreConfigList contains a list of StoreConfig.
type StoreConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoreConfig `json:"items"`
}

// GetStoreConfig returns SecretStoreConfig.
func (in *StoreConfig) GetStoreConfig() xpv1.SecretStoreConfig {
	return in.Spec.SecretStoreConfig
}
