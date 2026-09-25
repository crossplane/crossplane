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

package discovery

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

func TestComputeDifference(t *testing.T) {
	tests := []struct {
		name     string
		external []v1alpha1.ExternalResource
		existing []string
		want     []v1alpha1.ExternalResource
	}{
		{
			name:     "NoExternalResources",
			external: []v1alpha1.ExternalResource{},
			existing: []string{"bucket-1", "bucket-2"},
			want:     []v1alpha1.ExternalResource{},
		},
		{
			name: "AllExternalResourcesManaged",
			external: []v1alpha1.ExternalResource{
				{ExternalName: "bucket-1"},
				{ExternalName: "bucket-2"},
			},
			existing: []string{"bucket-1", "bucket-2"},
			want:     []v1alpha1.ExternalResource{},
		},
		{
			name: "SomeUnmanaged",
			external: []v1alpha1.ExternalResource{
				{ExternalName: "bucket-1"},
				{ExternalName: "bucket-2"},
				{ExternalName: "bucket-3"},
			},
			existing: []string{"bucket-1"},
			want: []v1alpha1.ExternalResource{
				{ExternalName: "bucket-2"},
				{ExternalName: "bucket-3"},
			},
		},
		{
			name: "AllUnmanaged",
			external: []v1alpha1.ExternalResource{
				{ExternalName: "bucket-1"},
				{ExternalName: "bucket-2"},
			},
			existing: []string{},
			want: []v1alpha1.ExternalResource{
				{ExternalName: "bucket-1"},
				{ExternalName: "bucket-2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDifference(tt.external, tt.existing)

			// Zero out LastSeen for comparison since it's set to current time
			for i := range got {
				got[i].LastSeen = nil
			}
			for i := range tt.want {
				tt.want[i].LastSeen = nil
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("computeDifference: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name       string
		conditions []xpv2.Condition
		newCond    xpv2.Condition
		checkFn    func([]xpv2.Condition) bool
	}{
		{
			name:       "AddToEmpty",
			conditions: []xpv2.Condition{},
			newCond:    xpv2.Available(),
			checkFn: func(conds []xpv2.Condition) bool {
				return len(conds) == 1 && conds[0].Type == xpv2.Available().Type
			},
		},
		{
			name: "ReplaceExisting",
			conditions: []xpv2.Condition{
				xpv2.Available(),
				xpv2.Unavailable(),
			},
			newCond: xpv2.Unavailable().WithMessage("test"),
			checkFn: func(conds []xpv2.Condition) bool {
				if len(conds) != 2 {
					return false
				}
				// Check that the unavailable condition was replaced (message updated)
				for _, c := range conds {
					if c.Type == xpv2.Unavailable().Type && c.Message == "test" {
						return true
					}
				}
				return false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setCondition(tt.conditions, tt.newCond)

			if !tt.checkFn(got) {
				t.Errorf("setCondition: check failed for %s", tt.name)
			}
		})
	}
}
