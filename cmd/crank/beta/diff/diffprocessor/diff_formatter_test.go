package diffprocessor

import (
	"bytes"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/google/go-cmp/cmp"
	"github.com/sergi/go-diff/diffmatchpatch"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenerateDiffWithOptions(t *testing.T) {
	// Create test resources for diffing
	current := tu.NewResource("example.org/v1", "TestResource", "test-resource").
		WithSpecField("field1", "old-value").
		WithSpecField("field2", int64(123)).
		Build()

	desired := tu.NewResource("example.org/v1", "TestResource", "test-resource").
		WithSpecField("field1", "new-value").
		WithSpecField("field2", int64(456)).
		WithSpecField("field4", "added-field").
		Build()

	// Identical to current, for no-change test
	noChanges := current.DeepCopy()

	tests := []struct {
		name     string
		current  *unstructured.Unstructured
		desired  *unstructured.Unstructured
		kind     string
		resName  string
		options  DiffOptions
		wantDiff *ResourceDiff
		wantNil  bool
	}{
		{
			name:    "ModifiedResource",
			current: current,
			desired: desired,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &ResourceDiff{
				ResourceKind: "TestResource",
				ResourceName: "test-resource",
				DiffType:     DiffTypeModified,
				Current:      current,
				Desired:      desired,
				// LineDiffs will be checked separately
			},
		},
		{
			name:    "NoChanges",
			current: current,
			desired: noChanges,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &ResourceDiff{
				ResourceKind: "TestResource",
				ResourceName: "test-resource",
				DiffType:     DiffTypeEqual,
				Current:      current,
				Desired:      noChanges,
			},
		},
		{
			name:    "NewResource",
			current: nil,
			desired: desired,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &ResourceDiff{
				ResourceKind: "TestResource",
				ResourceName: "test-resource",
				DiffType:     DiffTypeAdded,
				Current:      nil,
				Desired:      desired,
				// LineDiffs will be checked separately
			},
		},
		{
			name:    "RemovedResource",
			current: current,
			desired: nil,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &ResourceDiff{
				ResourceKind: "TestResource",
				ResourceName: "test-resource",
				DiffType:     DiffTypeRemoved,
				Current:      current,
				Desired:      nil,
				// LineDiffs will be checked separately
			},
		},
		//		{
		// TODO:  this should check for an error.  illegal condition.
		//name:    "BothNil",
		//current: nil,
		//desired: nil,
		//kind:    "TestResource",
		//resName: "test-resource",
		//options: DefaultDiffOptions(),
		//wantNil: true,
		//		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := GenerateDiffWithOptions(tt.current, tt.desired, tu.TestLogger(t), tt.options)

			if err != nil {
				t.Fatalf("GenerateDiffWithOptions() returned error: %v", err)
			}

			if tt.wantNil {
				if diff != nil {
					t.Errorf("GenerateDiffWithOptions() = %v, want nil", diff)
				}
				return
			}

			if diff == nil {
				t.Fatalf("GenerateDiffWithOptions() returned nil, want non-nil")
			}

			// Check the basic properties
			if diff.ResourceKind != tt.wantDiff.ResourceKind {
				t.Errorf("ResourceKind = %v, want %v", diff.ResourceKind, tt.wantDiff.ResourceKind)
			}

			if diff.ResourceName != tt.wantDiff.ResourceName {
				t.Errorf("ResourceName = %v, want %v", diff.ResourceName, tt.wantDiff.ResourceName)
			}

			if diff.DiffType != tt.wantDiff.DiffType {
				t.Errorf("DiffType = %v, want %v", diff.DiffType, tt.wantDiff.DiffType)
			}

			// Check for line diffs - should be non-empty for changed resources
			if diff.DiffType != DiffTypeEqual && len(diff.LineDiffs) == 0 {
				t.Errorf("LineDiffs is empty for %s", tt.name)
			}

			// Check Current and Desired references
			if diff.Current != tt.wantDiff.Current && !cmp.Equal(diff.Current, tt.wantDiff.Current) {
				t.Errorf("Current resource doesn't match expected")
			}

			if diff.Desired != tt.wantDiff.Desired && !cmp.Equal(diff.Desired, tt.wantDiff.Desired) {
				t.Errorf("Desired resource doesn't match expected")
			}
		})
	}
}

