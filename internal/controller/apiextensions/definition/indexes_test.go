// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package definition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIndexCompositeResourceRefGVKs(t *testing.T) {
	type args struct {
		object client.Object
	}
	tests := map[string]struct {
		args args
		want []string
	}{
		"Nil":             {args: args{object: nil}, want: nil},
		"NotUnstructured": {args: args{object: &corev1.Pod{}}, want: nil},
		"NoRefs": {args: args{object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
		}}, want: []string{}},
		"References": {args: args{object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRefs": []interface{}{
						map[string]interface{}{
							"apiVersion": "nop.crossplane.io/v1alpha1",
							"kind":       "NopResource",
							"name":       "mr",
						},
						map[string]interface{}{
							"apiVersion": "nop.example.org/v1alpha1",
							"kind":       "NopResource",
							"name":       "xr",
						},
					},
				},
			},
		}}, want: []string{"nop.crossplane.io/v1alpha1, Kind=NopResource", "nop.example.org/v1alpha1, Kind=NopResource"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IndexCompositeResourceRefGVKs(tc.args.object)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIndexCompositeResourceRefGVKs(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}

func TestIndexCompositeResourcesRefs(t *testing.T) {
	type args struct {
		object client.Object
	}
	tests := map[string]struct {
		args args
		want []string
	}{
		"Nil":             {args: args{object: nil}, want: nil},
		"NotUnstructured": {args: args{object: &corev1.Pod{}}, want: nil},
		"NoRefs": {args: args{object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
		}}, want: []string{}},
		"References": {args: args{object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRefs": []interface{}{
						map[string]interface{}{
							"apiVersion": "nop.crossplane.io/v1alpha1",
							"kind":       "NopResource",
							"name":       "mr",
						},
						map[string]interface{}{
							"apiVersion": "nop.example.org/v1alpha1",
							"kind":       "NopResource",
							"name":       "xr",
						},
						map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"name":       "cm",
							"namespace":  "ns",
						},
					},
				},
			},
		}}, want: []string{"mr..NopResource.nop.crossplane.io/v1alpha1", "xr..NopResource.nop.example.org/v1alpha1", "cm.ns.ConfigMap.v1"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := IndexCompositeResourcesRefs(tc.args.object)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIndexCompositeResourcesRefs(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}
