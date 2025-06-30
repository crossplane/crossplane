/*
Copyright 2024 The Crossplane Authors.

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

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsCompositeResourceCRD(t *testing.T) {
	cases := map[string]struct {
		obj  runtime.Object
		want bool
	}{
		"NotCRD": {
			want: false,
		},
		"XRD": {
			obj:  &v2.CompositeResourceDefinition{},
			want: false,
		},
		"ClaimCRD": {
			obj: &extv1.CustomResourceDefinition{
				Spec: extv1.CustomResourceDefinitionSpec{
					Names: extv1.CustomResourceDefinitionNames{
						Categories: []string{
							"claim",
						},
					},
				},
			},
			want: false,
		},
		"CompositeCRD": {
			obj: &extv1.CustomResourceDefinition{
				Spec: extv1.CustomResourceDefinitionSpec{
					Names: extv1.CustomResourceDefinitionNames{
						Categories: []string{
							"composite",
						},
					},
				},
			},
			want: true,
		},
		"OtherCRD": {
			obj: &extv1.CustomResourceDefinition{
				Spec: extv1.CustomResourceDefinitionSpec{
					Names: extv1.CustomResourceDefinitionNames{
						Categories: []string{},
					},
				},
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsCompositeResourceCRD()(tc.obj)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nIsCompositeResourceCRD(...): -want, +got:\n%s", name, diff)
			}
		})
	}
}
