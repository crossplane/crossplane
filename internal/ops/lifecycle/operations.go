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

// Package lifecycle provides utilities for managing the lifecycle of Operations.
package lifecycle

import (
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/apis/ops/v1alpha1"
)

// LatestCreateTime returns the latest creation timestamp of a set of
// Operations.
func LatestCreateTime(ops ...v1alpha1.Operation) time.Time {
	latest := time.Time{}

	for _, op := range ops {
		if t := op.GetCreationTimestamp(); t.After(latest) {
			latest = t.Time
		}
	}

	return latest
}

// LatestSucceededTransitionTime returns the latest transition timestamp for the Succeeded
// condition for a set of Operations.
func LatestSucceededTransitionTime(ops ...v1alpha1.Operation) time.Time {
	latest := time.Time{}

	for _, op := range ops {
		if t := op.GetCondition(v1alpha1.TypeSucceeded).LastTransitionTime; t.After(latest) {
			latest = t.Time
		}
	}

	return latest
}

// WithReason filters the supplied operations to only the ones that have the
// supplied Succeeded condition reason.
func WithReason(r xpv1.ConditionReason, ops ...v1alpha1.Operation) []v1alpha1.Operation {
	out := make([]v1alpha1.Operation, 0)
	for _, op := range ops {
		if op.GetCondition(v1alpha1.TypeSucceeded).Reason == r {
			out = append(out, op)
		}
	}
	return out
}

// MarkGarbage accepts a number of succeeded and failed Operations to keep. It
// returns the slice of Operations that should be deleted. It keeps the Operations
// with the most recent creation timestamps. If keepSucceeded is nil, all succeeded
// Operations will be kept. If keepFailed is nil, all failed Operations will be kept.
func MarkGarbage(keepSucceeded, keepFailed *int32, ops ...v1alpha1.Operation) []v1alpha1.Operation {
	if keepSucceeded == nil && keepFailed == nil {
		return []v1alpha1.Operation{}
	}

	del := make([]v1alpha1.Operation, 0)

	// Sort latest first.
	slices.SortFunc(ops, func(a, b v1alpha1.Operation) int {
		switch {
		case a.GetCreationTimestamp().Time.Before(b.GetCreationTimestamp().Time):
			return 1
		case a.GetCreationTimestamp().Time.Equal(b.GetCreationTimestamp().Time):
			return 0
		default:
			return -1
		}
	})

	var keptSucceeded, keptFailed int32 = 0, 0
	for _, op := range ops {
		s := op.GetCondition(v1alpha1.TypeSucceeded).Status
		switch {
		case s == corev1.ConditionTrue && keepSucceeded != nil:
			if keptSucceeded < *keepSucceeded {
				keptSucceeded++
				continue
			}
			del = append(del, op)
		case s == corev1.ConditionFalse && keepFailed != nil:
			if keptFailed < *keepFailed {
				keptFailed++
				continue
			}
			del = append(del, op)
		default:
			// Keep it.
		}
	}

	return del
}

// RunningOperationRefs returns RunningOperationRefs to the supplied names.
func RunningOperationRefs(running []string) []v1alpha1.RunningOperationRef {
	out := make([]v1alpha1.RunningOperationRef, len(running))
	for i := range running {
		out[i] = v1alpha1.RunningOperationRef{Name: running[i]}
	}
	return out
}
