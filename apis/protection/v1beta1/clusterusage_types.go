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

	"github.com/crossplane/crossplane/internal/protection"
)

// A ClusterUsage defines a deletion blocking relationship between two
// resources.
//
// Usages prevent accidental deletion of a single resource or deletion of
// resources with dependent resources.
//
// Read the Crossplane documentation for
// [more information about usages](https://docs.crossplane.io/latest/concepts/usages).
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="DETAILS",type="string",JSONPath=".metadata.annotations.crossplane\\.io/usage-details"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:scope=cluster,subresource:status
type ClusterUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UsageSpec   `json:"spec"`
	Status            UsageStatus `json:"status,omitempty"`
}

// GetUserOf gets the resource this ClusterUsage indicates a use of.
func (u *ClusterUsage) GetUserOf() protection.Resource {
	conv := GeneratedResourceConverter{}
	return conv.ToInternal(u.Spec.Of)
}

// SetUserOf sets the resource this ClusterUsage indicates a use of.
func (u *ClusterUsage) SetUserOf(r protection.Resource) {
	conv := GeneratedResourceConverter{}
	u.Spec.Of = conv.FromInternal(r)
}

// GetUsedBy gets the resource this ClusterUsage indicates a use by.
func (u *ClusterUsage) GetUsedBy() *protection.Resource {
	if u.Spec.By == nil {
		return nil
	}
	conv := GeneratedResourceConverter{}
	out := conv.ToInternal(*u.Spec.By)
	return &out
}

// SetUsedBy sets the resource this ClusterUsage indicates a use by.
func (u *ClusterUsage) SetUsedBy(r *protection.Resource) {
	if r == nil {
		u.Spec.By = nil
		return
	}
	conv := GeneratedResourceConverter{}
	out := conv.FromInternal(*r)
	u.Spec.By = &out
}

// GetReason gets the reason this ClusterUsage exists.
func (u *ClusterUsage) GetReason() *string {
	return u.Spec.Reason
}

// SetReason sets the reason this ClusterUsage exists.
func (u *ClusterUsage) SetReason(reason *string) {
	u.Spec.Reason = reason
}

// GetReplayDeletion gets a boolean that indicates whether deletion of the used
// resource will be replayed when this ClusterUsage is deleted.
func (u *ClusterUsage) GetReplayDeletion() *bool {
	return u.Spec.ReplayDeletion
}

// SetReplayDeletion specifies whether deletion of the used resource will be
// replayed when this ClusterUsage is deleted.
func (u *ClusterUsage) SetReplayDeletion(replay *bool) {
	u.Spec.ReplayDeletion = replay
}

// GetCondition of this ClusterUsage.
func (u *ClusterUsage) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return u.Status.GetCondition(ct)
}

// SetConditions of this ClusterUsage.
func (u *ClusterUsage) SetConditions(c ...xpv1.Condition) {
	u.Status.SetConditions(c...)
}

// +kubebuilder:object:root=true

// ClusterUsageList contains a list of Usage.
type ClusterUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterUsage `json:"items"`
}
