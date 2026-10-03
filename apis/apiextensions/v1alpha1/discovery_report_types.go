/*
Copyright 2025 The Crossplane Authors.

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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// DiscoveryReportSpec specifies the desired state of resource discovery.
type DiscoveryReportSpec struct {
	// MRDRef references the ManagedResourceDefinition being discovered.
	// +kubebuilder:validation:Required
	MRDRef xpv2.TypedReference `json:"mrdRef"`

	// Interval specifies how often to discover resources of this kind.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1h"
	Interval metav1.Duration `json:"interval,omitempty"`

	// ProviderConfig specifies which ProviderConfig to use for discovery.
	// If empty, discovery runs for all ProviderConfigs of the MRD's provider.
	// +kubebuilder:validation:Optional
	ProviderConfig *xpv2.Reference `json:"providerConfig,omitempty"`
}

// DiscoveryReportStatus shows the observed state of resource discovery.
type DiscoveryReportStatus struct {
	xpv2.ConditionedStatus `json:",inline"`

	// DiscoveredResourceCount is the total number of resources found in the external system.
	// +kubebuilder:validation:Optional
	DiscoveredResourceCount *int64 `json:"discoveredResourceCount,omitempty"`

	// UnmanagedResourceCount is the number of discovered resources with no corresponding CR.
	// +kubebuilder:validation:Optional
	UnmanagedResourceCount *int64 `json:"unmanagedResourceCount,omitempty"`

	// UnmanagedResources lists the external names of resources not managed by Crossplane.
	// +kubebuilder:validation:Optional
	UnmanagedResources []ExternalResource `json:"unmanagedResources,omitempty"`

	// LastDiscoveryTime is when the last discovery scan completed.
	// +kubebuilder:validation:Optional
	LastDiscoveryTime *metav1.Time `json:"lastDiscoveryTime,omitempty"`

	// NextDiscoveryTime is when the next discovery scan is scheduled.
	// +kubebuilder:validation:Optional
	NextDiscoveryTime *metav1.Time `json:"nextDiscoveryTime,omitempty"`
}

// ExternalResource represents a discovered external resource.
type ExternalResource struct {
	// ExternalName is the identifier of the resource in the external system.
	// This is the value used in the crossplane.io/external-name annotation.
	ExternalName string `json:"externalName"`

	// LastSeen is when this resource was last discovered.
	// +kubebuilder:validation:Optional
	LastSeen *metav1.Time `json:"lastSeen,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// DiscoveryReport documents the results of enumerating external resources of a given kind.
//
// +kubebuilder:printcolumn:name="MRD",type="string",JSONPath=".spec.mrdRef.name"
// +kubebuilder:printcolumn:name="DISCOVERED",type="integer",JSONPath=".status.discoveredResourceCount"
// +kubebuilder:printcolumn:name="UNMANAGED",type="integer",JSONPath=".status.unmanagedResourceCount"
// +kubebuilder:printcolumn:name="LAST-SCAN",type="date",JSONPath=".status.lastDiscoveryTime"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=dreport;dreports
type DiscoveryReport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DiscoveryReportSpec   `json:"spec,omitempty"`
	Status DiscoveryReportStatus `json:"status,omitempty"`
}

// GetCondition of this DiscoveryReport.
func (d *DiscoveryReport) GetCondition(ct xpv2.ConditionType) xpv2.Condition {
	return d.Status.GetCondition(ct)
}

// SetConditions of this DiscoveryReport.
func (d *DiscoveryReport) SetConditions(c ...xpv2.Condition) {
	d.Status.SetConditions(c...)
}

// +kubebuilder:object:root=true

// DiscoveryReportList contains a list of DiscoveryReports.
type DiscoveryReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DiscoveryReport `json:"items"`
}
