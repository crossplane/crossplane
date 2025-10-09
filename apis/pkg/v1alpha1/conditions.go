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
	// TypeValidated indicates the transaction has completed pre-flight validation.
	TypeValidated xpv1.ConditionType = "Validated"

	// TypeInstalled indicates the transaction has completed package installation.
	TypeInstalled xpv1.ConditionType = "Installed"

	// TypeSucceeded indicates the transaction has completed (successfully or with failure).
	TypeSucceeded xpv1.ConditionType = "Succeeded"
)

// Condition reasons.
const (
	// Validation reasons.
	ReasonValidationPassed   xpv1.ConditionReason = "ValidationPassed"
	ReasonMissingDependency  xpv1.ConditionReason = "MissingDependency"
	ReasonVersionConflict    xpv1.ConditionReason = "VersionConflict"
	ReasonCircularDependency xpv1.ConditionReason = "CircularDependency"
	ReasonCRDConflict        xpv1.ConditionReason = "CRDConflict"
	ReasonSchemaIncompatible xpv1.ConditionReason = "SchemaIncompatible"

	// Installation reasons.
	ReasonInstallationComplete   xpv1.ConditionReason = "InstallationComplete"
	ReasonInstallationInProgress xpv1.ConditionReason = "InstallationInProgress"
	ReasonInstallationPending    xpv1.ConditionReason = "InstallationPending"
	ReasonCRDInstallationFailed  xpv1.ConditionReason = "CRDInstallationFailed"

	// Transaction reasons.
	ReasonTransactionRunning  xpv1.ConditionReason = "TransactionRunning"
	ReasonTransactionComplete xpv1.ConditionReason = "TransactionComplete"
	ReasonTransactionFailed   xpv1.ConditionReason = "TransactionFailed"
)

// ValidationPassed indicates that pre-flight validation succeeded.
func ValidationPassed() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidated,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidationPassed,
	}
}

// ValidationFailed indicates that pre-flight validation failed.
func ValidationFailed(reason xpv1.ConditionReason, message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidated,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// InstallationPending indicates that installation is pending validation.
func InstallationPending() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInstallationPending,
	}
}

// InstallationInProgress indicates that package installation is in progress.
func InstallationInProgress() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInstallationInProgress,
	}
}

// InstallationComplete indicates that package installation completed successfully.
func InstallationComplete() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonInstallationComplete,
	}
}

// InstallationFailed indicates that package installation failed.
func InstallationFailed(reason xpv1.ConditionReason, message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeInstalled,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
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
