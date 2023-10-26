/*
Copyright 2023 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing perimpliedions and
limitations under the License.
*/

package semver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGreatestLowerBounds(t *testing.T) {
	type args struct {
		constraints string
	}
	type want struct {
		versions []string
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"WithOperator": {reason: "No operator gives the value as lower bound", args: args{constraints: "1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"Equal":        {reason: "= gives the value as lower bound", args: args{constraints: "=1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"Inequality":   {reason: "Inequality gives no lower bound", args: args{constraints: "!=1.2.3"}, want: want{versions: []string{"0.0.0-a"}}},
		"Greater":      {reason: "> gives the value as lower bound", args: args{constraints: ">1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"GreaterEqual": {reason: ">= gives the value as lower bound", args: args{constraints: ">=1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"EqualGreater": {reason: "=> gives the value as lower bound", args: args{constraints: "=>1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"Less":         {reason: "< gives no lower bound", args: args{constraints: "<1.2.3"}, want: want{versions: []string{"0.0.0-a"}}},
		"LessEqual":    {reason: "<= gives no lower bound", args: args{constraints: "<=1.2.3"}, want: want{versions: []string{"0.0.0-a"}}},
		"EqualLess":    {reason: "=< gives no lower bound", args: args{constraints: "=<1.2.3"}, want: want{versions: []string{"0.0.0-a"}}},
		"Tilde":        {reason: "~ gives a lower bound", args: args{constraints: "~1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"TildeGreater": {reason: "~> gives a lower bound", args: args{constraints: "~>1.2.3"}, want: want{versions: []string{"1.2.3"}}},
		"Caret":        {reason: "^ gives a lower bound", args: args{constraints: "^1.2.3"}, want: want{versions: []string{"1.2.3"}}},

		"NoLowerBoundsIgnored": {reason: "Constraints without lower bound are ignored", args: args{constraints: ">=1.2.3,<2.0.0"}, want: want{versions: []string{"1.2.3"}}},
		"GreatestLowerBound":   {reason: "Multiple lower bounds result in the greatest of them", args: args{constraints: ">=1.2.3,<2.0.0,>=2.1.0"}, want: want{versions: []string{"2.1.0"}}},
		"MultipleConstraints":  {reason: "Multiple constraints result in multiple lower bounds", args: args{constraints: ">=1.2.3,<2.0.0 || >=2.1.0"}, want: want{versions: []string{"1.2.3", "2.1.0"}}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cs, err := NewConstraint(tc.args.constraints)
			if err != nil {
				t.Fatalf("NewConstraint(...): %v", err)
			}
			vs := greatestLowerBounds(cs.constraints)
			got := make([]string, len(vs))
			for i, v := range vs {
				got[i] = v.String()
			}

			if diff := cmp.Diff(tc.want.versions, got); diff != "" {
				t.Errorf("\n%s\ngreatestLowerBounds(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCheckWithBreakingVersion(t *testing.T) {
	type args struct {
		constraints    string
		breakingChange string
		version        string
	}
	type want struct {
		passed bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BreakingVersionSpecified": {
			reason: "A constraint forces the version with the breaking change",
			args:   args{constraints: ">=2.1", breakingChange: "2", version: "2.3.1"}, want: want{passed: true},
		},
		"BreakingVersionNotSpecified": {
			reason: "No constraint forces the version with the breaking change",
			args:   args{constraints: ">=1.2", breakingChange: "2", version: "2.3.1"}, want: want{passed: false},
		},
		"BreakingVersionNotSpecifiedButCheckingOldVersion": {
			reason: "No constraint forces the version with the breaking change, but we are checking a version before",
			args:   args{constraints: ">=1.2", breakingChange: "2", version: "1.2.3"}, want: want{passed: true},
		},
		"OnlyOneConstraintForcesBreakingChange": {
			reason: "A constraint forces the version with the breaking change",
			args:   args{constraints: "=1.2.4 || >=2.1 || >1.3", breakingChange: "2", version: "2.3.1"}, want: want{passed: true},
		},
		"OnlyOneConstraintForcesBreakingChangeCheckingOldVersion": {
			reason: "A constraint forces the version with the breaking change",
			args:   args{constraints: "=1.2.3 || >=2.1 || >1.3", breakingChange: "2", version: "1.2.3"}, want: want{passed: true},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cs, err := NewConstraint(tc.args.constraints)
			if err != nil {
				t.Fatalf("NewConstraint(...): %v", err)
			}
			v, err := NewVersion(tc.args.version)
			if err != nil {
				t.Fatalf("NewVersion(version): %v", err)
			}
			breakingChange, err := NewVersion(tc.args.breakingChange)
			if err != nil {
				t.Fatalf("NewVersion(breakingChange): %v", err)
			}

			got := cs.CheckWithBreakingVersion(v, breakingChange)

			if diff := cmp.Diff(tc.want.passed, got); diff != "" {
				t.Errorf("\n%s\nCheckWithBreakingVersion(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
