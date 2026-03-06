/*
Copyright 2020 The Crossplane Authors.

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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// Condition types.
const (
	// A TypeEstablished XRD has created the CRD for its composite resource and
	// started a controller to reconcile instances of said resource.
	TypeEstablished xpv2.ConditionType = "Established"

	// A TypeHealthy indicates the resource is healthy and working.
	TypeHealthy xpv2.ConditionType = "Healthy"
)

// Reasons a resource is or is not healthy, established or offered.
const (
	ReasonHealthy                     xpv2.ConditionReason = "Running"
	ReasonUnhealthy                   xpv2.ConditionReason = "EncounteredErrors"
	EstablishedManagedResource        xpv2.ConditionReason = "EstablishedManagedResource"
	ReasonPendingManaged              xpv2.ConditionReason = "PendingManagedResource"
	ReasonInactiveManaged             xpv2.ConditionReason = "InactiveManagedResource"
	ReasonBlockedActivationPolicy     xpv2.ConditionReason = "BlockedManagedResourceActivationPolicy"
	ReasonTerminatingManaged          xpv2.ConditionReason = "TerminatingManagedResource"
	ReasonTerminatingActivationPolicy xpv2.ConditionReason = "TerminatingManagedResourceActivationPolicy"
)

// EstablishedManaged indicates that Crossplane has defined new kind of managed resource.
func EstablishedManaged() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             EstablishedManagedResource,
	}
}

// InactiveManaged indicates this managed resource is in the inactive state.
func InactiveManaged() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInactiveManaged,
	}
}

// PendingManaged indicates that Crossplane has defined and is waiting for a
// new kind of managed resource to become accepted.
func PendingManaged() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPendingManaged,
	}
}

// BlockedManaged indicates that Crossplane has encountered an error attempting to
// reconcile a managed resource definition.
func BlockedManaged() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPendingManaged,
	}
}

// Healthy indicates that the controller is running as expected.
func Healthy() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonHealthy,
	}
}

// Unhealthy indicates that the controller is running into issues.
func Unhealthy() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnhealthy,
	}
}

// BlockedActivationPolicy indicates that Crossplane is blocked attempting to
// reconcile of a managed resource activation policy.
func BlockedActivationPolicy() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonBlockedActivationPolicy,
	}
}

// TerminatingActivationPolicy indicates that Crossplane is terminating the
// controller for and removing the definition of a managed resource activation
// policy.
func TerminatingActivationPolicy() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingActivationPolicy,
	}
}

// TerminatingManaged indicates that Crossplane is terminating the controller
// for and removing the definition of a managed resource.
func TerminatingManaged() xpv2.Condition {
	return xpv2.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingManaged,
	}
}
