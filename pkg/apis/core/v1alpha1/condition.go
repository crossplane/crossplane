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

// ConditionType type for possible conditions the resource could be in.
type ConditionType string

// Resource conditions.
const (
	// Pending means that the resource create request has been received and is waiting to be fulfilled.
	Pending ConditionType = "Pending"
	// Creating means that the resource create request has been accepted and the resource is in
	// the process of being created.
	Creating ConditionType = "Creating"
	// Deleting means that the resource is in the process of being deleted.
	Deleting ConditionType = "Deleting"
	// Failed means that the resource is in a failure state, for example it failed to be created.
	Failed ConditionType = "Failed"
	// Ready means that the resource creation has been successful and the resource is ready to
	// accept requests and perform operations.
	Ready ConditionType = "Ready"
)

// Condition contains details for the current condition of this pod.
type Condition struct {
	Type               ConditionType
	Status             corev1.ConditionStatus
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// Equal returns true if the condition is identical to the supplied condition,
// ignoring the LastTransitionTime. github.com/go-test/deep uses this method to
// test equality.
func (c Condition) Equal(other Condition) bool {
	return c.Type == other.Type &&
		c.Status == other.Status &&
		c.Reason == other.Reason &&
		c.Message == other.Message
}

// Conditionable defines set of functionality to operate on Conditions
type Conditionable interface {
	Condition(ConditionType) *Condition
	SetCondition(Condition)
	RemoveCondition(ConditionType)
	UnsetCondition(ConditionType)
	UnsetAllConditions()
}

// ConditionedStatus defines the observed state of RDS resource
type ConditionedStatus struct {
	// Conditions indicate state for particular aspects of a CustomResourceDefinition
	Conditions []Condition
}

// Condition returns a provider condition with the provided type if it exists.
func (c *ConditionedStatus) Condition(conditionType ConditionType) *Condition {
	for _, c := range c.Conditions {
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// IsCondition of provided type is present and set to true
func (c *ConditionedStatus) IsCondition(ctype ConditionType) bool {
	condition := c.Condition(ctype)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// IsReady returns true if the status is currently ready.
func (c *ConditionedStatus) IsReady() bool {
	return c.IsCondition(Ready)
}

// IsFailed returns true if the status is currently failed.
func (c *ConditionedStatus) IsFailed() bool {
	return c.IsCondition(Failed)
}

// SetCondition adds/replaces the given condition in the credentials controller status.
func (c *ConditionedStatus) SetCondition(condition Condition) {
	current := c.Condition(condition.Type)
	if current != nil && current.Equal(condition) {
		return
	}
	newConditions := FilterOutCondition(c.Conditions, condition.Type)
	newConditions = append(newConditions, condition)
	c.Conditions = newConditions
}

// SetFailed set failed as an active condition
func (c *ConditionedStatus) SetFailed(reason, msg string) {
	c.SetCondition(NewCondition(Failed, reason, msg))
}

// SetReady set ready as an active condition
func (c *ConditionedStatus) SetReady() {
	c.SetCondition(NewCondition(Ready, "", ""))
}

// SetCreating set creating as an active condition
func (c *ConditionedStatus) SetCreating() {
	c.SetCondition(NewCondition(Creating, "", ""))
}

// SetPending set pending as an active condition
func (c *ConditionedStatus) SetPending() {
	c.SetCondition(NewCondition(Pending, "", ""))
}

// SetDeleting set creating as an active condition
func (c *ConditionedStatus) SetDeleting() {
	c.SetCondition(NewCondition(Deleting, "", ""))
}

// UnsetCondition set condition status to false with the given type - if found.
func (c *ConditionedStatus) UnsetCondition(conditionType ConditionType) {
	current := c.Condition(conditionType)
	if current != nil && current.Status == corev1.ConditionTrue {
		current.Status = corev1.ConditionFalse
		c.SetCondition(*current)
	}
}

// UnsetAllConditions set conditions status to false on all conditions
func (c *ConditionedStatus) UnsetAllConditions() {
	for i := range c.Conditions {
		c.Conditions[i].Status = corev1.ConditionFalse
	}
}

// RemoveCondition removes the condition with the provided type from the credentials controller status.
func (c *ConditionedStatus) RemoveCondition(condType ConditionType) {
	c.Conditions = FilterOutCondition(c.Conditions, condType)
}

// RemoveAllConditions removes all condition entries
func (c *ConditionedStatus) RemoveAllConditions() {
	c.Conditions = []Condition{}
}

// NewCondition creates a new resource condition.
func NewCondition(condType ConditionType, reason, msg string) Condition {
	return Condition{
		Type:               condType,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// FilterOutCondition returns a new slice of credentials controller conditions
// without conditions with the provided type.
func FilterOutCondition(conditions []Condition, condType ConditionType) []Condition {
	var newConditions []Condition // nolint:prealloc
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
