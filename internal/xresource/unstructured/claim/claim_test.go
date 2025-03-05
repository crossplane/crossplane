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

package claim

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/internal/xresource/unstructured/reference"
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
		reason string
		u      *Unstructured
		set    []xpv1.Condition
		get    xpv1.ConditionType
		want   xpv1.Condition
	}{
		"NewCondition": {
			reason: "It should be possible to set a condition of an empty Unstructured.",
			u:      New(),
			set:    []xpv1.Condition{xpv1.Available(), xpv1.ReconcileSuccess()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"ExistingCondition": {
			reason: "It should be possible to overwrite a condition that is already set.",
			u:      New(WithConditions(xpv1.Creating())),
			set:    []xpv1.Condition{xpv1.Available()},
			get:    xpv1.TypeReady,
			want:   xpv1.Available(),
		},
		"WeirdStatus": {
			reason: "It should not be possible to set a condition when status is not an object.",
			u: &Unstructured{unstructured.Unstructured{Object: map[string]any{
				"status": "wat",
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Condition{},
		},
		"WeirdStatusConditions": {
			reason: "Conditions should be overwritten if they are not an object.",
			u: &Unstructured{unstructured.Unstructured{Object: map[string]any{
				"status": map[string]any{
					"conditions": "wat",
				},
			}}},
			set:  []xpv1.Condition{xpv1.Available()},
			get:  xpv1.TypeReady,
			want: xpv1.Available(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetConditions(tc.set...)
			got := tc.u.GetCondition(tc.get)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nu.GetCondition(%s): -want, +got:\n%s", tc.reason, tc.get, diff)
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
	p := xpv1.UpdateManual
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv1.UpdatePolicy
		want *xpv1.UpdatePolicy
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

func TestCompositeDeletePolicy(t *testing.T) {
	p := xpv1.CompositeDeleteBackground
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv1.CompositeDeletePolicy
		want *xpv1.CompositeDeletePolicy
	}{
		"NewRef": {
			u:    New(),
			set:  &p,
			want: &p,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetCompositeDeletePolicy(tc.set)
			got := tc.u.GetCompositeDeletePolicy()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetCompositeDeletePolicy(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestResourceReference(t *testing.T) {
	ref := &reference.Composite{Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *reference.Composite
		want *reference.Composite
	}{
		"NewRef": {
			u:    New(),
			set:  ref,
			want: ref,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.u.SetResourceReference(tc.set)
			got := tc.u.GetResourceReference()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\nu.GetResourceReference(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestClaimReference(t *testing.T) {
	ref := &reference.Claim{Namespace: "ns", Name: "cool", APIVersion: "foo.com/v1", Kind: "Foo"}
	u := &Unstructured{}
	u.SetName(ref.Name)
	u.SetNamespace(ref.Namespace)
	u.SetAPIVersion(ref.APIVersion)
	u.SetKind(ref.Kind)
	got := u.GetReference()
	if diff := cmp.Diff(ref, got); diff != "" {
		t.Errorf("\nu.GetClaimReference(): -want, +got:\n%s", diff)
	}
}

func TestWriteConnectionSecretToReference(t *testing.T) {
	ref := &xpv1.LocalSecretReference{Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv1.LocalSecretReference
		want *xpv1.LocalSecretReference
	}{
		"NewRef": {
			u:    New(),
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
		j, _ := json.Marshal(t) //nolint:errchkjson // No encoding issue in practice.
		_ = json.Unmarshal(j, out)
		return out
	}

	cases := map[string]struct {
		u    *Unstructured
		set  *metav1.Time
		want *metav1.Time
	}{
		"NewTime": {
			u:    New(),
			set:  now,
			want: lores(now),
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
