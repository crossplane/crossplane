package renderer

import (
	"bytes"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
	"testing"
)

func TestDefaultDiffRenderer_RenderDiffs(t *testing.T) {
	// Create test diffs
	addedDiff := &ResourceDiff{
		Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "TestResource"},
		ResourceName: "added-resource",
		DiffType:     DiffTypeAdded,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffInsert, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: added-resource\nspec:\n  field: value"},
		},
	}

	modifiedDiff := &ResourceDiff{
		Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "TestResource"},
		ResourceName: "modified-resource",
		DiffType:     DiffTypeModified,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffEqual, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: modified-resource\n"},
			{Type: diffmatchpatch.DiffDelete, Text: "spec:\n  field: old-value"},
			{Type: diffmatchpatch.DiffInsert, Text: "spec:\n  field: new-value"},
		},
	}

	removedDiff := &ResourceDiff{
		Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "TestResource"},
		ResourceName: "removed-resource",
		DiffType:     DiffTypeRemoved,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffDelete, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: removed-resource\nspec:\n  field: value"},
		},
	}

	equalDiff := &ResourceDiff{
		Gvk:          schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "TestResource"},
		ResourceName: "equal-resource",
		DiffType:     DiffTypeEqual,
		LineDiffs:    []diffmatchpatch.Diff{},
	}

	tests := map[string]struct {
		diffs           map[string]*ResourceDiff
		options         DiffOptions
		expectedOutputs []string
		notExpected     []string
	}{
		"RenderAllDiffTypes": {
			diffs: map[string]*ResourceDiff{
				addedDiff.GetDiffKey():    addedDiff,
				modifiedDiff.GetDiffKey(): modifiedDiff,
				removedDiff.GetDiffKey():  removedDiff,
				equalDiff.GetDiffKey():    equalDiff,
			},
			options: DiffOptions{
				UseColors:      false,
				AddPrefix:      "+ ",
				DeletePrefix:   "- ",
				ContextPrefix:  "  ",
				ContextLines:   3,
				ChunkSeparator: "...",
				Compact:        false,
			},
			expectedOutputs: []string{
				"+++ TestResource/added-resource",
				"--- TestResource/removed-resource",
				"~~~ TestResource/modified-resource",
				"+ apiVersion: example.org/v1",
				"- spec:",
				"-   field: old-value",
				"+ spec:",
				"+   field: new-value",
			},
			notExpected: []string{
				"TestResource/equal-resource", // Equal resources should not be rendered
			},
		},
		"CompactMode": {
			diffs: map[string]*ResourceDiff{
				modifiedDiff.GetDiffKey(): modifiedDiff,
			},
			options: DiffOptions{
				UseColors:      false,
				AddPrefix:      "+ ",
				DeletePrefix:   "- ",
				ContextPrefix:  "  ",
				ContextLines:   1, // Fewer context lines for compact mode
				ChunkSeparator: "...",
				Compact:        true,
			},
			expectedOutputs: []string{
				"~~~ TestResource/modified-resource",
				"- spec:",
				"-   field: old-value",
				"+ spec:",
				"+   field: new-value",
			},
			notExpected: []string{
				"  apiVersion: example.org/v1", // Should be omitted due to context line limit
				"  metadata:",
			},
		},
		"EmptyDiffs": {
			diffs: map[string]*ResourceDiff{},
			options: DiffOptions{
				UseColors:      false,
				AddPrefix:      "+ ",
				DeletePrefix:   "- ",
				ContextPrefix:  "  ",
				ContextLines:   3,
				ChunkSeparator: "...",
				Compact:        false,
			},
			expectedOutputs: []string{},
		},
		"OnlyEqualDiffs": {
			diffs: map[string]*ResourceDiff{
				equalDiff.GetDiffKey(): equalDiff,
			},
			options: DiffOptions{
				UseColors:      false,
				AddPrefix:      "+ ",
				DeletePrefix:   "- ",
				ContextPrefix:  "  ",
				ContextLines:   3,
				ChunkSeparator: "...",
				Compact:        false,
			},
			expectedOutputs: []string{},
			notExpected:     []string{"TestResource/equal-resource"},
		},
		"SummaryOutput": {
			diffs: map[string]*ResourceDiff{
				addedDiff.GetDiffKey():    addedDiff,
				modifiedDiff.GetDiffKey(): modifiedDiff,
				removedDiff.GetDiffKey():  removedDiff,
			},
			options: DiffOptions{
				UseColors:      false,
				AddPrefix:      "+ ",
				DeletePrefix:   "- ",
				ContextPrefix:  "  ",
				ContextLines:   3,
				ChunkSeparator: "...",
				Compact:        false,
			},
			expectedOutputs: []string{
				"Summary:", "1 added", "1 modified", "1 removed",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			logger := tu.TestLogger(t, false)

			// Create a renderer
			renderer := NewDiffRenderer(logger, tt.options)

			// Create a buffer to capture output
			var buffer bytes.Buffer

			// Call the method under test
			err := renderer.RenderDiffs(&buffer, tt.diffs)
			if err != nil {
				t.Fatalf("RenderDiffs() failed with error: %v", err)
			}

			// Get the output as a string
			output := buffer.String()

			// Check for expected output
			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q but it didn't\nOutput: %s", expected, output)
				}
			}

			// Check for things that should not be in the output
			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("Output should not contain %q but it did\nOutput: %s", notExpected, output)
				}
			}
		})
	}
}

func TestGetLineDiff(t *testing.T) {
	tests := map[string]struct {
		oldText  string
		newText  string
		expected []diffmatchpatch.Operation
	}{
		"NoChanges": {
			oldText: "line1\nline2\nline3\n",
			newText: "line1\nline2\nline3\n",
			expected: []diffmatchpatch.Operation{
				diffmatchpatch.DiffEqual,
			},
		},
		"LineAdded": {
			oldText: "line1\nline2\n",
			newText: "line1\nline2\nline3\n",
			expected: []diffmatchpatch.Operation{
				diffmatchpatch.DiffEqual,
				diffmatchpatch.DiffInsert,
			},
		},
		"LineRemoved": {
			oldText: "line1\nline2\nline3\n",
			newText: "line1\nline3\n",
			expected: []diffmatchpatch.Operation{
				diffmatchpatch.DiffEqual,
				diffmatchpatch.DiffDelete,
				diffmatchpatch.DiffEqual,
			},
		},
		"LineModified": {
			oldText: "line1\nline2\nline3\n",
			newText: "line1\nmodified2\nline3\n",
			expected: []diffmatchpatch.Operation{
				diffmatchpatch.DiffEqual,
				diffmatchpatch.DiffDelete,
				diffmatchpatch.DiffInsert,
				diffmatchpatch.DiffEqual,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := GetLineDiff(tt.oldText, tt.newText)

			// Check that we have the expected diff types
			if len(result) != len(tt.expected) {
				t.Errorf("GetLineDiff() returned %d diffs, want %d", len(result), len(tt.expected))
				for i, diff := range result {
					t.Logf("Diff %d: Type=%s, Text=%q", i, diff.Type, diff.Text)
				}
				return
			}

			// Verify the types match in sequence
			for i, expectedType := range tt.expected {
				if result[i].Type != expectedType {
					t.Errorf("GetLineDiff() diff[%d] has type %s, want %s", i, result[i].Type, expectedType)
				}
			}
		})
	}
}
