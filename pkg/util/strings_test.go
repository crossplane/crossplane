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

	"github.com/google/go-cmp/cmp"
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
		{name: "Empty", args: "", want: map[string]string{}},
		{name: "Single", args: "foo:bar", want: map[string]string{"foo": "bar"}},
		{name: "Multi", args: "foo:bar, one:two", want: map[string]string{"foo": "bar", "one": "two"}},
		{name: "DupeKey", args: "foo:bar,foo:buz", want: map[string]string{"foo": "buz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMap(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
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
		{name: "Empty", args: "", want: false},
		{name: "LowerTrue", args: "true", want: true},
		{name: "UpperTrue", args: "True", want: true},
		{name: "UpperRTrue", args: "tRue", want: false},
		{name: "SpaceTrue", args: " true", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBool(tt.args)
			if diff := cmp.Diff(got, tt.want); diff != "" {
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
			name: "NoNameFormat",
			args: args{
				format: "",
				value:  "test-value",
			},
			want: "test-value",
		},
		{
			name: "FormatStringOnly",
			args: args{
				format: "%s",
				value:  "test-value",
			},
			want: "test-value",
		},
		{
			name: "FormatStringAtTheBeginning",
			args: args{
				format: "%s-foo",
				value:  "test-value",
			},
			want: "test-value-foo",
		},
		{
			name: "FormatStringAtTheEnd",
			args: args{
				format: "foo-%s",
				value:  "test-value",
			},
			want: "foo-test-value",
		},
		{
			name: "FormatStringInTheMiddle",
			args: args{
				format: "foo-%s-bar",
				value:  "test-value",
			},
			want: "foo-test-value-bar",
		},
		{
			name: "ConstantString",
			args: args{
				format: "foo-bar",
				value:  "test-value",
			},
			want: "foo-bar",
		},
		{
			name: "InvalidMultipleSubstitutions",
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
			if diff := cmp.Diff(got, tt.want); diff != "" {
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
		{name: "Empty", args: args{s: "", sep: ","}, want: []string{}},
		{name: "Comma", args: args{s: ",", sep: ","}, want: []string{}},
		{name: "Values", args: args{s: " a,,b ", sep: ","}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.args.s, tt.args.sep)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("Split() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}
