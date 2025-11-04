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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// Condition types.
const (
	// TypeResolved indicates the transaction has completed dependency resolution.
	TypeResolved xpv1.ConditionType = "Resolved"

	// TypeValidated indicates the transaction has completed pre-flight validation.
	TypeValidated xpv1.ConditionType = "Validated"

	// TypeInstalled indicates the transaction has completed package installation.
	TypeInstalled xpv1.ConditionType = "Installed"

	// TypeSucceeded indicates the transaction has completed (successfully or with failure).
	TypeSucceeded xpv1.ConditionType = "Succeeded"
)

// Condition reasons.
const (
	// Resolution reasons.
	ReasonResolved         xpv1.ConditionReason = "Resolved"
	ReasonResolutionFailed xpv1.ConditionReason = "ResolutionFailed"

	// Validation reasons.
	ReasonValidated        xpv1.ConditionReason = "Validated"
	ReasonValidationFailed xpv1.ConditionReason = "ValidationFailed"

	// Installation reasons.
	ReasonInstalled          xpv1.ConditionReason = "Installed"
	ReasonInstallationFailed xpv1.ConditionReason = "InstallationFailed"

	// Transaction reasons.
	ReasonTransactionRunning  xpv1.ConditionReason = "TransactionRunning"
	ReasonTransactionBlocked  xpv1.ConditionReason = "TransactionBlocked"
	ReasonTransactionComplete xpv1.ConditionReason = "TransactionComplete"
	ReasonTransactionFailed   xpv1.ConditionReason = "TransactionFailed"
)

// ResolutionSuccess indicates that dependency resolution succeeded.
func ResolutionSuccess() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonResolved,
	}
}

// ResolutionError indicates that dependency resolution failed.
func ResolutionError(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeResolved,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonResolutionFailed,
		Message:            message,
	}
}

// ValidationSuccess indicates that pre-flight validation succeeded.
func ValidationSuccess() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidated,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidated,
	}
}

// ValidationError indicates that pre-flight validation failed.
func ValidationError(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidated,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidationFailed,
		Message:            message,
	}
}

// InstallationSuccess indicates that package installation completed successfully.
func InstallationSuccess() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInstalled,
	}
}

// InstallationError indicates that package installation failed.
func InstallationError(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInstallationFailed,
		Message:            message,
	}
}

// TransactionRunning indicates that a transaction is running.
func TransactionRunning() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTransactionRunning,
	}
}

// TransactionBlocked indicates that a transaction is waiting for a lock.
func TransactionBlocked(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTransactionBlocked,
		Message:            message,
	}
}

// TransactionComplete indicates that a transaction completed successfully.
func TransactionComplete() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTransactionComplete,
	}
}

// TransactionFailed indicates that a transaction failed.
func TransactionFailed(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonTransactionFailed,
		Message:            message,
	}
}
