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
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
)

// TODO: how do we pretty print conditioned status items? There may be multiple of them, and they
// can have varying status (e.g., True, False, Unknown)

// +kubebuilder:object:root=true

// StackRequest is the CRD type for a request to add a stack to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type==Ready)].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type StackRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackRequestSpec   `json:"spec,omitempty"`
	Status StackRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackRequestList contains a list of StackRequest
type StackRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StackRequest `json:"items"`
}

// StackRequestSpec specifies details about a request to add a stack to Crossplane.
type StackRequestSpec struct {
	// Source is the domain name for the stack registry hosting the stack being requested,
	// e.g., registry.crossplane.io
	Source string `json:"source,omitempty"`

	// Package is the name of the stack package that is being requested, e.g., myapp.
	// Either Package or CustomResourceDefinition can be specified.
	Package string `json:"package,omitempty"`

	// CustomResourceDefinition is the full name of a CRD that is owned by the stack being
	// requested. This can be a convenient way of installing a stack when the desired
	// CRD is known, but the package name that contains it is not known.
	// Either Package or CustomResourceDefinition can be specified.
	CustomResourceDefinition string `json:"crd,omitempty"`
}

// StackRequestStatus defines the observed state of StackRequest
type StackRequestStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	InstallJob  *corev1.ObjectReference `json:"installJob,omitempty"`
	StackRecord *corev1.ObjectReference `json:"stackRecord,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterStackRequest is the CRD type for a request to add a stack to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type==Ready)].status"
// +kubebuilder:printcolumn:name="SOURCE",type="string",JSONPath=".spec.source"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="CRD",type="string",JSONPath=".spec.crd"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterStackRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStackRequestSpec   `json:"spec,omitempty"`
	Status ClusterStackRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterStackRequestList contains a list of StackRequest
type ClusterStackRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterStackRequest `json:"items"`
}

// ClusterStackRequestSpec specifies details about a request to add a stack to Crossplane.
type ClusterStackRequestSpec struct {
	// Source is the domain name for the stack registry hosting the stack being requested,
	// e.g., registry.crossplane.io
	Source string `json:"source,omitempty"`

	// Package is the name of the stack package that is being requested, e.g., myapp.
	// Either Package or CustomResourceDefinition can be specified.
	Package string `json:"package,omitempty"`

	// CustomResourceDefinition is the full name of a CRD that is owned by the stack being
	// requested. This can be a convenient way of installing a stack when the desired
	// CRD is known, but the package name that contains it is not known.
	// Either Package or CustomResourceDefinition can be specified.
	CustomResourceDefinition string `json:"crd,omitempty"`
}

// ClusterStackRequestStatus defines the observed state of StackRequest
type ClusterStackRequestStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`

	InstallJob  *corev1.ObjectReference `json:"installJob,omitempty"`
	StackRecord *corev1.ObjectReference `json:"stackRecord,omitempty"`
}

// +kubebuilder:object:root=true

// Stack is the CRD type for a request to add a stack to Crossplane.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type==Ready)].status"
// +kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackSpec   `json:"spec,omitempty"`
	Status StackStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackList contains a list of Stack
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Stack `json:"items"`
}

// StackSpec specifies details about a stack that has been added to Crossplane
type StackSpec struct {
	AppMetadataSpec `json:",inline"`
	CRDs            CRDList         `json:"customresourcedefinitions,omitempty"`
	Controller      ControllerSpec  `json:"controller,omitempty"`
	Permissions     PermissionsSpec `json:"permissions,omitempty"`
}

// StackStatus defines the observed state of Stack
type StackStatus struct {
	runtimev1alpha1.ConditionedStatus `json:"conditionedStatus,omitempty"`
	ControllerRef                     *corev1.ObjectReference `json:"controllerRef,omitempty"`
}

// AppMetadataSpec defines metadata about the stack application
type AppMetadataSpec struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Icons       []IconSpec        `json:"icons,omitempty"`
	Maintainers []ContributorSpec `json:"maintainers,omitempty"`
	Owners      []ContributorSpec `json:"owners,omitempty"`
	Company     string            `json:"company,omitempty"`
	Category    string            `json:"category,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Website     string            `json:"website,omitempty"`
	Source      string            `json:"source,omitempty"`
	License     string            `json:"license,omitempty"`
}

// CRDList is the full list of CRDs that this stack owns and depends on
type CRDList struct {
	// Owned is the list of CRDs that this stack defines and owns
	Owned []metav1.TypeMeta `json:"owns,omitempty"`

	// DependsOn is the list of CRDs that this stack depends on. This data drives the
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

// IconSpec defines the icon for a stack
type IconSpec struct {
	Base64IconData string `json:"base64Data"`
	MediaType      string `json:"mediatype"`
}

// ContributorSpec defines a contributor for a stack (e.g., maintainer, owner, etc.)
type ContributorSpec struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// LinkSpec defines a useful link about a stack (e.g., homepage, about page, etc.)
type LinkSpec struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// ControllerSpec defines the controller that implements the logic for a stack, which can come
// in different flavors. A golang code (controller-runtime) controller with a managing Deployment
// is all that is supported currently, but more types will come in the future (e.g., templates,
// functions/hooks, templates, a new DSL, etc.
type ControllerSpec struct {
	Deployment *ControllerDeployment `json:"deployment,omitempty"`
	Job        *ControllerJob        `json:"job,omitempty"`
}

// ControllerDeployment defines a controller for a stack that is managed by a Deployment.
type ControllerDeployment struct {
	Name string              `json:"name"`
	Spec apps.DeploymentSpec `json:"spec"`
}

// ControllerJob defines a controller for a stack that is installed by a Job.
type ControllerJob struct {
	Name string        `json:"name"`
	Spec batch.JobSpec `json:"spec"`
}

// PermissionsSpec defines the permissions that a stack will require to operate.
type PermissionsSpec struct {
	Rules []rbac.PolicyRule `json:"rules,omitempty"`
}
