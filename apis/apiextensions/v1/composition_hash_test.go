/*
Copyright 2026 The Crossplane Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompositionHash(t *testing.T) {
	newComp := func(limit *int64) *Composition {
		return &Composition{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      map[string]string{"channel": "dev"},
				Annotations: map[string]string{"cool": "very"},
			},
			Spec: CompositionSpec{
				CompositeTypeRef: TypeReference{APIVersion: "example.org/v1", Kind: "XExample"},
				Mode:             CompositionModePipeline,
				Pipeline: []PipelineStep{{
					Step:        "compose",
					FunctionRef: FunctionReference{Name: "function-example"},
				}},
				RevisionHistoryLimit: limit,
			},
		}
	}

	golden := "600b2cf44398dd5ac5b89f7379ab13ef737ff4418c937b988d53762aca8ef84b"

	cases := map[string]struct {
		reason string
		comp   *Composition
		want   string
	}{
		"RevisionHistoryLimitUnset": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is unset.",
			comp:   newComp(nil),
			want:   golden,
		},
		"RevisionHistoryLimitDefault": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to its default.",
			comp:   newComp(new(int64(1))),
			want:   golden,
		},
		"RevisionHistoryLimitZero": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to zero.",
			comp:   newComp(new(int64(0))),
			want:   golden,
		},
		"RevisionHistoryLimitNonDefault": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to a non-default value.",
			comp:   newComp(new(int64(10))),
			want:   golden,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			before := tc.comp.DeepCopy()

			got := tc.comp.Hash()
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nHash(): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(before, tc.comp); diff != "" {
				t.Errorf("\nHash() mutated the composition: -before +after:\n%s", diff)
			}
		})
	}
}
