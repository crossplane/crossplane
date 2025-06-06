/*
Copyright 2023 The Crossplane Authors.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A Function installs an OCI compatible Crossplane package, extending
// Crossplane with support for a new kind of composition function.
//
// Read the Crossplane documentation for
// [more information about Functions](https://docs.crossplane.io/latest/concepts/composition-functions).
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="INSTALLED",type="string",JSONPath=".status.conditions[?(@.type=='Installed')].status"
// +kubebuilder:printcolumn:name="HEALTHY",type="string",JSONPath=".status.conditions[?(@.type=='Healthy')].status"
// +kubebuilder:printcolumn:name="PACKAGE",type="string",JSONPath=".spec.package"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,pkg}
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FunctionSpec   `json:"spec,omitempty"`
	Status FunctionStatus `json:"status,omitempty"`
}

// FunctionSpec specifies the configuration of a Function.
type FunctionSpec struct {
	PackageSpec `json:",inline"`

	PackageRuntimeSpec `json:",inline"`
}

// FunctionStatus represents the observed state of a Function.
type FunctionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
	PackageStatus          `json:",inline"`

	// Type of this function.
	// A Composition function can only be used in Composition pipelines.
	// An Operation function can only be used in Operation pipelines.
	// +kubebuilder:validation:Enum=Composition;Operation
	Type FunctionType `json:"type,omitempty"`
}

// +kubebuilder:object:root=true

// FunctionList contains a list of Function.
type FunctionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Function `json:"items"`
}

// FunctionRevisionSpec specifies configuration for a FunctionRevision.
type FunctionRevisionSpec struct {
	PackageRevisionSpec        `json:",inline"`
	PackageRevisionRuntimeSpec `json:",inline"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A FunctionRevision represents a revision of a Function. Crossplane
// creates new revisions when there are changes to the Function.
//
// Crossplane creates and manages FunctionRevisions. Don't directly edit
// FunctionRevisions.
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="HEALTHY",type="string",JSONPath=".status.conditions[?(@.type=='Healthy')].status"
// +kubebuilder:printcolumn:name="REVISION",type="string",JSONPath=".spec.revision"
// +kubebuilder:printcolumn:name="IMAGE",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".spec.desiredState"
// +kubebuilder:printcolumn:name="DEP-FOUND",type="string",JSONPath=".status.foundDependencies"
// +kubebuilder:printcolumn:name="DEP-INSTALLED",type="string",JSONPath=".status.installedDependencies"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,pkgrev}
type FunctionRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FunctionRevisionSpec   `json:"spec,omitempty"`
	Status FunctionRevisionStatus `json:"status,omitempty"`
}

// FunctionRevisionStatus represents the observed state of a FunctionRevision.
type FunctionRevisionStatus struct {
	PackageRevisionStatus `json:",inline"`

	// Endpoint is the gRPC endpoint where Crossplane will send
	// RunFunctionRequests.
	Endpoint string `json:"endpoint,omitempty"`

	// Type of this function.
	// A Composition function can only be used in Composition pipelines.
	// An Operation function can only be used in Operation pipelines.
	// +kubebuilder:validation:Enum=Composition;Operation
	Type FunctionType `json:"type,omitempty"`
}

// A FunctionType represents the type of Function.
type FunctionType string

const (
	// FunctionTypeComposition functions are used in a composition pipeline.
	FunctionTypeComposition FunctionType = "Composition"

	// FunctionTypeOperation functions are used in an operation pipeline.
	FunctionTypeOperation FunctionType = "Operation"
)

// +kubebuilder:object:root=true

// FunctionRevisionList contains a list of FunctionRevision.
type FunctionRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FunctionRevision `json:"items"`
}
