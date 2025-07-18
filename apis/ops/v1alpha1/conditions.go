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
	// A TypeSucceeded condition indicates whether an operation has Succeeded.
	TypeSucceeded xpv1.ConditionType = "Succeeded"

	// A TypeValidPipeline Operation has a valid function pipeline.
	TypeValidPipeline xpv1.ConditionType = "ValidPipeline"

	// A TypeWatching condition indicates whether a WatchOperation is
	// actively watching resources.
	TypeWatching xpv1.ConditionType = "Watching"

	// A TypeScheduling condition indicates whether a CronOperation is
	// actively scheduling operations.
	TypeScheduling xpv1.ConditionType = "Scheduling"
)

// Reasons a package is or is not installed.
const (
	ReasonPipelineRunning xpv1.ConditionReason = "PipelineRunning"
	ReasonPipelineSuccess xpv1.ConditionReason = "PipelineSuccess"
	ReasonPipelineError   xpv1.ConditionReason = "PipelineError"

	ReasonValidPipeline       xpv1.ConditionReason = "ValidPipeline"
	ReasonMissingCapabilities xpv1.ConditionReason = "MissingCapabilities"

	ReasonWatchActive xpv1.ConditionReason = "WatchActive"
	ReasonWatchFailed xpv1.ConditionReason = "WatchFailed"
	ReasonWatchPaused xpv1.ConditionReason = "WatchPaused"

	ReasonScheduleActive  xpv1.ConditionReason = "ScheduleActive"
	ReasonScheduleInvalid xpv1.ConditionReason = "ScheduleInvalid"
	ReasonSchedulePaused  xpv1.ConditionReason = "SchedulePaused"
)

// Running indicates that an operation is running.
func Running() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineRunning,
	}
}

// Complete indicates that an operation is complete.
func Complete() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineSuccess,
	}
}

// Failed indicates that an operation has failed.
func Failed(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeSucceeded,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonPipelineError,
		Message:            message,
	}
}

// ValidPipeline indicates that an operation has a valid function pipeline.
func ValidPipeline() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonValidPipeline,
	}
}

// MissingCapabilities indicates that an operation's functions are missing
// required capabilities.
func MissingCapabilities(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeValidPipeline,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonMissingCapabilities,
		Message:            message,
	}
}

// WatchActive indicates that a WatchOperation is actively watching resources.
func WatchActive() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeWatching,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchActive,
	}
}

// WatchFailed indicates that a WatchOperation failed to establish or maintain
// its watch.
func WatchFailed(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeWatching,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchFailed,
		Message:            message,
	}
}

// WatchPaused indicates that a WatchOperation is paused and not
// actively watching resources.
func WatchPaused() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeWatching,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonWatchPaused,
	}
}

// ScheduleActive indicates that a CronOperation is actively scheduling
// operations.
func ScheduleActive() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeScheduling,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonScheduleActive,
	}
}

// ScheduleInvalid indicates that a CronOperation has an invalid cron schedule.
func ScheduleInvalid(message string) xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeScheduling,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonScheduleInvalid,
		Message:            message,
	}
}

// SchedulePaused indicates that a CronOperation is paused and not
// actively scheduling operations.
func SchedulePaused() xpv1.Condition {
	return xpv1.Condition{
		Type:               TypeScheduling,
		Status:             corev1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             ReasonSchedulePaused,
	}
}
