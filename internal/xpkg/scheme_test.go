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

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type mockHub struct{ runtime.Object }

func (h mockHub) Hub() {}

type mockConvertible struct {
	conversion.Convertible
	err error
}

func (c *mockConvertible) ConvertTo(_ conversion.Hub) error { return c.err }

func TestConvertTo(t *testing.T) {
	type args struct {
		meta       runtime.Object
		candidates []conversion.Hub
	}

	type want struct {
		meta runtime.Object
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ErrNotConvertible": {
			reason: "We should return an error if we try to convert an object that is not convertible.",
			args: args{
				meta: nil,
			},
			want: want{
				err: errors.New(errNotConvertible),
			},
		},
		"ErrNoConversion": {
			reason: "We should return an error if none of the supplied candidates convert successfully.",
			args: args{
				meta:       &mockConvertible{err: errors.New("nope")},
				candidates: []conversion.Hub{&mockHub{}},
			},
			want: want{
				err: errors.New(errNoConversions),
			},
		},
		"SuccessfulConversion": {
			reason: "We should not return an error if one of the supplied candidates converted successfully.",
			args: args{
				meta:       &mockConvertible{},
				candidates: []conversion.Hub{&mockHub{}},
			},
			want: want{
				meta: &mockHub{},
				err:  nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ConvertTo(tc.args.meta, tc.args.candidates...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInternalise(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.meta, got); diff != "" {
				t.Errorf("\n%s\nInternalise(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
