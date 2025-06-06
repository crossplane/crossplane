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
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/resources"
)

func TestConfigurationValidation(t *testing.T) {
	cases := map[string]struct {
		reason       string
		current, old *v1.Configuration
		validatorFn  func(obj, old *v1.Configuration) field.ErrorList
		wantErrs     []string
	}{
		"CreateEmpty": {
			reason:      "Attempted to create an empty Configuration.",
			current:     resources.New[v1.Configuration](t),
			validatorFn: resources.ValidatorFor[v1.Configuration](t),
			wantErrs:    []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := tc.validatorFn(tc.current, tc.old)
			if got := len(errs); got != len(tc.wantErrs) {
				t.Errorf("expected errors %v, got %v", len(tc.wantErrs), len(errs))
				return
			}
			for i := range tc.wantErrs {
				got := errs[i].Error()
				if got != tc.wantErrs[i] {
					t.Errorf("want error %q, got %q", tc.wantErrs[i], got)
				}
			}
		})
	}
}

func TestConfigurationRevisionValidation(t *testing.T) {
	cases := map[string]struct {
		reason       string
		current, old *v1.ConfigurationRevision
		validatorFn  func(obj, old *v1.ConfigurationRevision) field.ErrorList
		wantErrs     []string
	}{
		"CreateEmpty": {
			reason:      "Attempted to create an empty ConfigurationRevision.",
			current:     resources.New[v1.ConfigurationRevision](t),
			validatorFn: resources.ValidatorFor[v1.ConfigurationRevision](t),
			wantErrs:    []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			errs := tc.validatorFn(tc.current, tc.old)
			if got := len(errs); got != len(tc.wantErrs) {
				t.Errorf("expected errors %v, got %v", len(tc.wantErrs), len(errs))
				return
			}
			for i := range tc.wantErrs {
				got := errs[i].Error()
				if got != tc.wantErrs[i] {
					t.Errorf("want error %q, got %q", tc.wantErrs[i], got)
				}
			}
		})
	}
}
