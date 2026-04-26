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

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParseAnnotations(t *testing.T) {
	type args struct {
		kvs []string
	}
	type want struct {
		anns map[string]string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptySlice": {
			reason: "Empty input should return an empty map with no error.",
			args:   args{kvs: []string{}},
			want:   want{anns: map[string]string{}},
		},
		"SingleEntry": {
			reason: "A single valid key=value entry should be parsed correctly.",
			args:   args{kvs: []string{"org.example/key=value"}},
			want:   want{anns: map[string]string{"org.example/key": "value"}},
		},
		"MultipleEntries": {
			reason: "Multiple valid key=value entries should all be parsed.",
			args: args{kvs: []string{
				"org.opencontainers.image.source=https://github.com/example/pkg",
				"org.opencontainers.image.version=v1.0.0",
			}},
			want: want{anns: map[string]string{
				"org.opencontainers.image.source":  "https://github.com/example/pkg",
				"org.opencontainers.image.version": "v1.0.0",
			}},
		},
		"ValueContainsEquals": {
			reason: "Values that contain '=' characters should be preserved intact.",
			args:   args{kvs: []string{"key=val=ue"}},
			want:   want{anns: map[string]string{"key": "val=ue"}},
		},
		"MissingEquals": {
			reason: "An entry without '=' should return an error.",
			args:   args{kvs: []string{"invalid-no-equals"}},
			want:   want{err: cmpopts.AnyError},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseAnnotations(tc.args.kvs)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nparseAnnotations(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.anns, got); diff != "" {
				t.Errorf("\n%s\nparseAnnotations(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMergeAnnotations(t *testing.T) {
	type args struct {
		metaAnnotations map[string]string
		flagAnnotations []string
	}
	type want struct {
		anns map[string]string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FlagOnly": {
			reason: "Flag annotations alone should appear in the result.",
			args: args{
				metaAnnotations: nil,
				flagAnnotations: []string{"org.example/env=production"},
			},
			want: want{anns: map[string]string{"org.example/env": "production"}},
		},
		"MetadataOnly": {
			reason: "Metadata annotations alone should appear in the result.",
			args: args{
				metaAnnotations: map[string]string{"org.example/team": "platform"},
				flagAnnotations: []string{},
			},
			want: want{anns: map[string]string{"org.example/team": "platform"}},
		},
		"FlagOverridesMetadata": {
			reason: "Flag annotation should override a metadata annotation with the same key.",
			args: args{
				metaAnnotations: map[string]string{
					"org.example/env":  "staging",
					"org.example/team": "platform",
				},
				flagAnnotations: []string{"org.example/env=production"},
			},
			want: want{anns: map[string]string{
				"org.example/env":  "production",
				"org.example/team": "platform",
			}},
		},
		"BothEmpty": {
			reason: "Empty metadata and no flags should produce an empty map.",
			args: args{
				metaAnnotations: nil,
				flagAnnotations: []string{},
			},
			want: want{anns: map[string]string{}},
		},
		"MalformedFlag": {
			reason: "A malformed flag annotation (missing '=') should return an error.",
			args: args{
				metaAnnotations: map[string]string{"org.example/key": "val"},
				flagAnnotations: []string{"bad-annotation"},
			},
			want: want{err: cmpopts.AnyError},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := mergeAnnotations(tc.args.metaAnnotations, tc.args.flagAnnotations)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nmergeAnnotations(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.anns, got); diff != "" {
				t.Errorf("\n%s\nmergeAnnotations(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
