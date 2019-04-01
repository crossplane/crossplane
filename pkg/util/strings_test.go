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

func Test_parseMap(t *testing.T) {
	tests := []struct {
		name string
		args string
		want map[string]string
	}{
		{name: "empty", args: "", want: map[string]string{}},
		{name: "single", args: "foo:bar", want: map[string]string{"foo": "bar"}},
		{name: "multi", args: "foo:bar, one:two", want: map[string]string{"foo": "bar", "one": "two"}},
		{name: "dupe key", args: "foo:bar,foo:buz", want: map[string]string{"foo": "buz"}},
		{name: "nested map",
			args: "[foo:bar,nested:[foo:bar,fooz:booz]]",
			want: map[string]string{"foo": "bar", "nested": "foo:bar,fooz:booz"}},
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

func Test_parseBool(t *testing.T) {
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
