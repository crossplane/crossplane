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
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// TODO: how do we pretty print conditioned status items? There may be multiple of them, and they
// can have varying status (e.g., True, False, Unknown)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionRequest is the CRD type for a request to add an extension to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=="Ready")].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ExtensionRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtensionRequestSpec   `json:"spec,omitempty"`
	Status ExtensionRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionRequestList contains a list of ExtensionRequest
type ExtensionRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExtensionRequest `json:"items"`
}

// ExtensionRequestSpec specifies details about a request to add an extension to Crossplane.
type ExtensionRequestSpec struct {
	// Source is the domain name for the extension registry hosting the extension being requested,
	// e.g., registry.crossplane.io
	Source string `json:"source,omitempty"`

	// Package is the name of the extension package that is being requested, e.g., myapp.
	// Either Package or CustomResourceDefinition can be specified.
	Package string `json:"package,omitempty"`

	// CustomResourceDefinition is the full name of a CRD that is owned by the extension being
	// requested. This can be a convenient way of installing an extension when the desired
	// CRD is known, but the package name that contains it is not known.
	// Either Package or CustomResourceDefinition can be specified.
	CustomResourceDefinition string `json:"crd,omitempty"`
}

// ExtensionRequestStatus defines the observed state of ExtensionRequest
type ExtensionRequestStatus struct {
	corev1alpha1.ConditionedStatus

	InstallJob      *corev1.ObjectReference `json:"installJob,omitempty"`
	ExtensionRecord *corev1.ObjectReference `json:"extensionRecord,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Extension is the CRD type for a request to add an extension to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=="Ready")].status"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Extension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtensionSpec   `json:"spec,omitempty"`
	Status ExtensionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionList contains a list of Extension
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Extension `json:"items"`
}

// ExtensionSpec specifies details about an extension that has been added to Crossplane
type ExtensionSpec struct {
	AppMetadataSpec `json:",inline"`
	CRDs            CRDList         `json:"customresourcedefinitions,omitempty"`
	Controller      ControllerSpec  `json:"controller,omitempty"`
	Permissions     PermissionsSpec `json:"permissions,omitempty"`
}

// ExtensionStatus defines the observed state of Extension
type ExtensionStatus struct {
	corev1alpha1.ConditionedStatus
	ControllerRef *corev1.ObjectReference `json:"controllerRef,omitempty"`
}

// AppMetadataSpec defines metadata about the extension application
type AppMetadataSpec struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Icons       []IconSpec        `json:"icons,omitempty"`
	Maintainers []ContributorSpec `json:"maintainers,omitempty"`
	Owners      []ContributorSpec `json:"owners,omitempty"`
	Company     string            `json:"company,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Links       []LinkSpec        `json:"links,omitempty"`
	License     string            `json:"license,omitempty"`
}

// CRDList is the full list of CRDs that this extension owns and depends on
type CRDList struct {
	// Owned is the list of CRDs that this extension defines and owns
	Owned []metav1.TypeMeta `json:"owns,omitempty"`

	// DependsOn is the list of CRDs that this extension depends on. This data drives the
	// dependency resolution process.
	DependsOn []metav1.TypeMeta `json:"dependsOn,omitempty"`
}

// NewCRDList creates a new CRDList with its members initialized.
func NewCRDList() *CRDList {
	return &CRDList{
		Owned:     []metav1.TypeMeta{},
		DependsOn: []metav1.TypeMeta{},
	}
}

// IconSpec defines the icon for an extension
type IconSpec struct {
	Base64IconData string `json:"base64Data"`
	MediaType      string `json:"mediatype"`
}

// ContributorSpec defines a contributor for an extension (e.g., maintainer, owner, etc.)
type ContributorSpec struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// LinkSpec defines a useful link about an extension (e.g., homepage, about page, etc.)
type LinkSpec struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// ControllerSpec defines the controller that implements the logic for an extension, which can come
// in different flavors. A golang code (controller-runtime) controller with a managing Deployment
// is all that is supported currently, but more types will come in the future (e.g., templates,
// functions/hooks, templates, a new DSL, etc.
type ControllerSpec struct {
	Deployment *ControllerDeployment `json:"deployment,omitempty"`
}

// ControllerDeployment defines a controller for an extension that is managed by a Deployment.
type ControllerDeployment struct {
	Name string              `json:"name"`
	Spec apps.DeploymentSpec `json:"spec"`
}

// PermissionsSpec defines the permissions that an extension will require to operate.
type PermissionsSpec struct {
	Rules []rbac.PolicyRule `json:"rules,omitempty"`
}
