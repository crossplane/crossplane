/*
Copyright 2018 The Crossplane Authors.

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionEqual(t *testing.T) {
	cases := map[string]struct {
		a    Condition
		b    Condition
		want bool
	}{
		"IdenticalIgnoringTimestamp": {
			a:    Condition{Type: Ready, Status: corev1.ConditionTrue, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			b:    Condition{Type: Ready, Status: corev1.ConditionTrue, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			want: true,
		},
		"DifferentType": {
			a:    Condition{Type: Ready, Status: corev1.ConditionTrue, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			b:    Condition{Type: Synced, Status: corev1.ConditionTrue, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			want: false,
		},
		"DifferentStatus": {
			a:    Condition{Type: Ready, Status: corev1.ConditionTrue, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			b:    Condition{Type: Ready, Status: corev1.ConditionFalse, Reason: Available, Message: "cool", LastTransitionTime: metav1.Now()},
			want: false,
		},
		"DifferentReason": {
			a:    Condition{Type: Ready, Status: corev1.ConditionFalse, Reason: Creating, Message: "cool", LastTransitionTime: metav1.Now()},
			b:    Condition{Type: Ready, Status: corev1.ConditionFalse, Reason: Deleting, Message: "cool", LastTransitionTime: metav1.Now()},
			want: false,
		},
		"DifferentMessage": {
			a:    Condition{Type: Ready, Status: corev1.ConditionFalse, Reason: Creating, Message: "cool", LastTransitionTime: metav1.Now()},
			b:    Condition{Type: Ready, Status: corev1.ConditionFalse, Reason: Creating, Message: "uncool", LastTransitionTime: metav1.Now()},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.a.Equal(tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("a.Equal(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditionedStatusEqual(t *testing.T) {
	ready := Condition{
		Type:               Ready,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}
	synced := Condition{
		Type:               Synced,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}

	cases := map[string]struct {
		a    *ConditionedStatus
		b    *ConditionedStatus
		want bool
	}{
		"Identical": {
			a:    &ConditionedStatus{Conditions: []Condition{ready, synced}},
			b:    &ConditionedStatus{Conditions: []Condition{ready, synced}},
			want: true,
		},
		"IdenticalExceptOrder": {
			a:    &ConditionedStatus{Conditions: []Condition{ready, synced}},
			b:    &ConditionedStatus{Conditions: []Condition{synced, ready}},
			want: true,
		},
		"DifferentLength": {
			a:    &ConditionedStatus{Conditions: []Condition{ready, synced}},
			b:    &ConditionedStatus{Conditions: []Condition{synced}},
			want: false,
		},
		"DifferentCondition": {
			a: &ConditionedStatus{Conditions: []Condition{ready, synced}},
			b: &ConditionedStatus{Conditions: []Condition{
				ready,
				{
					Type:               Synced,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Message:            "I'm different!",
				},
			}},
			want: false,
		},
		"AIsNil": {
			a:    nil,
			b:    &ConditionedStatus{Conditions: []Condition{synced}},
			want: false,
		},
		"BIsNil": {
			a:    &ConditionedStatus{Conditions: []Condition{synced}},
			b:    nil,
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.a.Equal(tc.b)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("a.Equal(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSet(t *testing.T) {
	ready := Condition{
		Type:               Ready,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}
	synced := Condition{
		Type:               Synced,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}

	cases := map[string]struct {
		cs   *ConditionedStatus
		c    []Condition
		want *ConditionedStatus
	}{
		"TypeDoesNotExist": {
			cs:   &ConditionedStatus{Conditions: []Condition{synced}},
			c:    []Condition{ready},
			want: &ConditionedStatus{Conditions: []Condition{ready, synced}},
		},
		"TypeIsIdentical": {
			cs:   &ConditionedStatus{Conditions: []Condition{ready}},
			c:    []Condition{ready},
			want: &ConditionedStatus{Conditions: []Condition{ready}},
		},
		"TypeIsDifferent": {
			cs: &ConditionedStatus{Conditions: []Condition{{
				Type:   Ready,
				Reason: ConditionReason("imdifferent!"),
			}}},
			c:    []Condition{ready},
			want: &ConditionedStatus{Conditions: []Condition{ready}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.Set(tc.c...)

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.Set(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSetCreating(t *testing.T) {
	creating := NewConditionedStatus()
	creating.SetCreating()

	errored := NewConditionedStatus()
	errored.SetCreating()
	errored.ReconcileError(errors.New("boom"))

	cases := map[string]struct {
		cs   *ConditionedStatus
		want *ConditionedStatus
	}{
		"CurrentlyUnknown": {
			cs:   NewConditionedStatus(),
			want: creating,
		},
		"CurrentlyCreating": {
			cs:   creating,
			want: creating,
		},
		"CurrentlyReconcileError": {
			cs:   errored,
			want: creating,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetCreating()

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.SetCreating(): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestSetDeleting(t *testing.T) {
	deleting := NewConditionedStatus()
	deleting.SetDeleting()

	creating := NewConditionedStatus()
	creating.SetCreating()

	available := NewConditionedStatus()
	available.SetAvailable()

	errored := NewConditionedStatus()
	errored.SetDeleting()
	errored.ReconcileError(errors.New("boom"))

	cases := map[string]struct {
		cs   *ConditionedStatus
		want *ConditionedStatus
	}{
		"CurrentlyCreating": {
			cs:   creating,
			want: deleting,
		},
		"CurrentlyDeleting": {
			cs:   deleting,
			want: deleting,
		},
		"CurrentlyAvailable": {
			cs:   available,
			want: deleting,
		},
		"CurrentlyReconcileError": {
			cs:   errored,
			want: deleting,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetDeleting()

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.SetDeleting(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSetAvailable(t *testing.T) {
	creating := NewConditionedStatus()
	creating.SetCreating()

	available := NewConditionedStatus()
	available.SetAvailable()

	errored := NewConditionedStatus()
	errored.SetAvailable()
	errored.ReconcileError(errors.New("boom"))

	cases := map[string]struct {
		cs   *ConditionedStatus
		want *ConditionedStatus
	}{
		"CurrentlyCreating": {
			cs:   creating,
			want: available,
		},
		"CurrentlyAvailable": {
			cs:   available,
			want: available,
		},
		"CurrentlyReconcileError": {
			cs:   errored,
			want: available,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetAvailable()

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.SetAvailable(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSetUnavailable(t *testing.T) {
	available := NewConditionedStatus()
	available.SetAvailable()

	unavailable := NewConditionedStatus()
	unavailable.SetUnavailable()

	errored := NewConditionedStatus()
	errored.SetAvailable()
	errored.ReconcileError(errors.New("boom"))

	cases := map[string]struct {
		cs   *ConditionedStatus
		want *ConditionedStatus
	}{
		"CurrentlyAvailable": {
			cs:   available,
			want: unavailable,
		},
		"CurrentlyUnavailable": {
			cs:   unavailable,
			want: unavailable,
		},
		"CurrentlyReconcileError": {
			cs:   errored,
			want: unavailable,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetUnavailable()

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.SetUnavailable(): -want, +got:\n%s", diff)
			}
		})
	}
}
func TestReconcileError(t *testing.T) {
	err := errors.New("boom")

	available := NewConditionedStatus()
	available.SetAvailable()

	errored := NewConditionedStatus()
	errored.SetAvailable()
	errored.ReconcileError(errors.New("boom"))

	cases := map[string]struct {
		cs   *ConditionedStatus
		want *ConditionedStatus
	}{
		"CurrentlyAvailable": {
			cs:   available,
			want: errored,
		},
		"CurrentlyReconcileError": {
			cs:   errored,
			want: errored,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.ReconcileError(err)

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.ReconcileError(...): -want, +got:\n%s", diff)
			}
		})
	}
}
