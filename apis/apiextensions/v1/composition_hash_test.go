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
	newComp := func() *Composition {
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
			},
		}
	}
	originalHash := "600b2cf44398dd5ac5b89f7379ab13ef737ff4418c937b988d53762aca8ef84b"
	if diff := cmp.Diff(originalHash, newComp().Hash()); diff != "" {
		t.Errorf("\ntest fixture hash is incorrect\nHash(): -want, +got:\n%s", diff)
	}

	cases := map[string]struct {
		reason string
		mutate func(*Composition)
		want   string
	}{
		"RevisionHistoryLimitDefault": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to its default.",
			mutate: func(c *Composition) {
				c.Spec.RevisionHistoryLimit = new(int64(1))
			},
			want: originalHash,
		},
		"RevisionHistoryLimitZero": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to zero.",
			mutate: func(c *Composition) {
				c.Spec.RevisionHistoryLimit = new(int64(0))
			},
			want: originalHash,
		},
		"RevisionHistoryLimitNonDefault": {
			reason: "The hash shouldn't change when RevisionHistoryLimit is set to a non-default value.",
			mutate: func(c *Composition) {
				c.Spec.RevisionHistoryLimit = new(int64(10))
			},
			want: originalHash,
		},
		"LabelChange": {
			reason: "The hash should change when labels change.",
			mutate: func(c *Composition) {
				c.Labels["new"] = "yes"
			},
			want: "0428779dfa246937272cfb0765b59399f4649d76414983a0ceb6f8c609c58563",
		},
		"AnnotationChange": {
			reason: "The hash should change when annotations change.",
			mutate: func(c *Composition) {
				c.Annotations["new"] = "yes"
			},
			want: "2517f0a31085d41b6ca0585db31c8724eff0dd15ef279fb083277f87cb67c8d8",
		},
		"SpecChange": {
			reason: "The hash should change when spec fields other than revisionHistoryLimit change.",
			mutate: func(c *Composition) {
				c.Spec.Pipeline = append(c.Spec.Pipeline, PipelineStep{
					Step: "auto-ready",
					FunctionRef: FunctionReference{
						Name: "crossplane-contrib-function-auto-ready",
					},
				})
			},
			want: "f79f0596108d84f77723762d205626ea056b83b1c013c14ed36814c504eaa19b",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			comp := newComp()
			tc.mutate(comp)
			before := comp.DeepCopy()
			got := comp.Hash()

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nHash(): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(before, comp); diff != "" {
				t.Errorf("\nHash() mutated the composition: -before +after:\n%s", diff)
			}
		})
	}
}
