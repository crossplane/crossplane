// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigurationSpec specifies the configuration of a Configuration.
type ConfigurationSpec struct {
	MetaSpec `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Configuration is the description of a Crossplane Configuration package.
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ConfigurationSpec `json:"spec"`
}
