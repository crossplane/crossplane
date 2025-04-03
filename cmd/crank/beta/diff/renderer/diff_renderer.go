package renderer

import (
	"cmp"
	"fmt"
	"io"
	"strings"

	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"

	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	dt "github.com/crossplane/crossplane/cmd/crank/beta/diff/renderer/types"
)

// DiffRenderer handles rendering diffs to output.
type DiffRenderer interface {
	// RenderDiffs formats and outputs diffs to the provided writer
	RenderDiffs(stdout io.Writer, diffs map[string]*dt.ResourceDiff) error
}

// DefaultDiffRenderer implements the DiffRenderer interface.
type DefaultDiffRenderer struct {
	logger   logging.Logger
	diffOpts DiffOptions
}

// NewDiffRenderer creates a new DefaultDiffRenderer with the given options.
func NewDiffRenderer(logger logging.Logger, diffOpts DiffOptions) DiffRenderer {
	return &DefaultDiffRenderer{
		logger:   logger,
		diffOpts: diffOpts,
	}
}

// SetDiffOptions updates the diff options used by the renderer.
func (r *DefaultDiffRenderer) SetDiffOptions(options DiffOptions) {
	r.diffOpts = options
}

func getKindName(d *dt.ResourceDiff) string {
	// Check if the name indicates a generated name (ends with "(generated)")
	if strings.HasSuffix(d.ResourceName, "(generated)") {
		return fmt.Sprintf("%s/%s", d.Gvk.Kind, d.ResourceName)
	}

	// Regular case with a specific name
	return fmt.Sprintf("%s/%s", d.Gvk.Kind, d.ResourceName)
}

// RenderDiffs formats and prints the diffs to the provided writer.
func (r *DefaultDiffRenderer) RenderDiffs(stdout io.Writer, diffs map[string]*dt.ResourceDiff) error {
	r.logger.Debug("Rendering diffs to output",
		"diffCount", len(diffs),
		"useColors", r.diffOpts.UseColors,
		"compact", r.diffOpts.Compact)

	// Sort the keys to ensure a consistent output order
	d := maps.Values(diffs)

	// Sort by GetKindName which is how it's displayed to the user
	slices.SortFunc(d, func(a, b *dt.ResourceDiff) int {
		return cmp.Compare(getKindName(a), getKindName(b))
	})

	// Track stats for summary logging
	addedCount := 0
	modifiedCount := 0
	removedCount := 0
	equalCount := 0
	outputCount := 0

	for _, diff := range d {
		resourceID := getKindName(diff)

		// Count by diff type for summary
		switch diff.DiffType {
		case dt.DiffTypeAdded:
			addedCount++
		case dt.DiffTypeRemoved:
			removedCount++
		case dt.DiffTypeModified:
			modifiedCount++
		case dt.DiffTypeEqual:
			equalCount++
			// Skip rendering equal resources
			continue
		}

		// Format the diff header based on the diff type
		var header string
		switch diff.DiffType {
		case dt.DiffTypeAdded:
			header = fmt.Sprintf("+++ %s", resourceID)
		case dt.DiffTypeRemoved:
			header = fmt.Sprintf("--- %s", resourceID)
		case dt.DiffTypeModified:
			header = fmt.Sprintf("~~~ %s", resourceID)
		case dt.DiffTypeEqual:
			// should never get here
			header = ""
		}

		// Format the diff content
		content := FormatDiff(diff.LineDiffs, r.diffOpts)

		if content != "" {
			_, err := fmt.Fprintf(stdout, "%s\n%s\n---\n", header, content)
			if err != nil {
				r.logger.Debug("Error writing diff to output", "resource", resourceID, "error", err)
				return errors.Wrap(err, "failed to write diff to output")
			}
			outputCount++
		} else {
			r.logger.Debug("Empty diff content, skipping output", "resource", resourceID)
		}
	}

	r.logger.Debug("Diff rendering complete",
		"added", addedCount,
		"removed", removedCount,
		"modified", modifiedCount,
		"equal", equalCount,
		"output", outputCount)

	// Add a summary to the output if there were diffs
	if outputCount > 0 {
		summary := strings.Builder{}
		summary.WriteString("\nSummary: ")

		if addedCount > 0 {
			summary.WriteString(fmt.Sprintf("%d added, ", addedCount))
		}
		if modifiedCount > 0 {
			summary.WriteString(fmt.Sprintf("%d modified, ", modifiedCount))
		}
		if removedCount > 0 {
			summary.WriteString(fmt.Sprintf("%d removed, ", removedCount))
		}

		// Remove trailing comma and space
		summaryStr := strings.TrimSuffix(summary.String(), ", ")

		if summaryStr != "\nSummary: " {
			_, err := fmt.Fprintln(stdout, summaryStr)
			if err != nil {
				return errors.Wrap(err, "failed to write summary to output")
			}
		}
	}

	return nil
}
