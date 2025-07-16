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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// LabelWatchOperationName is the label Crossplane adds to Operations to
// represent the WatchOperation that created them.
const LabelWatchOperationName = "ops.crossplane.io/watchoperation"

// WatchOperationSpec specifies the desired state of a WatchOperation.
type WatchOperationSpec struct {
	// Watch specifies the resource to watch.
	Watch WatchSpec `json:"watch"`

	// ConcurrencyPolicy specifies how to treat concurrent executions of an
	// operation.
	// +optional
	// +kubebuilder:default=Allow
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	ConcurrencyPolicy *ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// Suspend specifies whether the WatchOperation should be suspended.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`

	// SuccessfulHistoryLimit is the number of successful Operations to retain.
	// +optional
	// +kubebuilder:default=3
	SuccessfulHistoryLimit *int32 `json:"successfulHistoryLimit,omitempty"`

	// FailedHistoryLimit is the number of failed Operations to retain.
	// +optional
	// +kubebuilder:default=1
	FailedHistoryLimit *int32 `json:"failedHistoryLimit,omitempty"`

	// OperationTemplate is the template for the Operation to be created.
	OperationTemplate OperationTemplate `json:"operationTemplate"`
}

// WatchSpec specifies what resource to watch.
type WatchSpec struct {
	// APIVersion of the resource to watch.
	APIVersion string `json:"apiVersion"`

	// Kind of the resource to watch.
	Kind string `json:"kind"`

	// MatchLabels selects resources by label. If empty, all resources of the
	// specified kind are watched.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// Namespace selects resources in a specific namespace. If empty, all
	// namespaces are watched. Only applicable for namespaced resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// WatchOperationStatus represents the observed state of a WatchOperation.
type WatchOperationStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// RunningOperationRefs is a list of currently running Operations.
	// +optional
	RunningOperationRefs []RunningOperationRef `json:"runningOperationRefs,omitempty"`

	// WatchingResources is the number of resources this WatchOperation is
	// currently watching.
	// +optional
	WatchingResources int64 `json:"watchingResources,omitempty"`

	// LastScheduleTime is the last time the WatchOperation created an
	// Operation.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// LastSuccessfulTime is the last time the WatchOperation successfully
	// completed an Operation.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient

// A WatchOperation creates Operations when watched resources change.
//
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="WATCHING",type="string",JSONPath=".spec.watch.kind"
// +kubebuilder:printcolumn:name="SUSPEND",type="boolean",JSONPath=".spec.suspend"
// +kubebuilder:printcolumn:name="LAST SCHEDULE",type="date",JSONPath=".status.lastScheduleTime"
// +kubebuilder:printcolumn:name="LAST SUCCESS",type="date",JSONPath=".status.lastSuccessfulTime"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=watchops
type WatchOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WatchOperationSpec   `json:"spec,omitempty"`
	Status WatchOperationStatus `json:"status,omitempty"`
}

// SetConditions delegates to Status.SetConditions.
// Implements Conditioned.SetConditions.
func (wo *WatchOperation) SetConditions(cs ...xpv1.Condition) {
	wo.Status.SetConditions(cs...)
}

// GetCondition delegates to Status.GetCondition.
// Implements Conditioned.GetCondition.
func (wo *WatchOperation) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return wo.Status.GetCondition(ct)
}

// +kubebuilder:object:root=true

// WatchOperationList contains a list of WatchOperations.
type WatchOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WatchOperation `json:"items"`
}
