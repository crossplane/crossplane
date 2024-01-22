// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FunctionSpec specifies the configuration of a Function.
type FunctionSpec struct {
	MetaSpec `json:",inline"`

	// Image is the packaged Function image.
	Image *string `json:"image,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Function is the description of a Crossplane Function package.
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FunctionSpec `json:"spec"`
}
