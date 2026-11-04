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
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"

	"github.com/crossplane/crossplane/v2/internal/features"
	renderv1alpha1 "github.com/crossplane/crossplane/v2/proto/render/v1alpha1"
)

// Features returns the feature flags to render with. It enables the requested
// features, and returns an error if one isn't supported.
func Features(requested []renderv1alpha1.Feature) (*feature.Flags, error) {
	ff := &feature.Flags{}

	for _, f := range requested {
		switch f {
		case renderv1alpha1.Feature_FEATURE_FUNCTION_CONTROLLED_DELETION:
			ff.Enable(features.EnableAlphaFunctionControlledDeletion)
		case renderv1alpha1.Feature_FEATURE_UNSPECIFIED:
			return nil, errors.New("unspecified feature")
		default:
			return nil, errors.Errorf("unsupported feature %s", f)
		}
	}

	return ff, nil
}
