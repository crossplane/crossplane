/*
Copyright 2019 The Crossplane Authors.

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

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func TestConditionEqual(t *testing.T) {
	cases := map[string]struct {
		a    Condition
		b    Condition
		want bool
	}{
		"IdenticalIgnoringTimestamp": {
			a:    Condition{Type: TypeReady, LastTransitionTime: metav1.Now()},
			b:    Condition{Type: TypeReady, LastTransitionTime: metav1.Now()},
			want: true,
		},
		"DifferentType": {
			a:    Condition{Type: TypeReady},
			b:    Condition{Type: TypeSynced},
			want: false,
		},
		"DifferentStatus": {
			a:    Condition{Status: corev1.ConditionTrue},
			b:    Condition{Status: corev1.ConditionFalse},
			want: false,
		},
		"DifferentReason": {
			a:    Condition{Reason: ReasonCreating},
			b:    Condition{Reason: ReasonDeleting},
			want: false,
		},
		"DifferentMessage": {
			a:    Condition{Message: "cool"},
			b:    Condition{Message: "uncool"},
			want: false,
		},
		"CheckReconcilePaused": {
			a: ReconcilePaused(),
			b: Condition{
				Type:   TypeSynced,
				Status: corev1.ConditionFalse,
				Reason: ReasonReconcilePaused,
			},
			want: true,
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
	cases := map[string]struct {
		a    *ConditionedStatus
		b    *ConditionedStatus
		want bool
	}{
		"Identical": {
			a:    NewConditionedStatus(Available(), ReconcileSuccess()),
			b:    NewConditionedStatus(Available(), ReconcileSuccess()),
			want: true,
		},
		"IdenticalExceptOrder": {
			a:    NewConditionedStatus(Unavailable(), ReconcileSuccess()),
			b:    NewConditionedStatus(ReconcileSuccess(), Unavailable()),
			want: true,
		},
		"DifferentLength": {
			a:    NewConditionedStatus(Available(), ReconcileSuccess()),
			b:    NewConditionedStatus(ReconcileSuccess()),
			want: false,
		},
		"DifferentCondition": {
			a:    NewConditionedStatus(Creating(), ReconcileSuccess()),
			b:    NewConditionedStatus(Creating(), ReconcileError(errors.New("boom"))),
			want: false,
		},
		"AIsNil": {
			a:    nil,
			b:    NewConditionedStatus(Deleting(), ReconcileSuccess()),
			want: false,
		},
		"BIsNil": {
			a:    NewConditionedStatus(Available(), ReconcileSuccess()),
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

func TestSetConditions(t *testing.T) {
	cases := map[string]struct {
		cs   *ConditionedStatus
		c    []Condition
		want *ConditionedStatus
	}{
		"TypeIsIdentical": {
			cs:   NewConditionedStatus(Available()),
			c:    []Condition{Available()},
			want: NewConditionedStatus(Available()),
		},
		"TypeIsDifferent": {
			cs:   NewConditionedStatus(Creating()),
			c:    []Condition{Available()},
			want: NewConditionedStatus(Available()),
		},
		"TypeDoesNotExist": {
			cs:   NewConditionedStatus(ReconcileSuccess()),
			c:    []Condition{Available()},
			want: NewConditionedStatus(ReconcileSuccess(), Available()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.cs.SetConditions(tc.c...)

			got := tc.cs
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.SetConditions(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetCondition(t *testing.T) {
	cases := map[string]struct {
		cs   *ConditionedStatus
		t    ConditionType
		want Condition
	}{
		"ConditionExists": {
			cs:   NewConditionedStatus(Available()),
			t:    TypeReady,
			want: Available(),
		},
		"ConditionDoesNotExist": {
			cs: NewConditionedStatus(Available()),
			t:  TypeSynced,
			want: Condition{
				Type:   TypeSynced,
				Status: corev1.ConditionUnknown,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.cs.GetCondition(tc.t)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("tc.cs.GetConditions(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditionWithMessage(t *testing.T) {
	testMsg := "Something went wrong on cloud side"
	cases := map[string]struct {
		c    Condition
		msg  string
		want Condition
	}{
		"MessageAdded": {
			c:    Condition{Type: TypeReady, Reason: ReasonUnavailable},
			msg:  testMsg,
			want: Condition{Type: TypeReady, Reason: ReasonUnavailable, Message: testMsg},
		},
		"MessageChanged": {
			c:    Condition{Type: TypeReady, Reason: ReasonUnavailable, Message: "Some other message"},
			msg:  testMsg,
			want: Condition{Type: TypeReady, Reason: ReasonUnavailable, Message: testMsg},
		},
		"MessageCleared": {
			c:    Condition{Type: TypeReady, Reason: ReasonUnavailable, Message: testMsg},
			msg:  "",
			want: Condition{Type: TypeReady, Reason: ReasonUnavailable},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.c.WithMessage(tc.msg)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("a.Equal(b): -want, +got:\n%s", diff)
			}
		})
	}
}
