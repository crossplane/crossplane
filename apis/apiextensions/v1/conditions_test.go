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

package v1

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

func TestIsSystemConditionType(t *testing.T) {
	cases := map[string]struct {
		conditionType xpv1.ConditionType
		want          bool
	}{
		"CrossplaneRuntimeSystemCondition": {
			conditionType: xpv1.TypeReady,
			want:          true,
		},
		"CrossplaneRuntimeSystemConditionSynced": {
			conditionType: xpv1.TypeSynced,
			want:          true,
		},
		"CrossplaneCircuitCondition": {
			conditionType: TypeResponsive,
			want:          true,
		},
		"CustomCondition": {
			conditionType: "DatabaseReady",
			want:          false,
		},
		"AnotherCustomCondition": {
			conditionType: "BucketReady",
			want:          false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsSystemConditionType(tc.conditionType)
			if got != tc.want {
				t.Errorf("IsSystemConditionType(%q) = %v, want %v", tc.conditionType, got, tc.want)
			}
		})
	}
}
