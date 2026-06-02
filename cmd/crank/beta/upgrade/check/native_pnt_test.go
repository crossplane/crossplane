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

package check

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestNativePatchAndTransformRun(t *testing.T) {
	type want struct {
		findings []Finding
		err      error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"ListError": {
			reason: "A List failure surfaces as a wrapped error and no findings.",
			client: &test.MockClient{MockList: test.NewMockListFn(errBoom)},
			want:   want{err: cmpopts.AnyError},
		},
		"PipelineModeClean": {
			reason: "An explicit Pipeline-mode Composition with no native fields produces no findings.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositionList).Items = []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "clean"},
					Spec: apiextensionsv1.CompositionSpec{
						Mode: ptr.To(apiextensionsv1.CompositionModePipeline),
					},
				}}
				return nil
			})},
			want: want{findings: nil},
		},
		"NilModeDefaultsToResources": {
			reason: "A nil Mode defaults to Resources, so it is flagged at .spec.mode.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositionList).Items = []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "nilmode"},
				}}
				return nil
			})},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "nilmode"}, FieldPath: ".spec.mode"}}},
		},
		"AllNativeFields": {
			reason: "A Resources-mode Composition with resources and patchSets is flagged on all three fields.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositionList).Items = []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "native"},
					Spec: apiextensionsv1.CompositionSpec{
						Mode:      ptr.To(apiextensionsv1.CompositionModeResources),
						Resources: make([]apiextensionsv1.ComposedTemplate, 2),
						PatchSets: make([]apiextensionsv1.PatchSet, 1),
					},
				}}
				return nil
			})},
			want: want{findings: []Finding{
				{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "native"}, FieldPath: ".spec.mode"},
				{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "native"}, FieldPath: ".spec.resources"},
				{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "native"}, FieldPath: ".spec.patchSets"},
			}},
		},
		"PipelineModeWithLeftoverResources": {
			reason: "Resources/patchSets set under Pipeline mode are still flagged; only .spec.mode is clean.",
			client: &test.MockClient{MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
				o.(*apiextensionsv1.CompositionList).Items = []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "leftover"},
					Spec: apiextensionsv1.CompositionSpec{
						Mode:      ptr.To(apiextensionsv1.CompositionModePipeline),
						Resources: make([]apiextensionsv1.ComposedTemplate, 1),
					},
				}}
				return nil
			})},
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "leftover"}, FieldPath: ".spec.resources"}}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &NativePatchAndTransform{Client: tc.client}
			got, err := c.Run(context.Background())
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.findings, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRun(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
