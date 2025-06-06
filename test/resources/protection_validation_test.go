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

package resources

import (
	"testing"

	protectionv1beta1 "github.com/crossplane/crossplane/apis/protection/v1beta1"
)

func TestUsageValidation_protection_v1beta1(t *testing.T) {
	cases := map[string]struct {
		reason       string
		current, old *protectionv1beta1.Usage
		wantErrs     []string
	}{
		"CreateEmpty": {
			reason:   "Attempted to create an empty FunctionRevision.",
			current:  New[protectionv1beta1.Usage](t),
			wantErrs: []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doValidationTest(t, tc.current, tc.old, tc.wantErrs)
		})
	}
}
