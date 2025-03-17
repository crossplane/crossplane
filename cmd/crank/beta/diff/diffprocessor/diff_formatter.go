package diffprocessor

import (
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigsyaml "sigs.k8s.io/yaml"
	"strings"
)

// GenerateDiff produces a formatted diff between two unstructured objects
func GenerateDiff(current, desired *unstructured.Unstructured, kind, name string) (string, error) {

	cleanAndRender := func(obj *unstructured.Unstructured) (string, error) {
		clean := cleanupForDiff(obj.DeepCopy())

		// Convert both objects to YAML strings for diffing
		cleanYAML, err := sigsyaml.Marshal(clean.Object)
		if err != nil {
			return "", errors.Wrap(err, "cannot marshal current object to YAML")
		}

		return string(cleanYAML), nil
	}

	currentStr := ""
	var err error
	if current != nil {
		currentStr, err = cleanAndRender(current)
		if err != nil {
			return "", err
		}
	}

	desiredStr, err := cleanAndRender(desired)

	// get the full line by line diff
	diffResult := GetLineDiff(currentStr, desiredStr, CompactDiffOptions())

	if diffResult == "" {
		return "", nil
	}

	var leadChar string

	switch current {
	case nil:
		leadChar = "+++" // Resource does not exist
		// TODO: deleted resources should be shown as deleted
	default:
		leadChar = "~~~" // Resource exists and is changing
	}

	// Format the output with a resource header
	return fmt.Sprintf("%s %s/%s\n%s", leadChar, kind, name, diffResult), nil
}

// cleanupForDiff removes fields that shouldn't be included in the diff
func cleanupForDiff(obj *unstructured.Unstructured) *unstructured.Unstructured {
	// Remove server-side fields and metadata that we don't want to diff
	metadata, found, _ := unstructured.NestedMap(obj.Object, "metadata")
	if found {
		// Remove fields that change automatically or are server-side
		fieldsToRemove := []string{
			"resourceVersion",
			"uid",
			"generation",
			"creationTimestamp",
			"managedFields",
			"selfLink",
			"ownerReferences",
		}

		for _, field := range fieldsToRemove {
			delete(metadata, field)
		}

		unstructured.SetNestedMap(obj.Object, metadata, "metadata")
	}

	// Remove status field as we're focused on spec changes
	delete(obj.Object, "status")

	return obj
}

// Colors for terminal output
const (
	ColorRed   = "\x1b[31m"
	ColorGreen = "\x1b[32m"
	ColorReset = "\x1b[0m"
)

// DiffOptions holds configuration options for the diff output
type DiffOptions struct {
	// UseColors determines whether to colorize the output
	UseColors bool

	// AddPrefix is the prefix for added lines (default "+")
	AddPrefix string

	// DeletePrefix is the prefix for deleted lines (default "-")
	DeletePrefix string

	// ContextPrefix is the prefix for unchanged lines (default " ")
	ContextPrefix string

	// ContextLines is the number of unchanged lines to show before/after changes in compact mode
	ContextLines int

	// ChunkSeparator is the string used to separate chunks in compact mode
	ChunkSeparator string

	// Compact determines whether to show a compact diff
	Compact bool
}

// DefaultDiffOptions returns the default options with colors enabled
func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		UseColors:      true,
		AddPrefix:      "+ ",
		DeletePrefix:   "- ",
		ContextPrefix:  "  ",
		ContextLines:   3,
		ChunkSeparator: "...",
		Compact:        false,
	}
}

// CompactDiffOptions returns the default options with colors enabled
func CompactDiffOptions() DiffOptions {
	return DiffOptions{
		UseColors:      true,
		AddPrefix:      "+ ",
		DeletePrefix:   "- ",
		ContextPrefix:  "  ",
		ContextLines:   3,
		ChunkSeparator: "...",
		Compact:        true,
	}
}

// DiffFormatter is the interface that defines the contract for diff formatters
type DiffFormatter interface {
	Format(diffs []diffmatchpatch.Diff, options DiffOptions) string
}

// FullDiffFormatter formats diffs with all context lines
type FullDiffFormatter struct{}

// CompactDiffFormatter formats diffs with limited context lines
type CompactDiffFormatter struct{}

// NewFormatter returns a DiffFormatter based on whether compact mode is desired
func NewFormatter(compact bool) DiffFormatter {
	if compact {
		return &CompactDiffFormatter{}
	}
	return &FullDiffFormatter{}
}

