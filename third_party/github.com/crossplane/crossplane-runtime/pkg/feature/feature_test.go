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

package feature

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEnable(t *testing.T) {
	var cool Flag = "cool"

	t.Run("EnableMutatesZeroValue", func(t *testing.T) {
		f := &Flags{}
		f.Enable(cool)

		want := true
		got := f.Enabled(cool)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("f.Enabled(...): -want, +got:\n%s", diff)
		}
	})

	t.Run("EnabledOnEmptyFlagsReturnsFalse", func(t *testing.T) {
		f := &Flags{}

		want := false
		got := f.Enabled(cool)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("f.Enabled(...): -want, +got:\n%s", diff)
		}
	})

	t.Run("EnabledOnNilReturnsFalse", func(t *testing.T) {
		var f *Flags

		want := false
		got := f.Enabled(cool)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("f.Enabled(...): -want, +got:\n%s", diff)
		}
	})
}
