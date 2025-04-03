package renderer

import (
	types "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/google/go-cmp/cmp"
	"github.com/sergi/go-diff/diffmatchpatch"
	"strings"
	"testing"

	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	tests := map[string]struct {
		current  *un.Unstructured
		desired  *un.Unstructured
		kind     string
		resName  string
		options  DiffOptions
		wantDiff *types.ResourceDiff
		wantErr  bool
	}{
		"ModifiedResource": {
			current: current,
			desired: desired,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &types.ResourceDiff{
				Gvk:          current.GroupVersionKind(),
				ResourceName: "test-resource",
				DiffType:     types.DiffTypeModified,
				Current:      current,
				Desired:      desired,
				// LineDiffs will be checked separately
			},
		},
		"NoChanges": {
			current: current,
			desired: noChanges,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &types.ResourceDiff{
				Gvk:          current.GroupVersionKind(),
				ResourceName: "test-resource",
				DiffType:     types.DiffTypeEqual,
				Current:      current,
				Desired:      noChanges,
			},
		},
		"NewResource": {
			current: nil,
			desired: desired,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &types.ResourceDiff{
				Gvk:          desired.GroupVersionKind(),
				ResourceName: "test-resource",
				DiffType:     types.DiffTypeAdded,
				Current:      nil,
				Desired:      desired,
				// LineDiffs will be checked separately
			},
		},
		"RemovedResource": {
			current: current,
			desired: nil,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantDiff: &types.ResourceDiff{
				Gvk:          current.GroupVersionKind(),
				ResourceName: "test-resource",
				DiffType:     types.DiffTypeRemoved,
				Current:      current,
				Desired:      nil,
				// LineDiffs will be checked separately
			},
		},
		"BothNil": {
			current: nil,
			desired: nil,
			kind:    "TestResource",
			resName: "test-resource",
			options: DefaultDiffOptions(),
			wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diff, err := GenerateDiffWithOptions(tt.current, tt.desired, tu.TestLogger(t, false), tt.options)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateDiffWithOptions() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("GenerateDiffWithOptions() returned error: %v", err)
			}

			if diff == nil {
				t.Fatalf("GenerateDiffWithOptions() returned nil, want non-nil")
			}

			// Check the basic properties
			if diff.Gvk != tt.wantDiff.Gvk {
				t.Errorf("Gvk = %v, want %v", diff.Gvk.String(), tt.wantDiff.Gvk.String())
			}

			if diff.ResourceName != tt.wantDiff.ResourceName {
				t.Errorf("ResourceName = %v, want %v", diff.ResourceName, tt.wantDiff.ResourceName)
			}

			if diff.DiffType != tt.wantDiff.DiffType {
				t.Errorf("DiffType = %v, want %v", diff.DiffType, tt.wantDiff.DiffType)
			}

			// Check for line diffs - should be non-empty for changed resources
			if diff.DiffType != types.DiffTypeEqual && len(diff.LineDiffs) == 0 {
				t.Errorf("LineDiffs is empty for %s", name)
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

func TestFormatDiff(t *testing.T) {
	// Create test diffs
	simpleDiffs := []diffmatchpatch.Diff{
		{Type: diffmatchpatch.DiffEqual, Text: "unchanged line\n"},
		{Type: diffmatchpatch.DiffDelete, Text: "deleted line\n"},
		{Type: diffmatchpatch.DiffInsert, Text: "inserted line\n"},
		{Type: diffmatchpatch.DiffEqual, Text: "another unchanged line\n"},
	}

	tests := map[string]struct {
		diffs    []diffmatchpatch.Diff
		options  DiffOptions
		contains []string
		excludes []string
	}{
		"EmptyDiffs": {
			diffs:    []diffmatchpatch.Diff{},
			options:  DefaultDiffOptions(),
			contains: []string{},
			excludes: []string{"unchanged", "deleted", "inserted"},
		},
		"StandardFormatting": {
			diffs:   simpleDiffs,
			options: DefaultDiffOptions(),
			contains: []string{
				"unchanged line",
				"deleted line",
				"inserted line",
				"another unchanged line",
			},
		},
		"WithoutColors": {
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
		"WithColors": {
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
		"CompactFormat": {
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
		"CustomPrefixes": {
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

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
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
