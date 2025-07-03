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

package shared

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

	// A TypeOffered XRD has created the CRD for its composite resource claim
	// and started a controller to reconcile instances of said claim.
	// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
	TypeOffered xpv1.ConditionType = "Offered"
)

// Reasons a resource is or is not established or offered.
const (
	ReasonWatchingComposite xpv1.ConditionReason = "WatchingCompositeResource"
	// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
	ReasonWatchingClaim xpv1.ConditionReason = "WatchingCompositeResourceClaim"

	ReasonTerminatingComposite xpv1.ConditionReason = "TerminatingCompositeResource"
	// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
	ReasonTerminatingClaim xpv1.ConditionReason = "TerminatingCompositeResourceClaim"
)

// WatchingComposite indicates that Crossplane has defined and is watching for a
// new kind of composite resource.
func WatchingComposite() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingComposite,
	}
}

// TerminatingComposite indicates that Crossplane is terminating the controller
// for and removing the definition of a composite resource.
func TerminatingComposite() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeEstablished,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingComposite,
	}
}

// WatchingClaim indicates that Crossplane has defined and is watching for a
// new kind of composite resource claim.
// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
func WatchingClaim() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchingClaim,
	}
}

// TerminatingClaim indicates that Crossplane is terminating the controller and
// removing the definition of a composite resource claim.
// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
func TerminatingClaim() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeOffered,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTerminatingClaim,
	}
}
