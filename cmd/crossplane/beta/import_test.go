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

// Package beta implements Crossplane beta (experimental) commands.
package beta

import (
	"testing"
)

// TestGenerateMRName tests that the name generation is stable and consistent.
func TestGenerateMRName(t *testing.T) {
	type args struct {
		kind         string
		externalName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Simple bucket name",
			args: args{
				kind:         "Bucket",
				externalName: "my-bucket",
			},
		},
		{
			name: "Name with special characters",
			args: args{
				kind:         "Bucket",
				externalName: "bucket-prod-2025-01-15",
			},
		},
		{
			name: "Different kinds produce different prefixes",
			args: args{
				kind:         "Database",
				externalName: "db-prod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate name twice to test idempotency
			got1 := generateMRName(tt.args.kind, tt.args.externalName)
			got2 := generateMRName(tt.args.kind, tt.args.externalName)

			if got1 != got2 {
				t.Errorf("generateMRName not idempotent: first=%q, second=%q", got1, got2)
			}

			// Verify format (should be <kind>-<hash>)
			if len(got1) < len(tt.args.kind)+9 { // kind + dash + 8-char hash
				t.Errorf("generateMRName produced too short name: %q", got1)
			}
		})
	}
}

// TestSetDifference tests the set difference operation.
func TestSetDifference(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected int // Just check length
	}{
		{
			name:     "Empty sets",
			a:        []string{},
			b:        []string{},
			expected: 0,
		},
		{
			name:     "All in a are unmanaged",
			a:        []string{"bucket-1", "bucket-2"},
			b:        []string{},
			expected: 2,
		},
		{
			name:     "All in a are managed",
			a:        []string{"bucket-1", "bucket-2"},
			b:        []string{"bucket-1", "bucket-2"},
			expected: 0,
		},
		{
			name:     "Partial overlap",
			a:        []string{"bucket-1", "bucket-2", "bucket-3"},
			b:        []string{"bucket-1"},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setDifference(tt.a, tt.b)

			if len(got) != tt.expected {
				t.Errorf("setDifference length mismatch: got %d, expected %d", len(got), tt.expected)
			}
		})
	}
}