// Format implements the DiffFormatter interface for FullDiffFormatter
func (f *FullDiffFormatter) Format(diffs []diffmatchpatch.Diff, options DiffOptions) string {
	var builder strings.Builder

	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")

		// Handle the trailing newline correctly
		hasTrailingNewline := strings.HasSuffix(diff.Text, "\n")
		if hasTrailingNewline && len(lines) > 0 {
			lines = lines[:len(lines)-1]
		}

		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			for _, line := range lines {
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s%s\n", ColorGreen, options.AddPrefix, line, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.AddPrefix, line))
				}
			}
			if hasTrailingNewline && len(lines) == 0 {
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s\n", ColorGreen, options.AddPrefix, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s\n", options.AddPrefix))
				}
			}

		case diffmatchpatch.DiffDelete:
			for _, line := range lines {
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s%s\n", ColorRed, options.DeletePrefix, line, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.DeletePrefix, line))
				}
			}
			if hasTrailingNewline && len(lines) == 0 {
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s\n", ColorRed, options.DeletePrefix, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s\n", options.DeletePrefix))
				}
			}

		case diffmatchpatch.DiffEqual:
			for _, line := range lines {
				builder.WriteString(fmt.Sprintf("%s%s\n", options.ContextPrefix, line))
			}
			if hasTrailingNewline && len(lines) == 0 {
				builder.WriteString(fmt.Sprintf("%s\n", options.ContextPrefix))
			}
		}
	}

	return builder.String()
}

// Format implements the DiffFormatter interface for CompactDiffFormatter
func (f *CompactDiffFormatter) Format(diffs []diffmatchpatch.Diff, options DiffOptions) string {
	// First, convert diffs to line-based items
	type lineItem struct {
		Type    diffmatchpatch.Operation
		Content string
	}

	var allLines []lineItem

	// Process all diffs into individual lines
	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")

		// Handle the trailing newline correctly
		hasTrailingNewline := strings.HasSuffix(diff.Text, "\n")
		if hasTrailingNewline && len(lines) > 0 {
			lines = lines[:len(lines)-1]
		}

		for _, line := range lines {
			allLines = append(allLines, lineItem{
				Type:    diff.Type,
				Content: line,
			})
		}

		// Add an empty line for trailing newlines
		if hasTrailingNewline && len(lines) == 0 {
			allLines = append(allLines, lineItem{
				Type:    diff.Type,
				Content: "",
			})
		}
	}

	// Now build compact output with context
	var builder strings.Builder
	contextLines := options.ContextLines
	inChange := false
	lastPrintedIdx := -1

	for i := 0; i < len(allLines); i++ {
		// If this is a change (insert or delete)
		if allLines[i].Type != diffmatchpatch.DiffEqual {
			// If we weren't already in a change block
			if !inChange {
				inChange = true

				// Print separator if there's a gap
				if lastPrintedIdx != -1 && i-lastPrintedIdx > 1 {
					builder.WriteString(fmt.Sprintf("%s\n", options.ChunkSeparator))
				}

				// Print preceding context lines
				startIdx := max(0, i-contextLines)
				for j := startIdx; j < i; j++ {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.ContextPrefix, allLines[j].Content))
				}
			}

			// Print the change
			switch allLines[i].Type {
			case diffmatchpatch.DiffInsert:
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s%s\n", ColorGreen, options.AddPrefix, allLines[i].Content, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.AddPrefix, allLines[i].Content))
				}
			case diffmatchpatch.DiffDelete:
				if options.UseColors {
					builder.WriteString(fmt.Sprintf("%s%s%s%s\n", ColorRed, options.DeletePrefix, allLines[i].Content, ColorReset))
				} else {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.DeletePrefix, allLines[i].Content))
				}
			}

			lastPrintedIdx = i
		} else {
			// This is an equal/context line
			if inChange {
				// We were in a change, print following context
				if i-lastPrintedIdx <= contextLines {
					builder.WriteString(fmt.Sprintf("%s%s\n", options.ContextPrefix, allLines[i].Content))
					lastPrintedIdx = i
				} else {
					// We've printed enough context lines after the change
					inChange = false
				}
			}
		}
	}

	// Add final separator if there are lines after the last printed line
	if lastPrintedIdx != -1 && lastPrintedIdx < len(allLines)-1 {
		builder.WriteString(fmt.Sprintf("%s\n", options.ChunkSeparator))
	}

	return builder.String()
}

// GetLineDiff performs a proper line-by-line diff and returns the formatted result
func GetLineDiff(oldText, newText string, options DiffOptions) string {
	patch := diffmatchpatch.New()

	// Use the line-to-char conversion to treat each line as an atomic unit
	ch1, ch2, lines := patch.DiffLinesToChars(oldText, newText)

	diff := patch.DiffMain(ch1, ch2, false)
	patch.DiffCleanupSemantic(diff)

	lineDiffs := patch.DiffCharsToLines(diff, lines)

	// Use the appropriate formatter
	formatter := NewFormatter(options.Compact)
	return formatter.Format(lineDiffs, options)
}
