package diffprocessor

import (
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	sigsyaml "sigs.k8s.io/yaml"
	"strings"
)

// DiffType represents the type of diff (added, removed, modified)
type DiffType string

const (
	DiffTypeAdded    DiffType = "+"
	DiffTypeRemoved  DiffType = "-"
	DiffTypeModified DiffType = "~"
	DiffTypeEqual    DiffType = "="
)

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

// ResourceDiff represents the diff for a specific resource
type ResourceDiff struct {
	Gvk          schema.GroupVersionKind
	ResourceName string
	DiffType     DiffType
	LineDiffs    []diffmatchpatch.Diff
	Current      *unstructured.Unstructured // Optional, for reference
	Desired      *unstructured.Unstructured // Optional, for reference
}

func (d *ResourceDiff) getKindName() string {
	// Check if the name indicates a generated name (ends with "(generated)")
	if strings.HasSuffix(d.ResourceName, "(generated)") {
		return fmt.Sprintf("%s/%s", d.Gvk.Kind, d.ResourceName)
	}

	// Regular case with a specific name
	return fmt.Sprintf("%s/%s", d.Gvk.Kind, d.ResourceName)
}

func (d *ResourceDiff) GetDiffKey() string {
	return makeDiffKey(d.Gvk.Group+"/"+d.Gvk.Version, d.Gvk.Kind, d.ResourceName)
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

// FormatDiff formats a slice of diffs according to the provided options
func FormatDiff(diffs []diffmatchpatch.Diff, options DiffOptions) string {
	// Use the appropriate formatter
	formatter := NewFormatter(options.Compact)
	return formatter.Format(diffs, options)
}

// Format implements the DiffFormatter interface for FullDiffFormatter
func (f *FullDiffFormatter) Format(diffs []diffmatchpatch.Diff, options DiffOptions) string {
	var builder strings.Builder

	for _, diff := range diffs {
		formattedLines, _ := processLines(diff, options)
		for _, line := range formattedLines {
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// Format implements the DiffFormatter interface for CompactDiffFormatter
func (f *CompactDiffFormatter) Format(diffs []diffmatchpatch.Diff, options DiffOptions) string {
	// Create a flat array of all formatted lines with their diff types
	type lineItem struct {
		Type      diffmatchpatch.Operation
		Content   string
		Formatted string
	}

	var allLines []lineItem

	for _, diff := range diffs {
		formattedLines, hasTrailingNewline := processLines(diff, options)

		for i, formatted := range formattedLines {
			// For non-trailing empty lines or regular lines
			content := ""
			if isEmptyTrailer := hasTrailingNewline && len(formattedLines) == 1 && i == 0; !isEmptyTrailer {
				content = strings.Split(diff.Text, "\n")[i]
			}

			allLines = append(allLines, lineItem{
				Type:      diff.Type,
				Content:   content,
				Formatted: formatted,
			})
		}
	}

	// Now build compact output with context
	var builder strings.Builder
	contextLines := options.ContextLines

	// Find change blocks (sequences of inserts/deletes)
	type changeBlock struct {
		StartIdx int
		EndIdx   int
	}

	var changeBlocks []changeBlock
	var currentBlock *changeBlock

	// Identify all the change blocks
	for i := 0; i < len(allLines); i++ {
		if allLines[i].Type != diffmatchpatch.DiffEqual {
			// Start a new block if we don't have one
			if currentBlock == nil {
				currentBlock = &changeBlock{StartIdx: i, EndIdx: i}
			} else {
				// Extend current block
				currentBlock.EndIdx = i
			}
		} else if currentBlock != nil {
			// If we were in a block and hit an equal line, finish the block
			changeBlocks = append(changeBlocks, *currentBlock)
			currentBlock = nil
		}
	}

	// Add the last block if it's still active
	if currentBlock != nil {
		changeBlocks = append(changeBlocks, *currentBlock)
	}

	// If we have no change blocks, return an empty string
	if len(changeBlocks) == 0 {
		return ""
	}

	// Keep track of the last line we printed
	lastPrintedIdx := -1

	// Now process each block with its context
	for blockIdx, block := range changeBlocks {
		// Calculate visible range for context before the block
		contextStart := max(0, block.StartIdx-contextLines)

		// If this isn't the first block, check if we need a separator
		if blockIdx > 0 {
			prevBlock := changeBlocks[blockIdx-1]
			prevContextEnd := min(len(allLines), prevBlock.EndIdx+contextLines+1)

			// If there's a gap between the end of the previous context and the start of this context,
			// add a separator
			if contextStart > prevContextEnd {
				// Add separator
				builder.WriteString(fmt.Sprintf("%s\n", options.ChunkSeparator))
				lastPrintedIdx = -1 // Reset to force printing of context lines
			} else {
				// Contexts overlap or are adjacent - adjust the start to avoid duplicate lines
				contextStart = max(lastPrintedIdx+1, contextStart)
			}
		}

		// Print context before the change if we haven't already printed it
		for i := contextStart; i < block.StartIdx; i++ {
			if i > lastPrintedIdx {
				builder.WriteString(allLines[i].Formatted)
				builder.WriteString("\n")
				lastPrintedIdx = i
			}
		}

		// Print the changes
		for i := block.StartIdx; i <= block.EndIdx; i++ {
			builder.WriteString(allLines[i].Formatted)
			builder.WriteString("\n")
			lastPrintedIdx = i
		}

		// Print context after the change
		contextEnd := min(len(allLines), block.EndIdx+contextLines+1)
		for i := block.EndIdx + 1; i < contextEnd; i++ {
			builder.WriteString(allLines[i].Formatted)
			builder.WriteString("\n")
			lastPrintedIdx = i
		}
	}

	return builder.String()
}

// GetLineDiff performs a proper line-by-line diff and returns the raw diffs
func GetLineDiff(oldText, newText string) []diffmatchpatch.Diff {
	patch := diffmatchpatch.New()

	// Use the line-to-char conversion to treat each line as an atomic unit
	ch1, ch2, lines := patch.DiffLinesToChars(oldText, newText)

	diff := patch.DiffMain(ch1, ch2, false)
	patch.DiffCleanupSemantic(diff)

	return patch.DiffCharsToLines(diff, lines)
}

// GenerateDiffWithOptions produces a structured diff between two unstructured objects
func GenerateDiffWithOptions(current, desired *unstructured.Unstructured, logger logging.Logger, _ DiffOptions) (*ResourceDiff, error) {
	var diffType DiffType

	// Determine resource identifiers upfront
	resourceKey := "unknown/unknown"
	if desired != nil {
		resourceKey = fmt.Sprintf("%s/%s", desired.GetKind(), desired.GetName())
	} else if current != nil {
		resourceKey = fmt.Sprintf("%s/%s", current.GetKind(), current.GetName())
	}

	logger.Debug("Generating diff", "resource", resourceKey)

	// Determine diff type
	switch {
	case current == nil && desired != nil:
		diffType = DiffTypeAdded
		logger.Debug("Diff type: Resource is being added", "resource", resourceKey)
	case current != nil && desired == nil:
		diffType = DiffTypeRemoved
		logger.Debug("Diff type: Resource is being removed", "resource", resourceKey)
	case current != nil: // && desired != nil:
		diffType = DiffTypeModified
		logger.Debug("Diff type: Resource is being modified", "resource", resourceKey)
	default:
		logger.Debug("Error: both current and desired are nil")
		return nil, errors.New("both current and desired cannot be nil")
	}

	// For modifications, check if objects are semantically equal
	if diffType == DiffTypeModified {
		// Check for deep equality first
		if equality.Semantic.DeepEqual(current, desired) {
			logger.Debug("Resources are semantically equal", "resource", resourceKey)
			return equalDiff(current, desired), nil
		}

		// Clean up both objects for comparison
		currentClean := cleanupForDiff(current.DeepCopy(), logger)
		desiredClean := cleanupForDiff(desired.DeepCopy(), logger)

		// Check if the cleaned objects are equal
		if equality.Semantic.DeepEqual(currentClean.Object, desiredClean.Object) {
			logger.Debug("Resources are equal after cleanup (only metadata differences)", "resource", resourceKey)
			return equalDiff(current, desired), nil
		}

		logger.Debug("Resources are not equal after cleanup", "resource", resourceKey)
	}

	// Convert to YAML for text diff
	asString := func(obj *unstructured.Unstructured) (string, error) {
		if obj == nil {
			return "", nil
		}
		clean := cleanupForDiff(obj.DeepCopy(), logger)
		yaml, err := sigsyaml.Marshal(clean.Object)
		if err != nil {
			return "", err
		}
		return string(yaml), nil
	}

	currentStr, err := asString(current)
	if err != nil {
		logger.Debug("Error marshaling current object to YAML", "error", err)
		return nil, errors.Wrap(err, "cannot marshal current object to YAML")
	}

	desiredStr, err := asString(desired)
	if err != nil {
		logger.Debug("Error marshaling desired object to YAML", "error", err)
		return nil, errors.Wrap(err, "cannot marshal desired object to YAML")
	}

	// Return nil if content is identical
	if desiredStr == currentStr {
		logger.Debug("Resources have identical YAML representation", "resource", resourceKey)
		return equalDiff(current, desired), nil
	}

	// Get the line by line diff
	logger.Debug("Computing line-by-line diff", "resource", resourceKey)
	lineDiffs := GetLineDiff(currentStr, desiredStr)

	if len(lineDiffs) == 0 {
		logger.Debug("No differences found in line-by-line comparison", "resource", resourceKey)
		return equalDiff(current, desired), nil
	}

	logger.Debug("Diff calculation complete", "resource", resourceKey, "diff_chunks", len(lineDiffs))

	// Extract resource kind and name
	var name string
	var gvk schema.GroupVersionKind
	// For removed resources, use current's kind and name
	if diffType == DiffTypeRemoved { // current != nil
		name = current.GetName()
		gvk = current.GroupVersionKind()
	} else { // desired != nil
		// For added or modified resources, use desired's kind
		gvk = desired.GroupVersionKind()

		// For name, prefer the current resource name if it exists (for generateName cases)
		if current != nil && current.GetName() != "" {
			name = current.GetName()
		} else {
			// If desired has a name, use it
			if desired.GetName() != "" {
				name = desired.GetName()
			} else if desired.GetGenerateName() != "" {
				// Special handling for resources with generateName
				// Format as "prefix-(generated)" to match expected naming pattern
				name = desired.GetGenerateName() + "(generated)"
			}
		}
	}

	return &ResourceDiff{
		Gvk:          gvk,
		ResourceName: name,
		DiffType:     diffType,
		LineDiffs:    lineDiffs,
		Current:      current,
		Desired:      desired,
	}, nil
}

func equalDiff(current *unstructured.Unstructured, desired *unstructured.Unstructured) *ResourceDiff {
	return &ResourceDiff{
		Gvk:          current.GroupVersionKind(),
		ResourceName: current.GetName(),
		DiffType:     DiffTypeEqual,
		LineDiffs:    []diffmatchpatch.Diff{},
		Current:      current,
		Desired:      desired,
	}
}

// processLines extracts lines from a diff and processes them into a standardized format
// Returns the processed lines and whether there was a trailing newline
func processLines(diff diffmatchpatch.Diff, options DiffOptions) ([]string, bool) {
	lines := strings.Split(diff.Text, "\n")
	hasTrailingNewline := strings.HasSuffix(diff.Text, "\n")

	// If there's a trailing newline, the split produces an empty string at the end
	if hasTrailingNewline && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}

	var result []string

	// Format each line with appropriate prefix and color
	for _, line := range lines {
		result = append(result, formatLine(line, diff.Type, options))
	}

	// Add formatted empty line if there was just a newline
	if hasTrailingNewline && len(lines) == 0 {
		result = append(result, formatLine("", diff.Type, options))
	}

	return result, hasTrailingNewline
}

// formatLine applies the appropriate prefix and color to a single line
func formatLine(line string, diffType diffmatchpatch.Operation, options DiffOptions) string {
	var prefix string
	var colorStart, colorEnd string

	switch diffType {
	case diffmatchpatch.DiffInsert:
		prefix = options.AddPrefix
		if options.UseColors {
			colorStart = ColorGreen
			colorEnd = ColorReset
		}
	case diffmatchpatch.DiffDelete:
		prefix = options.DeletePrefix
		if options.UseColors {
			colorStart = ColorRed
			colorEnd = ColorReset
		}
	case diffmatchpatch.DiffEqual:
		prefix = options.ContextPrefix
	}

	if options.UseColors && colorStart != "" {
		return fmt.Sprintf("%s%s%s%s", colorStart, prefix, line, colorEnd)
	}
	return fmt.Sprintf("%s%s", prefix, line)
}

// cleanupForDiff removes fields that shouldn't be included in the diff
func cleanupForDiff(obj *unstructured.Unstructured, logger logging.Logger) *unstructured.Unstructured {
	resKind := obj.GetKind()
	resName := obj.GetName()
	resKey := fmt.Sprintf("%s/%s", resKind, resName)

	// Track all modifications for a single consolidated log message
	modifications := []string{}

	// Remove server-side fields and metadata that we don't want to diff
	metadata, found, _ := unstructured.NestedMap(obj.Object, "metadata")
	if found {
		// Special handling for objects with both name and generateName
		// If the name looks like a generated display name (ends with "(generated)")
		// and generateName is also present, remove the name to avoid confusion
		name, nameFound, _ := unstructured.NestedString(metadata, "name")
		generateName, genNameFound, _ := unstructured.NestedString(metadata, "generateName")

		if nameFound && genNameFound && strings.HasSuffix(name, "(generated)") {
			// This is a display name we added for diffing purposes - remove it
			// since we only added it for diffing but don't want it to show in the actual diff
			delete(metadata, "name")
			modifications = append(modifications, fmt.Sprintf("removed display name %q", name))

			// Also normalize generateName by removing any "(generated)" suffix
			if strings.HasSuffix(generateName, "(generated)-") {
				// For downstream resources that have generateName mangled with the parent's display name
				// Strip the "(generated)" part to match the original input
				originalGenName := strings.TrimSuffix(generateName, "(generated)-")
				metadata["generateName"] = originalGenName
				modifications = append(modifications, fmt.Sprintf("normalized generateName from %q to %q", generateName, originalGenName))
			}

			// Don't change the composite label - it should keep the (generated) suffix
			// This is because downstream resources should refer to their parent
			// with the same display name that appears in the diff
		}

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

		// Track which fields were actually removed for debugging
		removedFields := []string{}
		for _, field := range fieldsToRemove {
			if _, exists := metadata[field]; exists {
				delete(metadata, field)
				removedFields = append(removedFields, field)
			}
		}

		// Only record if some fields were actually removed
		if len(removedFields) > 0 {
			modifications = append(modifications, fmt.Sprintf("metadata fields: %s", strings.Join(removedFields, ", ")))
		}

		unstructured.SetNestedMap(obj.Object, metadata, "metadata")
	}

	// Remove resourceRefs field from spec if it exists
	spec, found, _ := unstructured.NestedMap(obj.Object, "spec")
	if found && spec != nil {
		if _, exists := spec["resourceRefs"]; exists {
			delete(spec, "resourceRefs")
			modifications = append(modifications, "resourceRefs from spec")
		}

		unstructured.SetNestedMap(obj.Object, spec, "spec")
	}

	// Remove status field as we're focused on spec changes
	if _, exists := obj.Object["status"]; exists {
		delete(obj.Object, "status")
		modifications = append(modifications, "status field")
	}

	// Log a single consolidated message if any modifications were made
	if len(modifications) > 0 {
		logger.Debug("Cleaned object for diff",
			"resource", resKey,
			"removed", strings.Join(modifications, ", "))
	}

	return obj
}
