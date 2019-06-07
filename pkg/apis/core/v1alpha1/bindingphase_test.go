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

package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const jsonQuote = "\""

func TestBindingStateMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		s    BindingState
		want []byte
	}{
		{
			name: BindingStateUnbound.String(),
			s:    BindingStateUnbound,
			want: []byte(jsonQuote + BindingStateUnbound.String() + jsonQuote),
		},
		{
			name: BindingStateBound.String(),
			s:    BindingStateBound,
			want: []byte(jsonQuote + BindingStateBound.String() + jsonQuote),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.s.MarshalJSON()
			if err != nil {
				t.Errorf("BindingState.MarshalJSON(): %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("BindingState.MarshalJSON(): -want, +got\n %+v", diff)
			}
		})
	}
}

func TestBindingStateUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name    string
		s       []byte
		want    BindingState
		wantErr error
	}{
		{
			name: BindingStateUnbound.String(),
			s:    []byte(jsonQuote + BindingStateUnbound.String() + jsonQuote),
			want: BindingStateUnbound,
		},
		{
			name: BindingStateBound.String(),
			s:    []byte(jsonQuote + BindingStateBound.String() + jsonQuote),
			want: BindingStateBound,
		},
		{
			name:    "Unknown",
			s:       []byte(jsonQuote + "Unknown" + jsonQuote),
			wantErr: errors.New("unknown binding state Unknown"),
		},
		{
			name: "NotAString",
			s:    []byte{1},

			// json.Unmarshal returns a *json.SyntaxError with an unexported
			// string message. We can't create one explicitly, so we create the
			// expected error here to compare them.
			wantErr: func() error {
				var i int
				return json.Unmarshal([]byte{1}, &i)
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got BindingState
			gotErr := got.UnmarshalJSON(tc.s)
			if diff := cmp.Diff(tc.wantErr, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("BindingState.UnmarshalJSON(): want error != got error\n %+v", diff)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("BindingState.UnmarshalJSON(): -want, +got\n %+v", diff)
			}
		})
	}
}
