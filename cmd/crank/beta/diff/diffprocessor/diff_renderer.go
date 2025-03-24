package diffprocessor

import (
	"cmp"
	"fmt"
	"io"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// DiffRenderer handles rendering diffs to output
type DiffRenderer interface {
	// RenderDiffs formats and outputs diffs to the provided writer
	RenderDiffs(stdout io.Writer, diffs map[string]*ResourceDiff) error
}

// DefaultDiffRenderer implements the DiffRenderer interface
type DefaultDiffRenderer struct {
	logger   logging.Logger
	diffOpts DiffOptions
}

// NewDiffRenderer creates a new DefaultDiffRenderer with the given options
func NewDiffRenderer(logger logging.Logger, diffOpts DiffOptions) DiffRenderer {
	return &DefaultDiffRenderer{
		logger:   logger,
		diffOpts: diffOpts,
	}
}

// SetDiffOptions updates the diff options used by the renderer
func (r *DefaultDiffRenderer) SetDiffOptions(options DiffOptions) {
	r.diffOpts = options
}

// RenderDiffs formats and prints the diffs to the provided writer
func (r *DefaultDiffRenderer) RenderDiffs(stdout io.Writer, diffs map[string]*ResourceDiff) error {
	r.logger.Debug("Rendering diffs to output",
		"diffCount", len(diffs),
		"useColors", r.diffOpts.UseColors,
		"compact", r.diffOpts.Compact)

	// Sort the keys to ensure a consistent output order
	d := maps.Values(diffs)

	// Sort by GetKindName which is how it's displayed to the user
	slices.SortFunc(d, func(a, b *ResourceDiff) int {
		return cmp.Compare(a.getKindName(), b.getKindName())
	})

	// Track stats for summary logging
	addedCount := 0
	modifiedCount := 0
	removedCount := 0
	equalCount := 0
	outputCount := 0

	for _, diff := range d {
		resourceID := diff.getKindName()

		// Count by diff type for summary
		switch diff.DiffType {
		case DiffTypeAdded:
			addedCount++
		case DiffTypeRemoved:
			removedCount++
		case DiffTypeModified:
			modifiedCount++
		case DiffTypeEqual:
			equalCount++
			// Skip rendering equal resources
			continue
		}

		// Format the diff header based on the diff type
		var header string
		switch diff.DiffType {
		case DiffTypeAdded:
			header = fmt.Sprintf("+++ %s", resourceID)
		case DiffTypeRemoved:
			header = fmt.Sprintf("--- %s", resourceID)
		case DiffTypeModified:
			header = fmt.Sprintf("~~~ %s", resourceID)
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
		summaryStr := summary.String()
		if strings.HasSuffix(summaryStr, ", ") {
			summaryStr = summaryStr[:len(summaryStr)-2]
		}

		if summaryStr != "\nSummary: " {
			_, err := fmt.Fprintln(stdout, summaryStr)
			if err != nil {
				return errors.Wrap(err, "failed to write summary to output")
			}
		}
	}

	return nil
}

// makeDiffKey creates a unique key for a resource diff
func makeDiffKey(apiVersion, kind, name string) string {
	return fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
}
