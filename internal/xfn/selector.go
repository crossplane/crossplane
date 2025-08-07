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

import fnv1 "github.com/crossplane/crossplane/v2/proto/fn/v1"

// RequiredResourceSelector is a common interface for required resource selectors
// that can be converted to protobuf ResourceSelector.
type RequiredResourceSelector interface { //nolint:interfacebloat // This interface aggregates common fields from two API types.
	GetRequirementName() string
	GetAPIVersion() string
	GetKind() string
	GetName() *string
	GetMatchLabels() map[string]string
	GetNamespace() *string
}

// ToProtobufResourceSelector converts a required resource selector to a
// protobuf ResourceSelector.
func ToProtobufResourceSelector(r RequiredResourceSelector) *fnv1.ResourceSelector {
	selector := &fnv1.ResourceSelector{
		ApiVersion: r.GetAPIVersion(),
		Kind:       r.GetKind(),
		Namespace:  r.GetNamespace(),
	}

	// You can only set one of name or matchLabels.
	if r.GetName() != nil {
		selector.Match = &fnv1.ResourceSelector_MatchName{
			MatchName: *r.GetName(),
		}
		return selector
	}

	if len(r.GetMatchLabels()) > 0 {
		selector.Match = &fnv1.ResourceSelector_MatchLabels{
			MatchLabels: &fnv1.MatchLabels{
				Labels: r.GetMatchLabels(),
			},
		}
	}

	return selector
}
