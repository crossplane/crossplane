// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderSpec specifies the configuration of a Provider.
type ProviderSpec struct {
	// Configuration for the packaged Provider's controller.
	Controller ControllerSpec `json:"controller"`

	MetaSpec `json:",inline"`
}

// ControllerSpec specifies the configuration for the packaged Provider
// controller.
type ControllerSpec struct {
	// Image is the packaged Provider controller image.
	Image *string `json:"image,omitempty"`

	// PermissionRequests for RBAC rules required for this provider's controller
	// to function. The RBAC manager is responsible for assessing the requested
	// permissions.
	// +optional
	PermissionRequests []rbacv1.PolicyRule `json:"permissionRequests,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Provider is the description of a Crossplane Provider package.
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProviderSpec `json:"spec"`
}
