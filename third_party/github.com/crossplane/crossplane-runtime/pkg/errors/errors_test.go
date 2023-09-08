/*
Copyright 2021 The Crossplane Authors.

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

package errors

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestWrap(t *testing.T) {
	type args struct {
		err     error
		message string
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"NilError": {
			args: args{
				err:     nil,
				message: "very useful context",
			},
			want: nil,
		},
		"NonNilError": {
			args: args{
				err:     New("boom"),
				message: "very useful context",
			},
			want: Errorf("very useful context: %w", New("boom")),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Wrap(tc.args.err, tc.args.message)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Wrap(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	type args struct {
		err     error
		message string
		args    []any
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"NilError": {
			args: args{
				err:     nil,
				message: "very useful context",
			},
			want: nil,
		},
		"NonNilError": {
			args: args{
				err:     New("boom"),
				message: "very useful context about %s",
				args:    []any{"ducks"},
			},
			want: Errorf("very useful context about %s: %w", "ducks", New("boom")),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Wrapf(tc.args.err, tc.args.message, tc.args.args...)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Wrapf(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCause(t *testing.T) {
	cases := map[string]struct {
		err  error
		want error
	}{
		"NilError": {
			err:  nil,
			want: nil,
		},
		"BareError": {
			err:  New("boom"),
			want: New("boom"),
		},
		"WrappedError": {
			err:  Wrap(Wrap(New("boom"), "interstitial context"), "very important context"),
			want: New("boom"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Cause(tc.err)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Cause(...): -want, +got:\n%s", diff)
			}
		})
	}
}
