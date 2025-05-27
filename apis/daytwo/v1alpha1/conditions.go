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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// Condition types.
const (
	// A TypeComplete condition indicates whether an operation has completed.
	TypeComplete xpv1.ConditionType = "Complete"

	// A TypeFailed condition indicates whether an operation has failed.
	TypeFailed xpv1.ConditionType = "Failed"
)

// Reasons a package is or is not installed.
const (
	ReasonPipelineRunning xpv1.ConditionReason = "PipelineRunning"
	ReasonPipelineSuccess xpv1.ConditionReason = "PipelineSuccess"
	ReasonPipelineError   xpv1.ConditionReason = "PipelineError"
)

// Running indicates that an operation is running.
func Running() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeComplete,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineRunning,
	}
}

// Complete indicates that an operation is complete.
func Complete() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeComplete,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineSuccess,
	}
}

// Failed indicates that an operation has failed.
func Failed() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeFailed,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineError,
	}
}