func TestRenderDiffs(t *testing.T) {
	// Test diffs for different scenarios
	addedDiff := &ResourceDiff{
		ResourceKind: "TestResource",
		ResourceName: "added-resource",
		DiffType:     DiffTypeAdded,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffInsert, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: added-resource\nspec:\n  param: value\n"},
		},
	}

	removedDiff := &ResourceDiff{
		ResourceKind: "TestResource",
		ResourceName: "removed-resource",
		DiffType:     DiffTypeRemoved,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffDelete, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: removed-resource\nspec:\n  param: value\n"},
		},
	}

	modifiedDiff := &ResourceDiff{
		ResourceKind: "TestResource",
		ResourceName: "modified-resource",
		DiffType:     DiffTypeModified,
		LineDiffs: []diffmatchpatch.Diff{
			{Type: diffmatchpatch.DiffEqual, Text: "apiVersion: example.org/v1\nkind: TestResource\nmetadata:\n  name: modified-resource\nspec:\n"},
			{Type: diffmatchpatch.DiffDelete, Text: "  param: old-value\n"},
			{Type: diffmatchpatch.DiffInsert, Text: "  param: new-value\n"},
		},
	}

	// Create test cases
	tests := []struct {
		name      string
		diffs     map[string]*ResourceDiff
		options   DiffOptions
		wantLines []string
	}{
		{
			name:      "EmptyDiffs",
			diffs:     map[string]*ResourceDiff{},
			options:   DefaultDiffOptions(),
			wantLines: []string{},
		},
		{
			name: "AddedResource",
			diffs: map[string]*ResourceDiff{
				"TestResource/added-resource": addedDiff,
			},
			options: DefaultDiffOptions(),
			wantLines: []string{
				"+++ TestResource/added-resource",
				tu.Green("+ apiVersion: example.org/v1"),
				tu.Green("+ kind: TestResource"),
				tu.Green("+ metadata:"),
				tu.Green("+   name: added-resource"),
				tu.Green("+ spec:"),
				tu.Green("+   param: value"),
				"---",
			},
		},
		{
			name: "RemovedResource",
			diffs: map[string]*ResourceDiff{
				"TestResource/removed-resource": removedDiff,
			},
			options: DefaultDiffOptions(),
			wantLines: []string{
				"--- TestResource/removed-resource",
				tu.Red("- apiVersion: example.org/v1"),
				tu.Red("- kind: TestResource"),
				tu.Red("- metadata:"),
				tu.Red("-   name: removed-resource"),
				tu.Red("- spec:"),
				tu.Red("-   param: value"),
				"---",
			},
		},
		{
			name: "ModifiedResource",
			diffs: map[string]*ResourceDiff{
				"TestResource/modified-resource": modifiedDiff,
			},
			options: DefaultDiffOptions(),
			wantLines: []string{
				"~~~ TestResource/modified-resource",
				"apiVersion: example.org/v1",
				"kind: TestResource",
				"metadata:",
				"  name: modified-resource",
				"spec:",
				tu.Red("-   param: old-value"),
				tu.Green("+   param: new-value"),
				"---",
			},
		},
		{
			name: "MultipleDiffs",
			diffs: map[string]*ResourceDiff{
				"TestResource/added-resource":   addedDiff,
				"TestResource/removed-resource": removedDiff,
			},
			options: DefaultDiffOptions(),
			wantLines: []string{
				"+++ TestResource/added-resource",
				"--- TestResource/removed-resource",
			},
		},
		{
			name: "NoColor",
			diffs: map[string]*ResourceDiff{
				"TestResource/modified-resource": modifiedDiff,
			},
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				return opts
			}(),
			wantLines: []string{
				"~~~ TestResource/modified-resource",
				"  apiVersion: example.org/v1",
				"  kind: TestResource",
				"  metadata:",
				"    name: modified-resource",
				"  spec:",
				"-   param: old-value",
				"+   param: new-value",
				"---",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the output
			var stdout bytes.Buffer

			// Create a processor with the right configuration
			p := &DefaultDiffProcessor{
				config: ProcessorConfig{
					Colorize: tt.options.UseColors,
					Compact:  tt.options.Compact,
					Logger:   tu.TestLogger(t),
				},
			}

			// Call the method under test
			err := p.RenderDiffs(&stdout, tt.diffs)
			if err != nil {
				t.Fatalf("RenderDiffs() error = %v", err)
			}

			// Get the actual output
			output := stdout.String()

			// Track failures
			failures := 0

			// Since the output may have ANSI color codes and platform-specific line endings,
			// let's check if each expected line is part of the output
			for _, line := range tt.wantLines {
				// Skip empty lines in want as they're common and could be misleading
				if strings.TrimSpace(line) == "" {
					continue
				}

				if !strings.Contains(output, line) {
					failures++

					// Log the failure details using t.Logf
					if tu.CompareIgnoringAnsi(output, line) {
						t.Logf("Line %d: ANSI codes differ - want: %q", failures, line)
					} else {
						t.Logf("Line %d: Missing expected content - want: %q", failures, line)
					}
				}
			}

			// If we had any failures, print the full output once
			if failures > 0 {
				t.Errorf("RenderDiffs() had %d line failures. Full output:\n%s",
					failures, output)
			}
		})
	}
}

