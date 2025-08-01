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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestActivationPolicyMatch(t *testing.T) {
	type args struct {
		policy ActivationPolicy
		name   string
	}
	type want struct {
		match bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ExactMatch": {
			reason: "Should match exact name",
			args: args{
				policy: ActivationPolicy("bucket.aws.crossplane.io"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"ExactMatchNoMatch": {
			reason: "Should not match different exact name",
			args: args{
				policy: ActivationPolicy("bucket.aws.crossplane.io"),
				name:   "instance.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"WildcardPrefixMatch": {
			reason: "Should match name with wildcard prefix",
			args: args{
				policy: ActivationPolicy("*.aws.crossplane.io"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"WildcardPrefixMatchMultiple": {
			reason: "Should match multiple components with wildcard prefix",
			args: args{
				policy: ActivationPolicy("*.aws.crossplane.io"),
				name:   "rds.instance.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"WildcardPrefixNoMatch": {
			reason: "Should not match different domain with wildcard prefix",
			args: args{
				policy: ActivationPolicy("*.aws.crossplane.io"),
				name:   "bucket.gcp.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"WildcardSuffixMatch": {
			reason: "Should match name with wildcard suffix",
			args: args{
				policy: ActivationPolicy("bucket.*"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"WildcardSuffixNoMatch": {
			reason: "Should not match different prefix with wildcard suffix",
			args: args{
				policy: ActivationPolicy("bucket.*"),
				name:   "instance.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"WildcardBothEndsMatch": {
			reason: "Should match name with wildcards at both ends",
			args: args{
				policy: ActivationPolicy("*aws*"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"WildcardBothEndsNoMatch": {
			reason: "Should not match name without pattern with wildcards at both ends",
			args: args{
				policy: ActivationPolicy("*aws*"),
				name:   "bucket.gcp.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"WildcardOnlyMatch": {
			reason: "Should match any name with wildcard only",
			args: args{
				policy: ActivationPolicy("*"),
				name:   "anything.goes.here",
			},
			want: want{
				match: true,
			},
		},
		"EmptyPolicyNoMatch": {
			reason: "Should not match non-empty name with empty policy",
			args: args{
				policy: ActivationPolicy(""),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"EmptyPolicyEmptyNameMatch": {
			reason: "Should match empty name with empty policy",
			args: args{
				policy: ActivationPolicy(""),
				name:   "",
			},
			want: want{
				match: true,
			},
		},
		"EmptyNameNoMatch": {
			reason: "Should not match empty name with non-empty policy",
			args: args{
				policy: ActivationPolicy("bucket.aws.crossplane.io"),
				name:   "",
			},
			want: want{
				match: false,
			},
		},
		"ComplexWildcardMatch": {
			reason: "Should match complex wildcard pattern",
			args: args{
				policy: ActivationPolicy("*bucket*.aws.*"),
				name:   "s3bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"ComplexWildcardNoMatch": {
			reason: "Should not match name that doesn't fit complex wildcard pattern",
			args: args{
				policy: ActivationPolicy("*bucket*.aws.*"),
				name:   "instance.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"QuestionMarkWildcardMatch": {
			reason: "Should match single character with question mark wildcard",
			args: args{
				policy: ActivationPolicy("bucket?.aws.crossplane.io"),
				name:   "bucket1.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"QuestionMarkWildcardNoMatch": {
			reason: "Should not match multiple characters with question mark wildcard",
			args: args{
				policy: ActivationPolicy("bucket?.aws.crossplane.io"),
				name:   "bucket123.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"CharacterClassMatch": {
			reason: "Should match character class pattern",
			args: args{
				policy: ActivationPolicy("bucket[0-9].aws.crossplane.io"),
				name:   "bucket5.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"CharacterClassNoMatch": {
			reason: "Should not match character outside class pattern",
			args: args{
				policy: ActivationPolicy("bucket[0-9].aws.crossplane.io"),
				name:   "bucketa.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"EscapedWildcardMatch": {
			reason: "Should match literal asterisk when escaped",
			args: args{
				policy: ActivationPolicy("bucket\\*.aws.crossplane.io"),
				name:   "bucket*.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"EscapedWildcardNoMatch": {
			reason: "Should not match expanded form when asterisk is escaped",
			args: args{
				policy: ActivationPolicy("bucket\\*.aws.crossplane.io"),
				name:   "bucket123.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"CaseSensitiveMatch": {
			reason: "Should be case sensitive for exact match",
			args: args{
				policy: ActivationPolicy("Bucket.aws.crossplane.io"),
				name:   "Bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"CaseSensitiveNoMatch": {
			reason: "Should not match different case",
			args: args{
				policy: ActivationPolicy("Bucket.aws.crossplane.io"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"WildcardCaseSensitiveMatch": {
			reason: "Should be case sensitive with wildcard",
			args: args{
				policy: ActivationPolicy("*.AWS.crossplane.io"),
				name:   "bucket.AWS.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
		"WildcardCaseSensitiveNoMatch": {
			reason: "Should not match different case with wildcard",
			args: args{
				policy: ActivationPolicy("*.AWS.crossplane.io"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: false,
			},
		},
		"DotInWildcardMatch": {
			reason: "Should match dot characters in wildcard",
			args: args{
				policy: ActivationPolicy("*.crossplane.io"),
				name:   "bucket.aws.crossplane.io",
			},
			want: want{
				match: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.policy.Match(tc.args.name)

			if diff := cmp.Diff(tc.want.match, got); diff != "" {
				t.Errorf("\n%s\nMatch(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagedResourceActivationPolicyStatusAppendActivated(t *testing.T) {
	type args struct {
		status *ManagedResourceActivationPolicyStatus
		name   string
	}
	type want struct {
		activated []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilActivatedList": {
			reason: "Should create new activated list when nil",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: nil,
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io"},
			},
		},
		"EmptyActivatedList": {
			reason: "Should append to empty activated list",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io"},
			},
		},
		"SingleItemList": {
			reason: "Should append to single item list and maintain sort order",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"instance.aws.crossplane.io"},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"MultipleItemsList": {
			reason: "Should append to multiple items list and maintain sort order",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"database.aws.crossplane.io", "instance.aws.crossplane.io"},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "database.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"AppendAtEnd": {
			reason: "Should append item that belongs at end of sorted list",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"bucket.aws.crossplane.io", "database.aws.crossplane.io"},
				},
				name: "instance.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "database.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"AppendAtBeginning": {
			reason: "Should append item that belongs at beginning of sorted list",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"database.aws.crossplane.io", "instance.aws.crossplane.io"},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "database.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"AppendInMiddle": {
			reason: "Should append item that belongs in middle of sorted list",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"bucket.aws.crossplane.io", "instance.aws.crossplane.io"},
				},
				name: "database.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "database.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"DuplicateItem": {
			reason: "Should handle duplicate items correctly with sorting",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"bucket.aws.crossplane.io", "instance.aws.crossplane.io"},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activated: []string{"bucket.aws.crossplane.io", "bucket.aws.crossplane.io", "instance.aws.crossplane.io"},
			},
		},
		"LargeList": {
			reason: "Should handle large activated list efficiently",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{
						"resource1.aws.crossplane.io",
						"resource2.aws.crossplane.io",
						"resource3.aws.crossplane.io",
						"resource4.aws.crossplane.io",
						"resource5.aws.crossplane.io",
						"resource6.aws.crossplane.io",
						"resource7.aws.crossplane.io",
						"resource8.aws.crossplane.io",
						"resource9.aws.crossplane.io",
					},
				},
				name: "resource10.aws.crossplane.io",
			},
			want: want{
				activated: []string{
					"resource1.aws.crossplane.io",
					"resource10.aws.crossplane.io",
					"resource2.aws.crossplane.io",
					"resource3.aws.crossplane.io",
					"resource4.aws.crossplane.io",
					"resource5.aws.crossplane.io",
					"resource6.aws.crossplane.io",
					"resource7.aws.crossplane.io",
					"resource8.aws.crossplane.io",
					"resource9.aws.crossplane.io",
				},
			},
		},
		"EmptyName": {
			reason: "Should handle empty name correctly",
			args: args{
				status: &ManagedResourceActivationPolicyStatus{
					Activated: []string{"bucket.aws.crossplane.io"},
				},
				name: "",
			},
			want: want{
				activated: []string{"", "bucket.aws.crossplane.io"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.status.AppendActivated(tc.args.name)

			if diff := cmp.Diff(tc.want.activated, tc.args.status.Activated); diff != "" {
				t.Errorf("\n%s\nAppendActivated(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagedResourceActivationPolicyActivates(t *testing.T) {
	type args struct {
		policy *ManagedResourceActivationPolicy
		name   string
	}
	type want struct {
		activate bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyActivationList": {
			reason: "Should not activate any name when activation list is empty",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{},
					},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activate: false,
			},
		},
		"NilActivationList": {
			reason: "Should not activate any name when activation list is nil",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: nil,
					},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activate: false,
			},
		},
		"SingleActivationMatch": {
			reason: "Should activate name that matches single activation policy",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
						},
					},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"SingleActivationNoMatch": {
			reason: "Should not activate name that doesn't match single activation policy",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
						},
					},
				},
				name: "instance.aws.crossplane.io",
			},
			want: want{
				activate: false,
			},
		},
		"MultipleActivationsFirstMatch": {
			reason: "Should activate name that matches first of multiple activation policies",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"instance.aws.crossplane.io",
							"*.gcp.crossplane.io",
						},
					},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"MultipleActivationsMiddleMatch": {
			reason: "Should activate name that matches middle of multiple activation policies",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"instance.aws.crossplane.io",
							"*.gcp.crossplane.io",
						},
					},
				},
				name: "instance.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"MultipleActivationsLastMatch": {
			reason: "Should activate name that matches last of multiple activation policies",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"instance.aws.crossplane.io",
							"*.gcp.crossplane.io",
						},
					},
				},
				name: "storage.gcp.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"MultipleActivationsNoMatch": {
			reason: "Should not activate name that doesn't match any of multiple activation policies",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"instance.aws.crossplane.io",
							"*.gcp.crossplane.io",
						},
					},
				},
				name: "database.azure.crossplane.io",
			},
			want: want{
				activate: false,
			},
		},
		"MultipleActivationsWildcardMatch": {
			reason: "Should activate name that matches wildcard in multiple activation policies",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"specific.aws.crossplane.io",
							"*.aws.crossplane.io",
							"exact.gcp.crossplane.io",
						},
					},
				},
				name: "anything.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"LargeActivationList": {
			reason: "Should handle large activation list efficiently",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"resource1.aws.crossplane.io",
							"resource2.aws.crossplane.io",
							"resource3.aws.crossplane.io",
							"resource4.aws.crossplane.io",
							"resource5.aws.crossplane.io",
							"resource6.aws.crossplane.io",
							"resource7.aws.crossplane.io",
							"resource8.aws.crossplane.io",
							"resource9.aws.crossplane.io",
							"resource10.aws.crossplane.io",
						},
					},
				},
				name: "resource5.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
		"LargeActivationListNoMatch": {
			reason: "Should not activate when name doesn't match any in large activation list",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"resource1.aws.crossplane.io",
							"resource2.aws.crossplane.io",
							"resource3.aws.crossplane.io",
							"resource4.aws.crossplane.io",
							"resource5.aws.crossplane.io",
							"resource6.aws.crossplane.io",
							"resource7.aws.crossplane.io",
							"resource8.aws.crossplane.io",
							"resource9.aws.crossplane.io",
							"resource10.aws.crossplane.io",
						},
					},
				},
				name: "resource11.aws.crossplane.io",
			},
			want: want{
				activate: false,
			},
		},
		"EmptyNameWithActivations": {
			reason: "Should not activate empty name even with activations present",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"*.aws.crossplane.io",
						},
					},
				},
				name: "",
			},
			want: want{
				activate: false,
			},
		},
		"EmptyPolicyInList": {
			reason: "Should handle empty policy in activation list",
			args: args{
				policy: &ManagedResourceActivationPolicy{
					Spec: ManagedResourceActivationPolicySpec{
						Activations: []ActivationPolicy{
							"bucket.aws.crossplane.io",
							"", // empty policy
							"*.gcp.crossplane.io",
						},
					},
				},
				name: "bucket.aws.crossplane.io",
			},
			want: want{
				activate: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.policy.Activates(tc.args.name)

			if diff := cmp.Diff(tc.want.activate, got); diff != "" {
				t.Errorf("\n%s\nActivates(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
