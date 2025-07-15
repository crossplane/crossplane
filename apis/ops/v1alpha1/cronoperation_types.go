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

// LabelCronOperationName is the label Crossplane adds to Operations to
// represent the CronOperation that created them.
const LabelCronOperationName = "ops.crossplane.io/cronoperation"

// ConcurrencyPolicy specifies how to treat concurrent executions of an
// operation.
type ConcurrencyPolicy string

const (
	// ConcurrencyPolicyAllow allows concurrent executions.
	ConcurrencyPolicyAllow ConcurrencyPolicy = "Allow"

	// ConcurrencyPolicyForbid forbids concurrent executions, skipping the next
	// run if the previous run hasn't finished yet.
	ConcurrencyPolicyForbid ConcurrencyPolicy = "Forbid"

	// ConcurrencyPolicyReplace replaces the currently running operation with a
	// new one.
	ConcurrencyPolicyReplace ConcurrencyPolicy = "Replace"
)

// CronOperationSpec specifies the desired state of a CronOperation.
type CronOperationSpec struct {
	// Schedule is the cron schedule for the operation.
	Schedule string `json:"schedule"`

	// StartingDeadlineSeconds is the deadline in seconds for starting the
	// operation if it misses its scheduled time for any reason.
	// +optional
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	// ConcurrencyPolicy specifies how to treat concurrent executions of an
	// operation.
	// +optional
	// +kubebuilder:default=Allow
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	ConcurrencyPolicy *ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// Suspend specifies whether the CronOperation should be suspended.
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

// OperationTemplate is a template for creating an Operation.
type OperationTemplate struct {
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the Operation to be created.
	Spec OperationSpec `json:"spec"`
}

// CronOperationStatus represents the observed state of a CronOperation.
type CronOperationStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// RunningOperationRefs is a list of currently running Operations.
	// +optional
	RunningOperationRefs []RunningOperationRef `json:"runningOperationRefs,omitempty"`

	// LastScheduleTime is the last time the CronOperation was scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// LastSuccessfulTime is the last time the CronOperation was successfully
	// completed.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
}

// A RunningOperationRef is a reference to a running operation.
type RunningOperationRef struct {
	// Name of the active operation.
	Name string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient

// A CronOperation creates Operations on a cron schedule.
//
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SCHEDULE",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="SUSPEND",type="boolean",JSONPath=".spec.suspend"
// +kubebuilder:printcolumn:name="LAST SCHEDULE",type="date",JSONPath=".status.lastScheduleTime"
// +kubebuilder:printcolumn:name="LAST SUCCESS",type="date",JSONPath=".status.lastSuccessfulTime"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=cronops
type CronOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronOperationSpec   `json:"spec,omitempty"`
	Status CronOperationStatus `json:"status,omitempty"`
}

// SetConditions delegates to Status.SetConditions.
// Implements Conditioned.SetConditions.
func (co *CronOperation) SetConditions(cs ...xpv1.Condition) {
	co.Status.SetConditions(cs...)
}

// GetCondition delegates to Status.GetCondition.
// Implements Conditioned.GetCondition.
func (co *CronOperation) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return co.Status.GetCondition(ct)
}

// +kubebuilder:object:root=true

// CronOperationList contains a list of CronOperations.
type CronOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CronOperation `json:"items"`
}