func TestFormatDiff(t *testing.T) {
	// Create test diffs
	simpleDiffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "unchanged line\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "another unchanged line\n"},
	}

	// Create test cases
	tests := []struct {
		name     string
		diffs    []diffmatchpatch.Diff
		options  DiffOptions
		contains []string
		excludes []string
	}{
		{
			name:     "EmptyDiffs",
			diffs:    []diffmatchpatch.Diff{},
			options:  DefaultDiffOptions(),
			contains: []string{},
			excludes: []string{"unchanged", "deleted", "inserted"},
		},
		{
			name:    "StandardFormatting",
			diffs:   simpleDiffs,
			options: DefaultDiffOptions(),
			contains: []string{
				"unchanged line",
				"deleted line",
				"inserted line",
				"another unchanged line",
			},
		},
		{
			name:  "WithoutColors",
			diffs: simpleDiffs,
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				return opts
			}(),
			contains: []string{
				"  unchanged line",
				"- deleted line",
				"+ inserted line",
				"  another unchanged line",
			},
			excludes: []string{
				"\x1b[31m", // Red color code
				"\x1b[32m", // Green color code
			},
		},
		{
			name:  "WithColors",
			diffs: simpleDiffs,
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = true
				return opts
			}(),
			contains: []string{
				"unchanged line",
				"deleted line",
				"inserted line",
			},
		},
		{
			name: "CompactFormat",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffEqual, Text: "context line 1\ncontext line 2\ncontext line 3\n"},
				{Type: diffmatchpatch.DiffDelete, Text: "deleted line 1\ndeleted line 2\n"},
				{Type: diffmatchpatch.DiffInsert, Text: "inserted line 1\ninserted line 2\n"},
				{Type: diffmatchpatch.DiffEqual, Text: "context line 4\ncontext line 5\ncontext line 6\n"},
			},
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.Compact = true
				opts.ContextLines = 1
				return opts
			}(),
			contains: []string{
				"context line 3",
				"deleted line 1",
				"deleted line 2",
				"inserted line 1",
				"inserted line 2",
				"context line 4",
			},
			excludes: []string{
				"context line 1",
				"context line 2",
				"context line 5",
				"context line 6",
			},
		},
		{
			name:  "CustomPrefixes",
			diffs: simpleDiffs,
			options: func() DiffOptions {
				opts := DefaultDiffOptions()
				opts.UseColors = false
				opts.AddPrefix = "ADD "
				opts.DeletePrefix = "DEL "
				opts.ContextPrefix = "CTX "
				return opts
			}(),
			contains: []string{
				"CTX unchanged line",
				"DEL deleted line",
				"ADD inserted line",
				"CTX another unchanged line",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Format the diff
			result := FormatDiff(tt.diffs, tt.options)

			// Check that the result contains expected substrings
			for _, expected := range tt.contains {
				if expected == "" {
					continue
				}
				if !strings.Contains(result, expected) {
					t.Errorf("FormatDiff() result missing expected content: %q", expected)
				}
			}

			// Check that the result excludes certain substrings
			for _, excluded := range tt.excludes {
				if excluded == "" {
					continue
				}
				if strings.Contains(result, excluded) {
					t.Errorf("FormatDiff() result contains unexpected content: %q", excluded)
				}
			}
		})
	}
}
