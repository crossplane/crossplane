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

package v2alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Condition types.
const (
	// A TypeEstablished XRD has created the CRD for its composite resource and
	// started a controller to reconcile instances of said resource.
	TypeEstablished xpv1.ConditionType = "Established"
)

// Reasons a resource is or is not established or offered.
const (
	EstablishedManagedResource xpv1.ConditionReason = "EstablishedManagedResource"
	ReasonPendingManaged       xpv1.ConditionReason = "PendingManagedResource"
	ReasonInactiveManaged      xpv1.ConditionReason = "InactiveManagedResource"
	ReasonTerminatingManaged   xpv1.ConditionReason = "TerminatingManagedResource"
)

// EstablishedManaged indicates that Crossplane has defined new kind of managed resource.
func EstablishedManaged() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             EstablishedManagedResource,
	}
}

// InactiveManaged indicates this managed resource is in the inactive state.
func InactiveManaged() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInactiveManaged,
	}
}

// PendingManaged indicates that Crossplane has defined and is waiting for a
// new kind of managed resource to become accepted.
func PendingManaged() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPendingManaged,
	}
}

// BlockedManaged indicates that Crossplane has encountered an error attempting to
// reconcile a managed resource definition.
func BlockedManaged() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPendingManaged,
	}
}

// TerminatingManaged indicates that Crossplane is terminating the controller
// for and removing the definition of a managed resource.
func TerminatingManaged() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingManaged,
	}
}
