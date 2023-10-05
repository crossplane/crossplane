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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	// TODO(negz): Ideally our v1beta1 package wouldn't import types from v1, as
	// this strongly couples the types. This would make life difficult if we
	// wanted to evolve this package in a different direction from the current
	// v1 implementation. Unfortunately the package manager implementation
	// requires any type that is reconciled as a package (or revision) to
	// satisfy interfaces that involve returning v1 types.
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// Function is the CRD type for a request to deploy a long-running Function.
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
	v1.PackageSpec `json:",inline"`

	v1.PackageRuntimeSpec `json:",inline"`
}

// FunctionStatus represents the observed state of a Function.
type FunctionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
	v1.PackageStatus       `json:",inline"`
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
	v1.PackageRevisionSpec        `json:",inline"`
	v1.PackageRevisionRuntimeSpec `json:",inline"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// A FunctionRevision that has been added to Crossplane.
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
	v1.PackageRevisionStatus `json:",inline"`

	// Endpoint is the gRPC endpoint where Crossplane will send
	// RunFunctionRequests.
	Endpoint string `json:"endpoint,omitempty"`
}

// +kubebuilder:object:root=true

// FunctionRevisionList contains a list of FunctionRevision.
type FunctionRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FunctionRevision `json:"items"`
}
