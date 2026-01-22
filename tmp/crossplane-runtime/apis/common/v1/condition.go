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

package v1

import (
	"github.com/crossplane/crossplane-runtime/v2/apis/common"
)

// A ConditionType represents a condition a resource could be in.
type ConditionType = common.ConditionType

// Condition types.
const (
	// TypeReady resources are believed to be ready to handle work.
	TypeReady ConditionType = common.TypeReady

	// TypeSynced resources are believed to be in sync with the
	// Kubernetes resources that manage their lifecycle.
	TypeSynced ConditionType = common.TypeSynced

	// TypeHealthy resources are believed to be in a healthy state and to have all
	// of their child resources in a healthy state. For example, a claim is
	// healthy when the claim is synced and the underlying composite resource is
	// both synced and healthy. A composite resource is healthy when the composite
	// resource is synced and all composed resources are synced and, if
	// applicable, healthy (e.g., the composed resource is a composite resource).
	// TODO: This condition is not yet implemented. It is currently just reserved
	// as a system condition. See the tracking issue for more details
	// https://github.com/crossplane/crossplane/issues/5643.
	TypeHealthy ConditionType = common.TypeHealthy
)

// A ConditionReason represents the reason a resource is in a condition.
type ConditionReason = common.ConditionReason

// Reasons a resource is or is not ready.
const (
	ReasonAvailable   = common.ReasonAvailable
	ReasonUnavailable = common.ReasonUnavailable
	ReasonCreating    = common.ReasonCreating
	ReasonDeleting    = common.ReasonDeleting
)

// Reasons a resource is or is not synced.
const (
	ReasonReconcileSuccess = common.ReasonReconcileSuccess
	ReasonReconcileError   = common.ReasonReconcileError
	ReasonReconcilePaused  = common.ReasonReconcilePaused
)

// See https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

// A Condition that may apply to a resource.
type Condition = common.Condition

// IsSystemConditionType returns true if the condition is owned by the
// Crossplane system (e.g, Ready, Synced, Healthy).
func IsSystemConditionType(t ConditionType) bool {
	return common.IsSystemConditionType(t)
}

// NOTE(negz): Conditions are implemented as a slice rather than a map to comply
// with Kubernetes API conventions. Ideally we'd comply by using a map that
// marshalled to a JSON array, but doing so confuses the CRD schema generator.
// https://github.com/kubernetes/community/blob/9bf8cd/contributors/devel/sig-architecture/api-conventions.md#lists-of-named-subobjects-preferred-over-maps

// NOTE(negz): Do not manipulate Conditions directly. Use the Set method.

// A ConditionedStatus reflects the observed status of a resource. Only
// one condition of each type may exist.
type ConditionedStatus = common.ConditionedStatus

// NewConditionedStatus returns a stat with the supplied conditions set.
func NewConditionedStatus(c ...Condition) *ConditionedStatus {
	return common.NewConditionedStatus(c...)
}

// Creating returns a condition that indicates the resource is currently
// being created.
func Creating() Condition {
	return common.Creating()
}

// Deleting returns a condition that indicates the resource is currently
// being deleted.
func Deleting() Condition {
	return common.Deleting()
}

// Available returns a condition that indicates the resource is
// currently observed to be available for use.
func Available() Condition {
	return common.Available()
}

// Unavailable returns a condition that indicates the resource is not
// currently available for use. Unavailable should be set only when Crossplane
// expects the resource to be available but knows it is not, for example
// because its API reports it is unhealthy.
func Unavailable() Condition {
	return common.Unavailable()
}

// ReconcileSuccess returns a condition indicating that Crossplane successfully
// completed the most recent reconciliation of the resource.
func ReconcileSuccess() Condition {
	return common.ReconcileSuccess()
}

// ReconcileError returns a condition indicating that Crossplane encountered an
// error while reconciling the resource. This could mean Crossplane was
// unable to update the resource to reflect its desired state, or that
// Crossplane was unable to determine the current actual state of the resource.
func ReconcileError(err error) Condition {
	return common.ReconcileError(err)
}

// ReconcilePaused returns a condition that indicates reconciliation on
// the managed resource is paused via the pause annotation.
func ReconcilePaused() Condition {
	return common.ReconcilePaused()
}
