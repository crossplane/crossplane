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

package composed

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

var _ client.Object = &Unstructured{}

func TestFromReference(t *testing.T) {
	ref := corev1.ObjectReference{
		APIVersion: "a/v1",
		Kind:       "k",
		Namespace:  "ns",
		Name:       "name",
	}
	cases := map[string]struct {
		ref  corev1.ObjectReference
		want *Unstructured
	}{
		"New": {
			ref: ref,
			want: &Unstructured{
				Unstructured: unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "a/v1",
						"kind":       "k",
						"metadata": map[string]any{
							"name":      "name",
							"namespace": "ns",
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := New(FromReference(tc.ref))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("New(FromReference(...): -want, +got:\n%s", diff)
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

func TestWriteConnectionSecretToReference(t *testing.T) {
	ref := &xpv1.SecretReference{Namespace: "ns", Name: "cool"}
	cases := map[string]struct {
		u    *Unstructured
		set  *xpv1.SecretReference
		want *xpv1.SecretReference
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
