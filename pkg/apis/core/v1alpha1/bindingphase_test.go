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
	"testing"

	"github.com/go-test/deep"
)

const jsonQuote = "\""

func TestBindingState(t *testing.T) {
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
			if diff := deep.Equal(tc.want, got); diff != nil {
				t.Errorf("BindingState.MarshalJSON(): want != got\n %+v", diff)
			}
		})
	}
}
