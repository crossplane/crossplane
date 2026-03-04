/*
Copyright 2020 The Crossplane Authors.

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

package composite

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var _ client.Object = &Unstructured{}

func TestWithGroupVersionKind(t *testing.T) {
	gvk := schema.GroupVersionKind{
		Group:   "g",
		Version: "v1",
		Kind:    "k",
	}
	cases := map[string]struct {
		gvk  schema.GroupVersionKind
		want *Unstructured
	}{
		"New": {
			gvk: gvk,
			want: &Unstructured{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "g/v1",
						"kind":       "k",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := New(WithGroupVersionKind(tc.gvk))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("New(WithGroupVersionKind(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConditions(t *testing.T) {
	cases := map[string]struct {
		reason  string
		u       *Unstructured
		set     []xpv2.Condition
		get     xpv2.ConditionType
		want    xpv2.Condition
		wantAll []xpv2.Condition
	}{
		"NewCondition": {
			reason:  "It should be possible to set a condition of an empty Unstructured.",
			u:       New(),
			set:     []xpv2.Condition{xpv2.Available(), xpv2.ReconcileSuccess()},
			get:     xpv2.TypeReady,
			want:    xpv2.Available(),
			wantAll: []xpv2.Condition{xpv2.Available(), xpv2.ReconcileSuccess()},
		},
		"ExistingCondition": {
			reason:  "It should be possible to overwrite a condition that is already set.",
			u:       New(WithConditions(xpv2.Creating())),
			set:     []xpv2.Condition{xpv2.Available()},
			get:     xpv2.TypeReady,
			want:    xpv2.Available(),
			wantAll: []xpv2.Condition{xpv2.Available()},
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Unstructured{Unstructured: unstructured.Unstructured{Object: map[string]any{
				"status": "wat",
			}}},
			set:     []xpv2.Condition{xpv2.Available()},
			get:     xpv2.TypeReady,
			want:    xpv2.Condition{},
			wantAll: nil,
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &Unstructured{Unstructured: unstructured.Unstructured{Object: map[string]any{
				"status": map[string]any{
					"conditions": "wat",
				},
			}}},
			set:     []xpv2.Condition{xpv2.Available()},
			get:     xpv2.TypeReady,
			want:    xpv2.Available(),
			wantAll: []xpv2.Condition{xpv2.Available()},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConditions(tc.set...)

			got := tc.u.GetCondition(tc.get)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nu.GetCondition(%s): -want, +got:\n%s", tc.reason, tc.get, diff)
			}

			gotAll := tc.u.GetConditions()
			if diff := cmp.Diff(tc.wantAll, gotAll); diff != "" {
				t.Errorf("\n%s\nu.GetConditions(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClaimConditionTypes(t *testing.T) {
	cases := map[string]struct {
		reason  string
		u       *Unstructured
		set     []xpv2.ConditionType
		want    []xpv2.ConditionType
		wantErr error
	}{
		"CannotSetSystemConditionTypes": {
			reason: "Claim conditions API should fail to set conditions if a system condition is detected.",
			u:      New(WithSchema(SchemaLegacy)),
			set: []xpv2.ConditionType{
				xpv2.ConditionType("DatabaseReady"),
				xpv2.ConditionType("NetworkReady"),
				// system condition
				xpv2.ConditionType("Ready"),
			},
			want:    []xpv2.ConditionType{},
			wantErr: errors.New("cannot set system condition Ready as a claim condition"),
		},
		"SetSingleCustomConditionType": {
			reason: "Claim condition API should work with a single custom condition type.",
			u:      New(WithSchema(SchemaLegacy)),
			set:    []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
			want:   []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
		},
		"SetMultipleCustomConditionTypes": {
			reason: "Claim condition API should work with multiple custom condition types.",
			u:      New(WithSchema(SchemaLegacy)),
			set:    []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady"), xpv2.ConditionType("NetworkReady")},
			want:   []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady"), xpv2.ConditionType("NetworkReady")},
		},
		"SetMultipleOfTheSameCustomConditionTypes": {
			reason: "Claim condition API not add more than one of the same condition.",
			u:      New(WithSchema(SchemaLegacy)),
			set:    []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady"), xpv2.ConditionType("DatabaseReady")},
			want:   []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Unstructured{
				Unstructured: unstructured.Unstructured{Object: map[string]any{
					"status": "wat",
				}},
				Schema: SchemaLegacy,
			},
			set:  []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
			want: []xpv2.ConditionType{},
		},
		"WeirdStatusClaimConditionTypes": {
			reason: "Claim conditions should be overwritten if they are not an object.",
			u: &Unstructured{
				Unstructured: unstructured.Unstructured{Object: map[string]any{
					"status": map[string]any{
						"claimConditionTypes": "wat",
					},
				}},
				Schema: SchemaLegacy,
			},
			set:  []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
			want: []xpv2.ConditionType{xpv2.ConditionType("DatabaseReady")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErr := tc.u.SetClaimConditionTypes(tc.set...)
			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nu.SetClaimConditionTypes(): -want, +got:\n%s", tc.reason, diff)
			}

			got := tc.u.GetClaimConditionTypes()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nu.GetClaimConditionTypes(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionSelector(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}}
	cases := map[string]struct {
		u    *Unstructured
		set  *metav1.LabelSelector
		want *metav1.LabelSelector
	}{
		"NewSel": {
			u:    New(),
			set:  sel,
			want: sel,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionSelector(tc.set)

			got := tc.u.GetCompositionSelector()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionSelector(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCompositionReference(t *testing.T) {
	ref := &corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *corev1.ObjectReference
		want *corev1.ObjectReference
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionReference(tc.set)

			got := tc.u.GetCompositionReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCompositionRevisionReference(t *testing.T) {
	ref := &corev1.LocalObjectReference{Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *corev1.LocalObjectReference
		want *corev1.LocalObjectReference
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionRevisionReference(tc.set)

			got := tc.u.GetCompositionRevisionReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionRevisionReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCompositionRevisionSelector(t *testing.T) {
	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"cool": "very"}}
	cases := map[string]struct {
		u    *Unstructured
		set  *metav1.LabelSelector
		want *metav1.LabelSelector
	}{
		"NewRef": {
			u:    New(),
			set:  sel,
			want: sel,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionRevisionSelector(tc.set)

			got := tc.u.GetCompositionRevisionSelector()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionRevisionSelector(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCompositionUpdatePolicy(t *testing.T) {
	p := xpv2.UpdateManual
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv2.UpdatePolicy
		want *xpv2.UpdatePolicy
	}{
		"NewRef": {
			u:    New(),
			set:  &p,
			want: &p,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositionUpdatePolicy(tc.set)

			got := tc.u.GetCompositionUpdatePolicy()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositionUpdatePolicy(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClaimReference(t *testing.T) {
	ref := &reference.Claim{Namespace: "ns", Name: "cool", APIVersion: "acme.com/v1", Kind: "Foo"}
	cases := map[string]struct {
		u    *Unstructured
		set  *reference.Claim
		want *reference.Claim
	}{
		"NewRef": {
			u:    New(WithSchema(SchemaLegacy)),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetClaimReference(tc.set)

			got := tc.u.GetClaimReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetClaimReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestResourceReferences(t *testing.T) {
	ref := corev1.ObjectReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  []corev1.ObjectReference
		want []corev1.ObjectReference
	}{
		"NewRef": {
			u:    New(),
			set:  []corev1.ObjectReference{ref},
			want: []corev1.ObjectReference{ref},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetResourceReferences(tc.set)

			got := tc.u.GetResourceReferences()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetResourceReferences(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWriteConnectionSecretToReference(t *testing.T) {
	ref := &xpv2.SecretReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv2.SecretReference
		want *xpv2.SecretReference
	}{
		"NewRef": {
			u:    New(WithSchema(SchemaLegacy)),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetWriteConnectionSecretToReference(tc.set)

			got := tc.u.GetWriteConnectionSecretToReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetWriteConnectionSecretToReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnectionDetailsLastPublishedTime(t *testing.T) {
	now := &metav1.Time{Time: time.Now()}

	// The timestamp loses a little resolution when round-tripped through JSON
	// encoding.
	lores := func(t *metav1.Time) *metav1.Time {
		out := &metav1.Time{}
		j, _ := json.Marshal(t) //nolint:errchkjson // No encoding error in practice.
		_ = json.Unmarshal(j, out)

		return out
	}

	cases := map[string]struct {
		u    *Unstructured
		set  *metav1.Time
		want *metav1.Time
	}{
		"NewTimeLegacy": {
			u:    New(WithSchema(SchemaLegacy)),
			set:  now,
			want: lores(now),
		},
		"NewTimeModern": {
			u:    New(WithSchema(SchemaModern)),
			set:  now,
			want: nil, // modern schema doesn't support connection details
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConnectionDetailsLastPublishedTime(tc.set)

			got := tc.u.GetConnectionDetailsLastPublishedTime()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetConnectionDetailsLastPublishedTime(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestObservedGeneration(t *testing.T) {
	cases := map[string]struct {
		u    *Unstructured
		want int64
	}{
		"Set": {
			u: New(func(u *Unstructured) {
				u.SetObservedGeneration(123)
			}),
			want: 123,
		},
		"NotFound": {
			u: New(),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.u.GetObservedGeneration()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetObservedGeneration(): -want, +got:\n%s", diff)
			}
		})
	}
}
