/*
Copyright 2021 The Crossplane Authors.

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

package resource

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewLateInitializer returns a new instance of *LateInitializer.
func NewLateInitializer() *LateInitializer {
	return &LateInitializer{}
}

// LateInitializer contains functions to late initialize two fields with varying
// types. The main purpose of LateInitializer is to be able to report whether
// anything different from the original value has been returned after all late
// initialization calls.
type LateInitializer struct {
	changed bool
}

// IsChanged reports whether the second argument is ever used in late initialization
// function calls.
func (li *LateInitializer) IsChanged() bool {
	return li.changed
}

// SetChanged marks the LateInitializer such that users can tell whether any
// of the late initialization calls returned the non-original argument.
func (li *LateInitializer) SetChanged() {
	li.changed = true
}

// LateInitializeStringPtr implements late initialization for *string.
func (li *LateInitializer) LateInitializeStringPtr(org *string, from *string) *string {
	if org != nil || from == nil {
		return org
	}
	li.SetChanged()
	return from
}

// LateInitializeInt64Ptr implements late initialization for *int64.
func (li *LateInitializer) LateInitializeInt64Ptr(org *int64, from *int64) *int64 {
	if org != nil || from == nil {
		return org
	}
	li.SetChanged()
	return from
}

// LateInitializeBoolPtr implements late initialization for *bool.
func (li *LateInitializer) LateInitializeBoolPtr(org *bool, from *bool) *bool {
	if org != nil || from == nil {
		return org
	}
	li.SetChanged()
	return from
}

// LateInitializeTimePtr implements late initialization for *metav1.Time from
// *time.Time.
func (li *LateInitializer) LateInitializeTimePtr(org *metav1.Time, from *time.Time) *metav1.Time {
	if org != nil || from == nil {
		return org
	}
	li.SetChanged()
	t := metav1.NewTime(*from)
	return &t
}
