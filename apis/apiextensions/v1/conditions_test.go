/*
Copyright 2025 The Crossplane Authors.

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

package v1

import (
	"bytes"
	"encoding/json"
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

func TestIsSystemConditionType(t *testing.T) {
	cases := map[string]struct {
		reason        string
		conditionType xpv2.ConditionType
		want          bool
	}{
		"CrossplaneRuntimeSystemCondition": {
			reason:        "builtin ready condition should be system type",
			conditionType: xpv2.TypeReady,
			want:          true,
		},
		"CrossplaneRuntimeSystemConditionSynced": {
			reason:        "builtin synced condition should be system type",
			conditionType: xpv2.TypeSynced,
			want:          true,
		},
		"CrossplaneCircuitCondition": {
			reason:        "circuit responsive condition should be system type",
			conditionType: TypeResponsive,
			want:          true,
		},
		"CustomCondition": {
			reason:        "custom database condition should not be system type",
			conditionType: "DatabaseReady",
			want:          false,
		},
		"AnotherCustomCondition": {
			reason:        "custom bucket condition should not be system type",
			conditionType: "BucketReady",
			want:          false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := IsSystemConditionType(tc.conditionType)
			if got != tc.want {
				t.Errorf("%s: IsSystemConditionType(%q) = %v, want %v", tc.reason, tc.conditionType, got, tc.want)
			}
		})
	}
}

func TestWatchCircuitClosedSerializesMessage(t *testing.T) {
	// The XR status is written with server-side apply, which only takes
	// ownership of fields present in the patch. Condition.Message is
	// json:"message,omitempty", so if the closed condition has no message the
	// key is absent from the patch and a message left behind by
	// WatchCircuitOpen can never be cleared. Condition.Equal compares Message,
	// so the orphaned message makes SetConditions rewrite the XR on every
	// reconcile, and the XR's self-watch turns that into an infinite loop.
	got, err := json.Marshal(WatchCircuitClosed())
	if err != nil {
		t.Fatalf("json.Marshal(WatchCircuitClosed()): %v", err)
	}
	if !bytes.Contains(got, []byte(`"message"`)) {
		t.Errorf("WatchCircuitClosed() must serialize a message key so server-side apply owns (and can clear) the field, got %s", got)
	}
}
