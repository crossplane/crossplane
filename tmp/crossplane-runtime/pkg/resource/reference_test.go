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

package resource

import (
	"fmt"
	"testing"
)

func TestReferenceStatusType_String(t *testing.T) {
	tests := map[string]struct {
		t    ReferenceStatusType
		want string
	}{
		"ReferenceStatusUnknown": {
			t:    ReferenceStatusUnknown,
			want: "Unknown",
		},
		"ReferenceNotFound": {
			t:    ReferenceNotFound,
			want: "NotFound",
		},
		"ReferenceNotReady": {
			t:    ReferenceNotReady,
			want: "NotReady",
		},
		"ReferenceReady": {
			t:    ReferenceReady,
			want: "Ready",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.t.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReferenceStatus_String(t *testing.T) {
	tests := map[string]struct {
		rs   ReferenceStatus
		want string
	}{
		"test-name-ready": {
			rs: ReferenceStatus{
				Name:   "test-name",
				Status: ReferenceReady,
			},
			want: fmt.Sprintf("{reference:test-name status:%s}", ReferenceReady.String()),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.rs.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
