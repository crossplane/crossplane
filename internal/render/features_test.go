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

package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/internal/features"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

func TestFeatures(t *testing.T) {
	type want struct {
		functionControlledDeletion bool
		err                        error
	}

	cases := map[string]struct {
		reason    string
		requested []renderv1alpha1.Feature
		want      want
	}{
		"NoFeatures": {
			reason: "With no features requested, function-controlled deletion should be disabled.",
		},
		"FunctionControlledDeletion": {
			reason:    "A requested supported feature should be enabled.",
			requested: []renderv1alpha1.Feature{renderv1alpha1.Feature_FEATURE_FUNCTION_CONTROLLED_DELETION},
			want: want{
				functionControlledDeletion: true,
			},
		},
		"UnspecifiedFeature": {
			reason:    "An unspecified feature should return an error.",
			requested: []renderv1alpha1.Feature{renderv1alpha1.Feature_FEATURE_UNSPECIFIED},
			want: want{
				err: errors.New("unspecified feature"),
			},
		},
		"UnknownFeature": {
			reason:    "A feature render doesn't know, for example from a newer client, should return an error.",
			requested: []renderv1alpha1.Feature{renderv1alpha1.Feature(42)},
			want: want{
				err: errors.New("unsupported feature 42"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ff, err := Features(tc.requested)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFeatures(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.functionControlledDeletion, ff.Enabled(features.EnableAlphaFunctionControlledDeletion)); diff != "" {
				t.Errorf("\n%s\nFeatures(...): -want function-controlled deletion, +got function-controlled deletion:\n%s", tc.reason, diff)
			}
		})
	}
}
