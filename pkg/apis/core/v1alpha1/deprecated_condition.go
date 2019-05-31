/*
Copyright 2018 The Crossplane Authors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeprecatedConditionType type for possible conditions the resource could be in.
type DeprecatedConditionType string

// Deprecated resource conditions. Please use the undeprecated equivalents for
// new APIs.
const (
	// DeprecatedPending means that the resource create request has been
	// received and is waiting to be fulfilled.
	DeprecatedPending DeprecatedConditionType = "Pending"

	// DeprecatedCreating means that the resource create request has been
	// accepted and the resource is in the process of being created.
	DeprecatedCreating DeprecatedConditionType = "Creating"

	// DeprecatedDeleting means that the resource is in the process of being deleted.
	DeprecatedDeleting DeprecatedConditionType = "Deleting"

	// DeprecatedFailed means that the resource is in a failure state, for example it failed to be created.
	DeprecatedFailed DeprecatedConditionType = "Failed"

	// DeprecatedReady means that the resource creation has been successful and the resource is ready to
	// accept requests and perform operations.
	DeprecatedReady DeprecatedConditionType = "Ready"
)

// DeprecatedCondition contains details for the current condition of a managed
// resource. Please use Condition instead for new APIs.
type DeprecatedCondition struct {
	Type               DeprecatedConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// Equal returns true if the condition is identical to the supplied condition,
// ignoring the LastTransitionTime. github.com/go-test/deep uses this method to
// test equality.
func (c DeprecatedCondition) Equal(other DeprecatedCondition) bool {
	return c.Type == other.Type &&
		c.Status == other.Status &&
		c.Reason == other.Reason &&
		c.Message == other.Message
}

// DeprecatedConditionable defines set of functionality to operate on Conditions
type DeprecatedConditionable interface {
	DeprecatedCondition(DeprecatedConditionType) *DeprecatedCondition
	SetDeprecatedCondition(DeprecatedCondition)
	RemoveDeprecatedCondition(DeprecatedConditionType)
	UnsetDeprecatedCondition(DeprecatedConditionType)
	UnsetAllDeprecatedConditions()
}

// DeprecatedConditionedStatus reflects the observed state of a managed
// resource. Please use ConditionedStatus for new resources.
type DeprecatedConditionedStatus struct {
	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []DeprecatedCondition
}

// DeprecatedCondition returns a provider condition with the provided type if it exists.
func (c *DeprecatedConditionedStatus) DeprecatedCondition(conditionType DeprecatedConditionType) *DeprecatedCondition {
	for i := range c.Conditions {
		// This loop is written this way (as opposed to for i, cnd := range...)
		// to avoid returning a pointer to a range variable whose content will
		// change as the loop iterates.
		// https://github.com/kyoh86/scopelint#whats-this
		cnd := c.Conditions[i]
		if cnd.Type == conditionType {
			return &cnd
		}
	}
	return nil
}

// IsDeprecatedCondition of provided type is present and set to true
func (c *DeprecatedConditionedStatus) IsDeprecatedCondition(ctype DeprecatedConditionType) bool {
	condition := c.DeprecatedCondition(ctype)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsReady returns true if the status is currently ready.
func (c *DeprecatedConditionedStatus) IsReady() bool {
	return c.IsDeprecatedCondition(DeprecatedReady)
}

// IsFailed returns true if the status is currently failed.
func (c *DeprecatedConditionedStatus) IsFailed() bool {
	return c.IsDeprecatedCondition(DeprecatedFailed)
}

// SetDeprecatedCondition adds/replaces the given condition in the credentials controller status.
func (c *DeprecatedConditionedStatus) SetDeprecatedCondition(condition DeprecatedCondition) {
	current := c.DeprecatedCondition(condition.Type)
	if current != nil && current.Equal(condition) {
		return
	}
	newConditions := FilterOutDeprecatedCondition(c.Conditions, condition.Type)
	newConditions = append(newConditions, condition)
	c.Conditions = newConditions
}

// SetFailed set failed as an active condition
func (c *DeprecatedConditionedStatus) SetFailed(reason, msg string) {
	c.SetDeprecatedCondition(NewDeprecatedCondition(DeprecatedFailed, reason, msg))
}

// SetReady set ready as an active condition
func (c *DeprecatedConditionedStatus) SetReady() {
	c.SetDeprecatedCondition(NewDeprecatedCondition(DeprecatedReady, "", ""))
}

// SetCreating set creating as an active condition
func (c *DeprecatedConditionedStatus) SetCreating() {
	c.SetDeprecatedCondition(NewDeprecatedCondition(DeprecatedCreating, "", ""))
}

// SetPending set pending as an active condition
func (c *DeprecatedConditionedStatus) SetPending() {
	c.SetDeprecatedCondition(NewDeprecatedCondition(DeprecatedPending, "", ""))
}

// SetDeleting set deleting as an active condition
func (c *DeprecatedConditionedStatus) SetDeleting() {
	c.SetDeprecatedCondition(NewDeprecatedCondition(DeprecatedDeleting, "", ""))
}

// UnsetDeprecatedCondition set condition status to false with the given type - if found.
func (c *DeprecatedConditionedStatus) UnsetDeprecatedCondition(conditionType DeprecatedConditionType) {
	current := c.DeprecatedCondition(conditionType)
	if current != nil && current.Status == corev1.ConditionTrue {
		current.Status = corev1.ConditionFalse
		c.SetDeprecatedCondition(*current)
	}
}

// UnsetAllDeprecatedConditions set conditions status to false on all conditions
func (c *DeprecatedConditionedStatus) UnsetAllDeprecatedConditions() {
	for i := range c.Conditions {
		c.Conditions[i].Status = corev1.ConditionFalse
	}
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func (c *DeprecatedConditionedStatus) RemoveCondition(condType DeprecatedConditionType) {
	c.Conditions = FilterOutDeprecatedCondition(c.Conditions, condType)
}

// RemoveAllConditions removes all condition entries
func (c *DeprecatedConditionedStatus) RemoveAllConditions() {
	c.Conditions = []DeprecatedCondition{}
}

// NewDeprecatedCondition creates a new resource condition.
func NewDeprecatedCondition(condType DeprecatedConditionType, reason, msg string) DeprecatedCondition {
	return DeprecatedCondition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// NewReadyDeprecatedCondition sets and activates Ready status condition
func NewReadyDeprecatedCondition() DeprecatedCondition {
	return NewDeprecatedCondition(DeprecatedReady, "", "")
}

// FilterOutDeprecatedCondition returns a new slice of credentials controller conditions
// without conditions with the provided type.
func FilterOutDeprecatedCondition(conditions []DeprecatedCondition, condType DeprecatedConditionType) []DeprecatedCondition {
	var newConditions []DeprecatedCondition // nolint:prealloc
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
