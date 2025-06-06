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

package validation

import (
	"testing"
	
	"github.com/crossplane/crossplane/test/resources"
)

// DoValidationTest is external so we can have generics.
func DoValidationTest[T any](t *testing.T, current, old *T, wantErrs []string) {
	t.Helper()
	validator := resources.ValidatorFor[T](t)

	errs := validator(current, old)
	t.Log(errs)

	if got := len(errs); got != len(wantErrs) {
		t.Errorf("expected errors %v, got %v", len(wantErrs), len(errs))
		return
	}

	for i := range wantErrs {
		got := errs[i].Error()
		if got != wantErrs[i] {
			t.Errorf("want error %q, got %q", wantErrs[i], got)
		}
	}
}
