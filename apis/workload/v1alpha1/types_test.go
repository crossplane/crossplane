/*
Copyright 2019 The Crossplane Authors.

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
)

func TestRemoteStatus(t *testing.T) {
	cases := []struct {
		name string
		want []byte
	}{
		{
			name: "ValidJSONObject",
			want: []byte(`{"coolness":"EXTREME!"}`),
		},
		{
			name: "ValidJSONArray",
			want: []byte(`["cool","cooler","coolest"]`),
		},
		{
			name: "ValidJSONString",
			want: []byte(`"hi"`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &RemoteStatus{}
			if err := json.Unmarshal(tc.want, rs); err != nil {
				t.Fatalf("json.Unmarshal(...): %s", err)
			}

			if diff := cmp.Diff(string(rs.Raw), string(tc.want)); diff != "" {
				t.Errorf("json.Unmarshal(...): got != want: %s", diff)
			}

			got, err := json.Marshal(rs)
			if err != nil {
				t.Fatalf("json.Marshal(...): %s", err)
			}

			if diff := cmp.Diff(string(got), string(tc.want)); diff != "" {
				t.Errorf("json.Marshal(...): got != want: %s", diff)
			}
		})
	}
}
