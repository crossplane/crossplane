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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

type mockHub struct{ runtime.Object }

func (h mockHub) Hub() {}

type mockConvertible struct {
	conversion.Convertible
	Fail bool
}

func (c *mockConvertible) ConvertTo(_ conversion.Hub) error {
	if c.Fail {
		return errors.New("nope")
	}
	return nil
}

func TestTryConvert(t *testing.T) {
	type args struct {
		meta       runtime.Object
		candidates []conversion.Hub
	}

	type want struct {
		meta runtime.Object
		ok   bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NotConvertible": {
			reason: "We should return the object unchanged if we try to convert an object that is not convertible.",
			args: args{
				meta: nil,
			},
			want: want{
				meta: nil,
				ok:   false,
			},
		},
		"ErrNoConversion": {
			reason: "We should return false if none of the supplied candidates convert successfully.",
			args: args{
				meta:       &mockConvertible{Fail: true},
				candidates: []conversion.Hub{&mockHub{}},
			},
			want: want{
				meta: &mockConvertible{Fail: true},
				ok:   false,
			},
		},
		"SuccessfulConversion": {
			reason: "We should not return true if one of the supplied candidates converted successfully.",
			args: args{
				meta:       &mockConvertible{},
				candidates: []conversion.Hub{&mockHub{}},
			},
			want: want{
				meta: &mockHub{},
				ok:   true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := TryConvert(tc.args.meta, tc.args.candidates...)
			if diff := cmp.Diff(tc.want.ok, ok); diff != "" {
				t.Errorf("\n%s\nTryConvert(...): -want ok, +got ok:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.meta, got); diff != "" {
				t.Errorf("\n%s\nTryConvert(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
