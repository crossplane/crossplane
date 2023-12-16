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

package predicate

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestStatusChangedPredicate(t *testing.T) {
	tests := map[string]struct {
		reason string
		event  event.UpdateEvent
		want   bool
	}{
		"NoOldObject": {
			reason: "update event is filtered out when the old object not exists",
			event: event.UpdateEvent{
				ObjectNew: &unstructured.Unstructured{},
			},
			want: false,
		},
		"NoNewObject": {
			reason: "update event is filtered out when the new object not exists",
			event: event.UpdateEvent{
				ObjectOld: &unstructured.Unstructured{},
			},
			want: false,
		},
		"NoOldObjectStatus": {
			reason: "update event not filtered out when old status does not exist but the new status exists",
			event: event.UpdateEvent{
				ObjectOld: &unstructured.Unstructured{},
				ObjectNew: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{},
					},
				},
			},
			want: true,
		},
		"NoNewObjectStatus": {
			reason: "update event not filtered out when old status exists but the new status does not",
			event: event.UpdateEvent{
				ObjectNew: &unstructured.Unstructured{},
				ObjectOld: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{},
					},
				},
			},
			want: true,
		},
		"DifferentStatuses": {
			reason: "update event not filtered out when statuses are different",
			event: event.UpdateEvent{
				ObjectNew: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{
							"bar": "foo",
						},
					},
				},
				ObjectOld: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{
							"foo": "bar",
						},
					},
				},
			},
			want: true,
		},
		"EqualStatuses": {
			reason: "update event filtered out when statuses are equal",
			event: event.UpdateEvent{
				ObjectNew: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{
							"foo": "bar",
						},
					},
				},
				ObjectOld: &unstructured.Unstructured{
					Object: map[string]any{
						"status": map[string]any{
							"foo": "bar",
						},
					},
				},
			},
			want: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			p := StatusChangedPredicate{}
			if got := p.Update(tc.event); got != tc.want {
				t.Errorf("%s: Update() = %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}
