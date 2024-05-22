/*
Copyright 2023 The Crossplane Authors.

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

package definition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
