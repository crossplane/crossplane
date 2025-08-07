/*
Copyright 2025 The Crossplane Authors.

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

package xfn

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"k8s.io/utils/ptr"

	apiextensionsv1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
	opsv1alpha1 "github.com/crossplane/crossplane/v2/apis/ops/v1alpha1"
	fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"
)

// Ensure types satisfy RequiredResourceSelector interface.
var (
	_ RequiredResourceSelector = &apiextensionsv1.RequiredResourceSelector{}
	_ RequiredResourceSelector = &opsv1alpha1.RequiredResourceSelector{}
)

func TestToProtobufResourceSelector(t *testing.T) {
	type args struct {
		selector RequiredResourceSelector
	}
	type want struct {
		result *fnv1.ResourceSelector
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicCompositionSelector": {
			reason: "Should convert basic composition API selector to protobuf selector",
			args: args{
				selector: &apiextensionsv1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "ConfigMap",
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
				},
			},
		},
		"BasicOperationSelector": {
			reason: "Should convert basic operation API selector to protobuf selector",
			args: args{
				selector: &opsv1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Pod",
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
				},
			},
		},
		"CompositionSelectorWithName": {
			reason: "Should convert composition selector with specific name",
			args: args{
				selector: &apiextensionsv1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Name:            ptr.To("test-configmap"),
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "test-configmap",
					},
				},
			},
		},
		"OperationSelectorWithName": {
			reason: "Should convert operation selector with specific name",
			args: args{
				selector: &opsv1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Secret",
					Name:            ptr.To("test-secret"),
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Secret",
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "test-secret",
					},
				},
			},
		},
		"CompositionSelectorWithLabels": {
			reason: "Should convert composition selector with match labels",
			args: args{
				selector: &apiextensionsv1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					MatchLabels:     map[string]string{"app": "test"},
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{"app": "test"},
						},
					},
				},
			},
		},
		"OperationSelectorWithLabels": {
			reason: "Should convert operation selector with match labels",
			args: args{
				selector: &opsv1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Pod",
					MatchLabels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Pod",
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"app": "test",
								"env": "prod",
							},
						},
					},
				},
			},
		},
		"CompositionNamespacedSelector": {
			reason: "Should convert composition namespaced selector",
			args: args{
				selector: &apiextensionsv1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "ConfigMap",
					Namespace:       ptr.To("test-namespace"),
					Name:            ptr.To("test-name"),
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  ptr.To("test-namespace"),
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "test-name",
					},
				},
			},
		},
		"OperationSelectorWithNameAndNamespace": {
			reason: "Should convert operation selector with both name and namespace",
			args: args{
				selector: &opsv1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "v1",
					Kind:            "Secret",
					Name:            ptr.To("my-secret"),
					Namespace:       ptr.To("kube-system"),
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "v1",
					Kind:       "Secret",
					Namespace:  ptr.To("kube-system"),
					Match: &fnv1.ResourceSelector_MatchName{
						MatchName: "my-secret",
					},
				},
			},
		},
		"OperationSelectorWithLabelsAndNamespace": {
			reason: "Should convert operation selector with labels and namespace",
			args: args{
				selector: &opsv1alpha1.RequiredResourceSelector{
					RequirementName: "test-req",
					APIVersion:      "apps/v1",
					Kind:            "Deployment",
					MatchLabels: map[string]string{
						"tier": "frontend",
					},
					Namespace: ptr.To("production"),
				},
			},
			want: want{
				result: &fnv1.ResourceSelector{
					ApiVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  ptr.To("production"),
					Match: &fnv1.ResourceSelector_MatchLabels{
						MatchLabels: &fnv1.MatchLabels{
							Labels: map[string]string{
								"tier": "frontend",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := ToProtobufResourceSelector(tc.args.selector)
			if diff := cmp.Diff(tc.want.result, result, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nToProtobufResourceSelector(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
