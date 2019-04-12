/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"testing"

	"github.com/go-test/deep"
	. "github.com/onsi/gomega"
)

func TestToLowerRemoveSpaces(t *testing.T) {
	g := NewGomegaWithT(t)

	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"FOO", "foo"},
		{"FoO bAr", "foobar"},
		{"Foo-Bar", "foo-bar"},
	}

	for _, tt := range cases {
		actual := ToLowerRemoveSpaces(tt.input)
		g.Expect(actual).To(Equal(tt.expected))
	}
}

func TestParseMap(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]string
	}{
		{name: "empty", args: "", want: map[string]string{}},
		{name: "single", args: "foo:bar", want: map[string]string{"foo": "bar"}},
		{name: "multi", args: "foo:bar, one:two", want: map[string]string{"foo": "bar", "one": "two"}},
		{name: "dupe key", args: "foo:bar,foo:buz", want: map[string]string{"foo": "buz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMap(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseMap() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name string
		args string
		want bool
	}{
		{name: "empty", args: "", want: false},
		{name: "true", args: "true", want: true},
		{name: "True", args: "True", want: true},
		{name: "tRue", args: "tRue", want: false},
		{name: "_true", args: " true", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBool(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("parseBool() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestConditionalStringFormat(t *testing.T) {
	type args struct {
		format string
		value  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no name format",
			args: args{
				format: "",
				value:  "test-value",
			},
			want: "test-value",
		},
		{
			name: "format string only",
			args: args{
				format: "%s",
				value:  "test-value",
			},
			want: "test-value",
		},
		{
			name: "format string at the beginning",
			args: args{
				format: "%s-foo",
				value:  "test-value",
			},
			want: "test-value-foo",
		},
		{
			name: "format string at the end",
			args: args{
				format: "foo-%s",
				value:  "test-value",
			},
			want: "foo-test-value",
		},
		{
			name: "format string in the middle",
			args: args{
				format: "foo-%s-bar",
				value:  "test-value",
			},
			want: "foo-test-value-bar",
		},
		{
			name: "constant string",
			args: args{
				format: "foo-bar",
				value:  "test-value",
			},
			want: "foo-bar",
		},
		{
			name: "invalid: multiple substitutions",
			args: args{
				format: "foo-%s-bar-%s",
				value:  "test-value",
			},
			want: "foo-test-value-bar-%!s(MISSING)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConditionalStringFormat(tt.args.format, tt.args.value)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("ConditionalStringFormat() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	type args struct {
		s   string
		sep string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{name: "empty", args: args{s: "", sep: ","}, want: []string{}},
		{name: "comma", args: args{s: ",", sep: ","}, want: []string{}},
		{name: "values", args: args{s: " a,,b ", sep: ","}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.args.s, tt.args.sep)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Split() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}
