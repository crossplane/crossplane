package diffprocessor

import (
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenerateDiff(t *testing.T) {
	// Create test resources
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
	// and also because DeepCopy throws an error on anything besides int64, and we use it inside the UUT
	noChanges := current.DeepCopy()

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
}

func TestGetLineDiff(t *testing.T) {
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
			name:       "Full Diff No Changes",
			oldText:    "line1\nline2\nline3",
			newText:    "line1\nline2\nline3",
			options:    DefaultDiffOptions(),
			expectDiff: true,
			contains:   []string{"  line1", "  line2", "  line3"},
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
			// For non-colored tests, disable colors to make comparison easier
			hasColorCodes := false
			for _, s := range tt.contains {
				if s == ColorRed || s == ColorGreen || s == ColorReset {
					hasColorCodes = true
					break
				}
			}
			if !hasColorCodes {
				tt.options.UseColors = false
			}

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
}

func TestFullDiffFormatter_Format(t *testing.T) {
	// Create test diffs
	diffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "unchanged line\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "another unchanged line\n"},
	}

	tests := []struct {
		name     string
		diffs    []diffmatchpatch.Diff
		options  DiffOptions
		expected string
	}{
		{
			name:  "Basic Formatting",
			diffs: diffs,
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				return opts
			}(),
			expected: "  unchanged line\n- deleted line\n+ inserted line\n  another unchanged line\n",
		},
		{
			name:  "With Custom Prefixes",
			diffs: diffs,
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				opts.AddPrefix = "ADD "
				opts.DeletePrefix = "DEL "
				opts.ContextPrefix = "CTX "
				return opts
			}(),
			expected: "CTX unchanged line\nDEL deleted line\nADD inserted line\nCTX another unchanged line\n",
		},
		{
			name: "With Empty Line",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffEqual, Text: "line before\n"},
				{Type: diffmatchpatch.DiffDelete, Text: "\n"},
				{Type: diffmatchpatch.DiffInsert, Text: "new content\n"},
			},
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				return opts
			}(),
			expected: "  line before\n- \n+ new content\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := &FullDiffFormatter{}
			result := formatter.Format(tt.diffs, tt.options)

			if result != tt.expected {
				t.Errorf("FullDiffFormatter.Format() got:\n%s\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestCompactDiffFormatter_Format(t *testing.T) {
	// Create test diffs with multiple sections and a clear gap between changes
	diffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "context line 1\ncontext line 2\ncontext line 3\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line 1\ndeleted line 2\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line 1\ninserted line 2\n"},
		// Increase the number of context lines between change blocks
		{Type: diffmatchpatch.DiffEqual, Text: "context line 4\ncontext line 5\ncontext line 6\ncontext line 7\ncontext line 8\ncontext line 9\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "another deleted\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "another inserted\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "context line 10\ncontext line 11\n"},
	}

	tests := []struct {
		name           string
		diffs          []diffmatchpatch.Diff
		options        DiffOptions
		expectedLines  []string // Exact lines that should be in the output
		forbiddenLines []string // Exact lines that should NOT be in the output
	}{
		{
			name:  "Basic Compact Formatting",
			diffs: diffs,
			options: func() DiffOptions {
				opts := CompactDiffOptions()
				opts.UseColors = false
				opts.ContextLines = 2 // Set context lines for predictable output
				return opts
			}(),
			expectedLines: []string{
				// Note: Using the exact spacing from the formatter output
				"  context line 2",
				"  context line 3",
				"- deleted line 1",
				"- deleted line 2",
				"+ inserted line 1",
				"+ inserted line 2",
				"  context line 4",
				"  context line 5",
				"...",
				"  context line 8",
				"  context line 9",
				"- another deleted",
				"+ another inserted",
				"  context line 10",
				"  context line 11",
			},
			forbiddenLines: []string{
				"  context line 1", // This should be omitted as it's beyond the context limit
				"  context line 6",
				"  context line 7", // These should be omitted and replaced with the separator
			},
		},
		{
			name:  "Zero Context Lines",
			diffs: diffs,
			options: func() DiffOptions {
				opts := CompactDiffOptions()
				opts.UseColors = false
				opts.ContextLines = 0 // No context lines
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
			forbiddenLines: []string{
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
				"  context line 11", // No context lines should be included
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail := false
			formatter := &CompactDiffFormatter{}
			result := formatter.Format(tt.diffs, tt.options)

			// Log the actual result for debugging
			resultLines := strings.Split(result, "\n")

			// Remove any trailing empty line which might appear from a final newline
			if len(resultLines) > 0 && resultLines[len(resultLines)-1] == "" {
				resultLines = resultLines[:len(resultLines)-1]
			}

			// Check for expected lines with exact format
			for _, expectedLine := range tt.expectedLines {
				foundLine := false
				for _, actualLine := range resultLines {
					if actualLine == expectedLine {
						foundLine = true
						break
					}
				}

				if !foundLine {

					// For better debugging, try to find a close match
					t.Logf("Expected line not found: %q", expectedLine)
					fail = true
					for _, actualLine := range resultLines {
						if strings.Contains(actualLine, expectedLine[2:]) { // Skip prefix spaces for content check
							t.Logf("  Similar line found: %q", actualLine)
						}
					}
				}
			}

			// Check for lines that should not be present
			for _, forbiddenLine := range tt.forbiddenLines {
				for _, actualLine := range resultLines {
					if actualLine == forbiddenLine {
						t.Logf("Unexpected line found: %q", forbiddenLine)
						fail = true
						break
					}
				}
			}

			if fail {
				// print the actual result for debugging in case of failure
				t.Logf("Actual result with line numbers:")
				for i, line := range resultLines {
					t.Logf("%d: %q", i, line)
				}

				t.Fail()
			}
		})
	}
}

func TestNewFormatter(t *testing.T) {
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
}
