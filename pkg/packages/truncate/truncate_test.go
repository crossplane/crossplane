/*
Copyright 2020 The Crossplane Authors.

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

// Package truncate provides functions for truncating Kubernetes values in a
// predictable way offering safe values usable in deterministic field searches.
package truncate

import (
	"strings"
	"testing"
)

var (
	str63  = strings.Repeat("1234567890", 6) + "123"
	str253 = strings.Repeat("1234567890", 25) + "123"
)

func Test_truncate(t *testing.T) {
	type args struct {
		str          string
		length       int
		suffixLength int
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "StrTooShort",
			args:    args{str: "a", length: 0, suffixLength: 0},
			want:    "",
			wantErr: true,
		},
		{
			name:    "SuffixExceedsStr",
			args:    args{str: "a", length: 0, suffixLength: 2},
			want:    "",
			wantErr: true,
		},
		{
			name:    "SuffixExceedsSum",
			args:    args{str: strings.Repeat("12345678890", 21), length: 200, suffixLength: 100},
			want:    "",
			wantErr: true,
		},
		{
			name:    "SuffixTooShort",
			args:    args{str: "12345678901", length: 10, suffixLength: 1},
			want:    "",
			wantErr: true,
		},
		{
			name:    "StrNotTruncated",
			args:    args{str: "1234567890", length: 10, suffixLength: 1},
			want:    "1234567890",
			wantErr: false,
		},
		{
			name:    "StrTruncated",
			args:    args{str: "12345678901", length: 10, suffixLength: 5},
			want:    "12345-ezw4",
			wantErr: false,
		},
		{
			name:    "AllTruncated",
			args:    args{str: "1234567890", length: 5, suffixLength: 5},
			want:    "-agzq",
			wantErr: false,
		},
		{
			name:    "TruncatedOnDot",
			args:    args{str: "1234.678901", length: 10, suffixLength: 5},
			want:    "1234-kjwi",
			wantErr: false,
		},
		{
			name: "hosted-package-ns-example",
			args: args{str: "cool-namespace-abcdefghijklmnopqrstuvwxyz", length: 32, suffixLength: 5},
			want: "cool-namespace-abcdefghijkl-lzi4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Truncate(tt.args.str, tt.args.length, tt.args.suffixLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("truncate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("truncate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabelName(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "short",
			args: args{str: "foo"},
			want: "foo",
		},
		{
			name: "max",
			args: args{str: str63},
			want: str63,
		},
		{
			name: "truncate",
			args: args{str: str253},
			want: strings.Repeat("1234567890", 5) + "1234567-ij43w",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LabelName(tt.args.str); got != tt.want {
				t.Errorf("LabelName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabelValue(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "short",
			args: args{str: "foo"},
			want: "foo",
		},
		{
			name: "max",
			args: args{str: str63},
			want: str63,
		},
		{
			name: "truncate",
			args: args{str: str253},
			want: strings.Repeat("1234567890", 5) + "1234567-ij43w",
		},

		{
			name: "hosted-stack-ns-and-name-example",
			args: args{str: "cool-namespace-abcdefghijkl-lzi4.cool-stack-abcdefghijklmnopqrstuvwxyz"},
			want: "cool-namespace-abcdefghijkl-lzi4.cool-stack-abcdefghijklm-h4q4b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LabelValue(tt.args.str); got != tt.want {
				t.Errorf("LabelValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceName(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "short",
			args: args{str: "foo"},
			want: "foo",
		},
		{
			name: "max",
			args: args{str: str253},
			want: str253,
		},
		{
			name: "truncate",
			args: args{str: str253 + "z"},
			want: strings.Repeat("1234567890", 24) + "1234567-lsbgh",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResourceName(tt.args.str); got != tt.want {
				t.Errorf("ResourceName() = %v, want %v", got, tt.want)
			}
		})
	}
}
