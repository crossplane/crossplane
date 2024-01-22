// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A EnvironmentConfig contains a set of arbitrary, unstructured values.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=envcfg
type EnvironmentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The data of this EnvironmentConfig.
	// This may contain any kind of structure that can be serialized into JSON.
	// +optional
	Data map[string]extv1.JSON `json:"data,omitempty"`
}

// +kubebuilder:object:root=true

// EnvironmentConfigList contains a list of EnvironmentConfigs.
type EnvironmentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvironmentConfig `json:"items"`
}
