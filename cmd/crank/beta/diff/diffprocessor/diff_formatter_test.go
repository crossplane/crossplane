package diffprocessor

import (
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiffFormatting(t *testing.T) {
	// Test resources for GenerateDiff tests
	current := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"name":            "test-resource",
				"resourceVersion": "1",
			},
			"spec": map[string]interface{}{
				"field1": "old-value",
				"field2": int64(123),
				"field3": []interface{}{
					"item1",
					"item2",
				},
			},
		},
	}

	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.org/v1",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"name":            "test-resource",
				"resourceVersion": "1",
			},
			"spec": map[string]interface{}{
				"field1": "new-value",
				"field2": int64(456),
				"field3": []interface{}{
					"item1",
					"item3",
				},
				"field4": "added-field",
			},
		},
	}

	// Identical to current, for no-change test
	noChanges := current.DeepCopy()

	// Generic diffs for formatter tests
	simpleDiffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "unchanged line\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "another unchanged line\n"},
	}

	// Long diffs with multiple blocks for compact formatter
	longDiffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "context line 1\ncontext line 2\ncontext line 3\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line 1\ndeleted line 2\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line 1\ninserted line 2\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "context line 4\ncontext line 5\ncontext line 6\ncontext line 7\ncontext line 8\ncontext line 9\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "another deleted\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "another inserted\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "context line 10\ncontext line 11\n"},
	}

	t.Run("GenerateDiff", func(t *testing.T) {
		tests := []struct {
			name         string
			current      *unstructured.Unstructured
			desired      *unstructured.Unstructured
			kind         string
			resourceName string
			expectDiff   bool
			expectPrefix string
		}{
			{
				name:         "With Changes",
				current:      current,
				desired:      desired,
				kind:         "TestResource",
				resourceName: "test-resource",
				expectDiff:   true,
				expectPrefix: "~~~ TestResource/test-resource",
			},
			{
				name:         "No Changes",
				current:      current,
				desired:      noChanges,
				kind:         "TestResource",
				resourceName: "test-resource",
				expectDiff:   false,
			},
			{
				name:         "New Resource",
				current:      nil,
				desired:      desired,
				kind:         "TestResource",
				resourceName: "test-resource",
				expectDiff:   true,
				expectPrefix: "+++ TestResource/test-resource",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				diff, err := GenerateDiff(tt.current, tt.desired, tt.kind, tt.resourceName)

				// Check for errors
				if err != nil {
					t.Fatalf("GenerateDiff() returned error: %v", err)
				}

				// For no changes, expect empty diff
				if !tt.expectDiff && diff != "" {
					t.Errorf("GenerateDiff() returned diff for identical resources: %s", diff)
				}

				// For expected changes, verify the diff has content and the correct prefix
				if tt.expectDiff {
					if diff == "" {
						t.Error("GenerateDiff() returned empty diff when changes were expected")
					}

					if !strings.HasPrefix(diff, tt.expectPrefix) {
						t.Errorf("GenerateDiff() returned diff with incorrect prefix, got: %s, want prefix: %s", diff, tt.expectPrefix)
					}
				}
			})
		}
	})

	t.Run("Line Diff Generation", func(t *testing.T) {
		tests := []struct {
			name        string
			oldText     string
			newText     string
			options     DiffOptions
			expectDiff  bool
			contains    []string
			notContains []string
		}{
			{
				name:       "Full Diff With Changes",
				oldText:    "line1\nline2\nline3",
				newText:    "line1\nmodified\nline3",
				options:    DefaultDiffOptions(),
				expectDiff: true,
				contains:   []string{"- line2", "+ modified"},
			},
			{
				name:       "Compact Diff",
				oldText:    "line1\nline2\nline3\nline4\nline5",
				newText:    "line1\nmodified\nline3\nline4\nchanged",
				options:    CompactDiffOptions(),
				expectDiff: true,
				contains:   []string{"- line2", "+ modified", "- line5", "+ changed"},
			},
			{
				name:       "Empty Text",
				oldText:    "",
				newText:    "",
				options:    DefaultDiffOptions(),
				expectDiff: false,
			},
			{
				name:       "Trailing Newlines",
				oldText:    "line1\nline2\n",
				newText:    "line1\nline2\nline3\n",
				options:    DefaultDiffOptions(),
				expectDiff: true,
				contains:   []string{"+ line3"},
			},
			{
				name:    "Colors Enabled",
				oldText: "original",
				newText: "modified",
				options: func() DiffOptions {
					opts := DefaultDiffOptions()
					opts.UseColors = true
					return opts
				}(),
				expectDiff: true,
				contains:   []string{ColorRed, ColorGreen, ColorReset},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				diff := GetLineDiff(tt.oldText, tt.newText, tt.options)

				if tt.expectDiff && diff == "" {
					t.Error("GetLineDiff() returned empty diff when changes were expected")
				}

				if !tt.expectDiff && diff != "" {
					t.Errorf("GetLineDiff() returned diff when no changes were expected: %s", diff)
				}

				// Check for expected content
				for _, s := range tt.contains {
					if !strings.Contains(diff, s) {
						t.Errorf("GetLineDiff() result missing expected content: %q", s)
					}
				}

				// Check for content that should not be present
				for _, s := range tt.notContains {
					if strings.Contains(diff, s) {
						t.Errorf("GetLineDiff() result contains unexpected content: %q", s)
					}
				}
			})
		}
	})

	t.Run("Diff Formatters", func(t *testing.T) {
		tests := []struct {
			name          string
			diffs         []diffmatchpatch.Diff
			options       DiffOptions
			compact       bool
			expectedLines []string
			avoidLines    []string
		}{
			{
				name:    "Full Formatter Basic",
				diffs:   simpleDiffs,
				compact: false,
				options: func() DiffOptions {
					opts := DefaultDiffOptions()
					opts.UseColors = false
					return opts
				}(),
				expectedLines: []string{
					"  unchanged line",
					"- deleted line",
					"+ inserted line",
					"  another unchanged line",
				},
			},
			{
				name:    "Compact Formatter Basic",
				diffs:   longDiffs,
				compact: true,
				options: func() DiffOptions {
					opts := CompactDiffOptions()
					opts.UseColors = false
					opts.ContextLines = 2
					return opts
				}(),
				expectedLines: []string{
					"  context line 2",
					"  context line 3",
					"- deleted line 1",
					"- deleted line 2",
					"+ inserted line 1",
					"+ inserted line 2",
					"  context line 4",
					"  context line 5",
					"...", // Separator
					"  context line 8",
					"  context line 9",
					"- another deleted",
					"+ another inserted",
					"  context line 10",
					"  context line 11",
				},
				avoidLines: []string{
					"  context line 1", // Should be omitted as it's beyond context limit
					"  context line 6", // Should be replaced with separator
					"  context line 7",
				},
			},
			{
				name:    "Zero Context Lines",
				diffs:   longDiffs,
				compact: true,
				options: func() DiffOptions {
					opts := CompactDiffOptions()
					opts.UseColors = false
					opts.ContextLines = 0
					return opts
				}(),
				expectedLines: []string{
					"- deleted line 1",
					"- deleted line 2",
					"+ inserted line 1",
					"+ inserted line 2",
					"...", // Separator
					"- another deleted",
					"+ another inserted",
				},
				avoidLines: []string{
					"  context line 1",
					"  context line 2",
					"  context line 3",
					"  context line 4",
					"  context line 5",
					"  context line 6",
					"  context line 7",
					"  context line 8",
					"  context line 9",
					"  context line 10",
					"  context line 11",
				},
			},
			{
				name:    "Custom Prefixes",
				diffs:   simpleDiffs,
				compact: false,
				options: func() DiffOptions {
					opts := DefaultDiffOptions()
					opts.UseColors = false
					opts.AddPrefix = "ADD "
					opts.DeletePrefix = "DEL "
					opts.ContextPrefix = "CTX "
					return opts
				}(),
				expectedLines: []string{
					"CTX unchanged line",
					"DEL deleted line",
					"ADD inserted line",
					"CTX another unchanged line",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Create the right formatter based on compact flag
				formatter := NewFormatter(tt.compact)
				result := formatter.Format(tt.diffs, tt.options)

				// Check for expected lines
				for _, expectedLine := range tt.expectedLines {
					if !strings.Contains(result, expectedLine) {
						t.Errorf("Formatter.Format() missing expected line: %q", expectedLine)
					}
				}

				// Check for lines that should be avoided
				for _, avoidLine := range tt.avoidLines {
					resultLines := strings.Split(result, "\n")
					for _, line := range resultLines {
						if line == avoidLine {
							t.Errorf("Formatter.Format() contained line that should be omitted: %q", avoidLine)
							break
						}
					}
				}
			})
		}
	})

	t.Run("Formatter Factory", func(t *testing.T) {
		tests := []struct {
			name                 string
			compact              bool
			wantFullFormatter    bool
			wantCompactFormatter bool
		}{
			{
				name:                 "Full Formatter",
				compact:              false,
				wantFullFormatter:    true,
				wantCompactFormatter: false,
			},
			{
				name:                 "Compact Formatter",
				compact:              true,
				wantFullFormatter:    false,
				wantCompactFormatter: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				formatter := NewFormatter(tt.compact)

				// Check if it's the right type using type assertions
				_, isFullFormatter := formatter.(*FullDiffFormatter)
				_, isCompactFormatter := formatter.(*CompactDiffFormatter)

				if tt.wantFullFormatter && !isFullFormatter {
					t.Errorf("NewFormatter(%v) did not return a *FullDiffFormatter", tt.compact)
				}

				if tt.wantCompactFormatter && !isCompactFormatter {
					t.Errorf("NewFormatter(%v) did not return a *CompactDiffFormatter", tt.compact)
				}
			})
		}
	})
}
