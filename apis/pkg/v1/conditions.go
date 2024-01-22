// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Condition types.
const (
	// A TypeInstalled indicates whether a package has been installed.
	TypeInstalled xpv1.ConditionType = "Installed"

	// A TypeHealthy indicates whether a package is healthy.
	TypeHealthy xpv1.ConditionType = "Healthy"
)

// Reasons a package is or is not installed.
const (
	ReasonUnpacking     xpv1.ConditionReason = "UnpackingPackage"
	ReasonInactive      xpv1.ConditionReason = "InactivePackageRevision"
	ReasonActive        xpv1.ConditionReason = "ActivePackageRevision"
	ReasonUnhealthy     xpv1.ConditionReason = "UnhealthyPackageRevision"
	ReasonHealthy       xpv1.ConditionReason = "HealthyPackageRevision"
	ReasonUnknownHealth xpv1.ConditionReason = "UnknownPackageRevisionHealth"
)

// Unpacking indicates that the package manager is waiting for a package
// revision to be unpacked.
func Unpacking() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnpacking,
	}
}

// Inactive indicates that the package manager is waiting for a package
// revision to be transitioned to an active state.
func Inactive() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInactive,
	}
}

// Active indicates that the package manager has installed and activated
// a package revision.
func Active() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonActive,
	}
}

// Unhealthy indicates that the current revision is unhealthy.
func Unhealthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnhealthy,
	}
}

// Healthy indicates that the current revision is healthy.
func Healthy() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonHealthy,
	}
}

// UnknownHealth indicates that the health of the current revision is unknown.
func UnknownHealth() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeHealthy,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonUnknownHealth,
	}
}
