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

package xpkg

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestToYAML(t *testing.T) {
	t.Parallel()
	t.Run("Slice", func(t *testing.T) {
		t.Parallel()
		got, err := toYAML([]any{"compute", "networking"})
		if err != nil {
			t.Fatalf("toYAML: %v", err)
		}
		want := "- compute\n- networking"
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("toYAML: -want +got:\n%s", diff)
		}
	})
	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		got, err := toYAML(nil)
		if err != nil {
			t.Fatalf("toYAML: %v", err)
		}
		if diff := cmp.Diff("null", got); diff != "" {
			t.Fatalf("toYAML(nil): -want +got:\n%s", diff)
		}
	})
}

func TestDict(t *testing.T) {
	t.Parallel()
	got, err := dict("a", 1, "b", "x")
	if err != nil {
		t.Fatalf("dict: %v", err)
	}
	want := map[string]any{"a": 1, "b": "x"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("dict: -want +got:\n%s", diff)
	}
}

func TestDictErrors(t *testing.T) {
	t.Parallel()
	t.Run("OddNumberOfArgs", func(t *testing.T) {
		t.Parallel()
		_, err := dict("a", 1, "b")
		if err == nil {
			t.Fatal("dict: want error for odd argument count")
		}
		want := "dict: requires an even number of arguments"
		if diff := cmp.Diff(want, err.Error()); diff != "" {
			t.Fatalf("dict: -want +got error string:\n%s", diff)
		}
	})
	t.Run("NonStringKey", func(t *testing.T) {
		t.Parallel()
		_, err := dict(1, "v")
		if err == nil {
			t.Fatal("dict: want error for non-string key")
		}
		want := "dict: argument 0 must be a string key"
		if diff := cmp.Diff(want, err.Error()); diff != "" {
			t.Fatalf("dict: -want +got error string:\n%s", diff)
		}
	})
}

func TestIndent(t *testing.T) {
	t.Parallel()
	got := indent(4, "a\nb")
	want := "    a\n    b"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("indent: -want +got:\n%s", diff)
	}
}

func TestIndentToYAMLMatchesProviderPattern(t *testing.T) {
	t.Parallel()
	fragment, err := toYAML([]any{"compute", "virtual-machines"})
	if err != nil {
		t.Fatalf("toYAML: %v", err)
	}
	got := indent(6, fragment)
	wantLines := []string{
		"      - compute",
		"      - virtual-machines",
	}
	if diff := cmp.Diff(strings.Join(wantLines, "\n"), got); diff != "" {
		t.Fatalf("indent(toYAML(slice)): -want +got:\n%s", diff)
	}
}
