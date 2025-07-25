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

	"github.com/google/go-cmp/cmp"
)

func TestCapabilitiesContainFuzzy(t *testing.T) {
	type args struct {
		capabilities []string
		key          string
	}
	type want struct {
		result bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ExactMatch": {
			reason: "Should return true for exact match",
			args: args{
				capabilities: []string{"composition", "operation"},
				key:          "composition",
			},
			want: want{
				result: true,
			},
		},
		"CaseInsensitiveMatch": {
			reason: "Should return true for case insensitive match",
			args: args{
				capabilities: []string{"Composition", "OPERATION"},
				key:          "composition",
			},
			want: want{
				result: true,
			},
		},
		"HyphenatedMatch": {
			reason: "Should return true when matching hyphenated capability",
			args: args{
				capabilities: []string{"safe-start", "composition"},
				key:          "safe-start",
			},
			want: want{
				result: true,
			},
		},
		"FuzzyMatchWithSpecialChars": {
			reason: "Should return true when key has special characters that are stripped",
			args: args{
				capabilities: []string{"safestart", "composition"},
				key:          "safe-start",
			},
			want: want{
				result: true,
			},
		},
		"FuzzyMatchDifferentSeparators": {
			reason: "Should return true when capability and key use different separators",
			args: args{
				capabilities: []string{"safe_start", "composition"},
				key:          "safe-start",
			},
			want: want{
				result: true,
			},
		},
		"NoMatch": {
			reason: "Should return false when no capability matches",
			args: args{
				capabilities: []string{"composition", "operation"},
				key:          "nonexistent",
			},
			want: want{
				result: false,
			},
		},
		"EmptyCapabilities": {
			reason: "Should return false when capabilities list is empty",
			args: args{
				capabilities: []string{},
				key:          "composition",
			},
			want: want{
				result: false,
			},
		},
		"EmptyKey": {
			reason: "Should return true when key is empty and empty capability exists",
			args: args{
				capabilities: []string{"", "composition"},
				key:          "",
			},
			want: want{
				result: true,
			},
		},
		"EmptyKeyNoMatch": {
			reason: "Should return false when key is empty and no empty capability exists",
			args: args{
				capabilities: []string{"composition", "operation"},
				key:          "",
			},
			want: want{
				result: false,
			},
		},
		"SpecialCharactersOnly": {
			reason: "Should return true when both capability and key reduce to empty after stripping special chars",
			args: args{
				capabilities: []string{"---", "composition"},
				key:          "___",
			},
			want: want{
				result: true,
			},
		},
		"NumbersInCapability": {
			reason: "Should return true when capability contains numbers",
			args: args{
				capabilities: []string{"v1-composition", "operation"},
				key:          "v1composition",
			},
			want: want{
				result: true,
			},
		},
		"UnicodeCharacters": {
			reason: "Should return true when matching with unicode characters stripped",
			args: args{
				capabilities: []string{"safe→start", "composition"},
				key:          "safestart",
			},
			want: want{
				result: true,
			},
		},
		"MixedCaseAndSpecialChars": {
			reason: "Should return true when matching mixed case with special characters",
			args: args{
				capabilities: []string{"Safe-Start", "Composition"},
				key:          "SAFE_START",
			},
			want: want{
				result: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CapabilitiesContainFuzzyMatch(tc.args.capabilities, tc.args.key)

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nCapabilitiesContainFuzzy(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
