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

package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/fatih/color"
	"golang.org/x/term"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// minWrapWidth is the floor for body wrapping. Below this we skip wrapping
// rather than emit tiny fragments.
const minWrapWidth = 40

// sidebarPrefix stamps every body line in a category section. The leading
// spaces align body content under the "[✗] " / "[!] " badge in the header.
const sidebarPrefix = "    │  "

const sidebarClose = "    └──"

// Badge glyphs for the verdict header and category lines. Each state uses the
// same glyph in both renderers so the visual vocabulary stays consistent.
const (
	badgeIssue      = "[✗]"
	badgeIncomplete = "[!]"
	badgeInfo       = "[i]"
	badgeOK         = "[✓]"
)

// Printer renders a Report.
type Printer interface {
	Print(w io.Writer, r Report) error
}

// NewPrinter returns a Printer for the named output format.
func NewPrinter(format string) (Printer, error) {
	switch format {
	case "text", "":
		return &TextPrinter{}, nil
	case "json":
		return &JSONPrinter{}, nil
	default:
		return nil, errors.Errorf("unknown output format %q", format)
	}
}

// TextPrinter renders a Report as a kubectl-style, human-readable summary.
type TextPrinter struct{}

// Print writes the given report to the given writer.
func (p *TextPrinter) Print(w io.Writer, r Report) error {
	bodyWidth := detectBodyWidth(w)

	// Print the header of an overall verdict badge plus a fixed
	// "N issues, M informational, K incomplete checks." breakdown.
	if len(r.Categories) > 0 {
		issues, info, incomplete := summarize(r.Categories)
		printVerdict(w, issues, info, incomplete)
	}

	for _, c := range r.Categories {
		printCategory(w, c, bodyWidth)
	}

	return nil
}

// summarize tallies issue-severity findings, info-severity findings, and
// incomplete checks across categories for the verdict header.
func summarize(categories []CategoryResult) (issues, info, incomplete int) {
	for _, c := range categories {
		if c.Err != "" {
			incomplete++
		}
		if c.Severity == SeverityInfo {
			info += len(c.Findings)
		} else {
			issues += len(c.Findings)
		}
	}
	return issues, info, incomplete
}

// printVerdict writes the one-line overall summary badge and breakdown.
// Example: [✗] 23 issues, 6 informational, 1 incomplete check.
func printVerdict(w io.Writer, issues, info, incomplete int) {
	var badge string
	switch {
	case issues > 0:
		badge = badgeIssue
	case incomplete > 0:
		badge = badgeIncomplete
	case info > 0:
		badge = badgeInfo
	default:
		badge = badgeOK
	}
	issuesFrag := color.RedString(pluralize(issues, "issue", "issues"))
	infoFrag := color.CyanString(pluralize(info, "informational", "informational"))
	incompleteFrag := color.YellowString(pluralize(incomplete, "incomplete check", "incomplete checks"))
	_, _ = fmt.Fprintf(w, "%s %s, %s, %s.\n\n", badge, issuesFrag, infoFrag, incompleteFrag)
}

// printCategory writes one category section: a single confirmation line when
// healthy, otherwise a header followed by a sidebar body of description,
// error, fix, docs, and findings.
func printCategory(w io.Writer, c CategoryResult, bodyWidth int) {
	// Healthy category: render as a single confirmation line. Description
	// / Fix / Docs are omitted intentionally - no need to give advice when
	// there is no problem.
	if len(c.Findings) == 0 && c.Err == "" {
		_, _ = fmt.Fprintf(w, "%s %s\n", badgeOK, c.Title)
		return
	}

	// An incomplete check beats findings in the badge: the findings list
	// may be partial when the check errored, so we surface the unknown
	// rather than imply exhaustiveness.
	//
	// Note that for all the printing here, we do a colored badge, no color for
	// the category title, then color again for the number/type of findings to
	// call special attention to them.
	switch {
	case c.Err != "":
		_, _ = fmt.Fprintf(w, "%s %s - %s\n", color.YellowString(badgeIncomplete), c.Title, color.YellowString("incomplete check"))
	case c.Severity == SeverityInfo:
		_, _ = fmt.Fprintf(w, "%s %s - %s\n", color.CyanString(badgeInfo), c.Title, color.CyanString(pluralize(len(c.Findings), "informational finding", "informational findings")))
	default:
		_, _ = fmt.Fprintf(w, "%s %s - %s\n", color.RedString(badgeIssue), c.Title, color.RedString(pluralize(len(c.Findings), "issue", "issues")))
	}

	pw := newLinePrefixWriter(w, sidebarPrefix)
	_, _ = fmt.Fprintln(pw)

	// Track whether we wrote any of the description/error/fix/docs block so we
	// can decide whether the findings table needs its own leading separator.
	wroteBody := false
	if c.Description != "" {
		writeWrapped(pw, "", c.Description, bodyWidth)
		wroteBody = true
	}
	if c.Err != "" {
		writeWrapped(pw, "Error: ", c.Err, bodyWidth)
		wroteBody = true
	}
	if c.Remediation != "" {
		writeWrapped(pw, "Fix:   ", c.Remediation, bodyWidth)
		wroteBody = true
	}
	if len(c.DocsURLs) > 0 {
		// One URL per line. The first carries the "Docs:" label; subsequent
		// lines indent under it. A single URL longer than the body width
		// still overflows the sidebar - wrapText won't break mid-word.
		docsIndent := strings.Repeat(" ", utf8.RuneCountInString("Docs:  "))
		for i, u := range c.DocsURLs {
			label := docsIndent
			if i == 0 {
				label = "Docs:  "
			}
			writeWrapped(pw, label, u, bodyWidth)
		}
		wroteBody = true
	}

	// print all the findings for this category, grouped by Kind so the output
	// has a familiar kubectl get feel to it
	if len(c.Findings) > 0 {
		// Separate the findings table from the body above it - but only when
		// there was a body. Without this guard, a category that has findings
		// and nothing else would emit two blank separator lines in a row (the
		// post-header one plus this one).
		if wroteBody {
			_, _ = fmt.Fprintln(pw)
		}
		groups := groupByKind(c.Findings)
		for i, g := range groups {
			if i > 0 {
				_, _ = fmt.Fprintln(pw)
			}
			printKindGroup(pw, g)
		}
	}

	_, _ = fmt.Fprintln(w, sidebarClose)
}

// linePrefixWriter prepends a constant prefix to each line written through it,
// e.g. category sections that need a consistent left-edge sidebar.
//
// Used as an io.Writer middleware: callers can either write to it directly
// or wrap it with another writer, in either case every new line written
// will contain the prefix.
type linePrefixWriter struct {
	w           io.Writer
	prefix      string
	atLineStart bool
}

func newLinePrefixWriter(w io.Writer, prefix string) *linePrefixWriter {
	return &linePrefixWriter{w: w, prefix: prefix, atLineStart: true}
}

// Write writes the given bytes, prepending the configured prefix to each new
// line so callers get a consistent left-edge sidebar without having to thread
// the prefix through every Fprint call.
func (lw *linePrefixWriter) Write(p []byte) (int, error) {
	written := 0
	// Process the given bytes one line at a time so we can stamp the prefix at
	// the start of each new line.
	for len(p) > 0 {
		if lw.atLineStart {
			// We're at the start of a new line, so emit the prefix now. When
			// the line has no content (its first byte is the newline), emit the
			// prefix with its trailing spaces trimmed: the sidebar bar still
			// continues, but we don't leave trailing whitespace on a blank line.
			prefix := lw.prefix
			if p[0] == '\n' {
				prefix = strings.TrimRight(prefix, " ")
			}
			if _, err := io.WriteString(lw.w, prefix); err != nil {
				return written, err
			}
			lw.atLineStart = false
		}

		nextNewline := bytes.IndexByte(p, '\n')
		if nextNewline < 0 {
			// No newlines left in p, flush the rest now
			n, err := lw.w.Write(p)
			return written + n, err
		}

		// Write up to (and including) the next newline
		n, err := lw.w.Write(p[:nextNewline+1])
		written += n
		if err != nil {
			return written, err
		}

		// We're at a newline now, set the atLineStart flag again
		p = p[nextNewline+1:]
		lw.atLineStart = true
	}
	return written, nil
}

// groupByKind partitions findings into sub-slices that share the same
// Group+Kind, preserving the order in which kinds were first seen.
func groupByKind(findings []Finding) [][]Finding {
	seen := map[string]int{}
	out := [][]Finding{}
	for _, f := range findings {
		key := f.Resource.Group + "|" + f.Resource.Kind
		if i, ok := seen[key]; ok {
			// this finding belongs to a kind we've already seen, append it to
			// the existing list for its kind
			out[i] = append(out[i], f)
			continue
		}

		// we haven't seen this finding's kind yet, start a new list for its kind
		seen[key] = len(out)
		out = append(out, []Finding{f})
	}
	return out
}

func printKindGroup(w io.Writer, g []Finding) {
	// determine if this group is for namespaced resources or not
	namespaced := false
	for _, f := range g {
		if f.Resource.Namespace != "" {
			namespaced = true
			break
		}
	}

	const padding = 3 // space between columns
	tw := tabwriter.NewWriter(w, 0, 4, padding, ' ', 0)

	// print the column headers
	if namespaced {
		_, _ = fmt.Fprintln(tw, "  NAMESPACE\tNAME\tFIELD")
	} else {
		_, _ = fmt.Fprintln(tw, "  NAME\tFIELD")
	}

	for _, f := range g {
		// print the name of each resource in a kind.group/name style similar to kubectl
		gk := schema.GroupKind{Group: f.Resource.Group, Kind: strings.ToLower(f.Resource.Kind)}
		name := gk.String() + "/" + f.Resource.Name

		field := f.FieldPath
		if field == "" {
			// no field path to include, just use "-"
			field = "-"
		}
		if namespaced {
			ns := f.Resource.Namespace
			if ns == "" {
				// no namespace to include, just use "-"
				ns = "-"
			}
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n", ns, name, field)
		} else {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\n", name, field)
		}
	}
	_ = tw.Flush()
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// detectBodyWidth returns the column width available for wrapped body lines
// (terminal width minus sidebar prefix). Returns 0 when the output isn't a
// terminal, so long lines stay intact for grep and other downstream tools.
func detectBodyWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	cols -= utf8.RuneCountInString(sidebarPrefix)
	if cols < minWrapWidth {
		return 0
	}
	return cols
}

// writeWrapped writes label followed by body, word-wrapping body to maxWidth
// columns and indenting continuation lines under the label. maxWidth <= 0
// disables wrapping.
func writeWrapped(w io.Writer, label, body string, maxWidth int) {
	if maxWidth <= 0 {
		// wrapping is disabled, just print with no wrapping
		_, _ = fmt.Fprintf(w, "%s%s\n", label, body)
		return
	}
	labelLen := utf8.RuneCountInString(label)
	contentWidth := maxWidth - labelLen
	if contentWidth < minWrapWidth {
		// the content isn't wide enough to bother wrapping it, print with no wrapping
		_, _ = fmt.Fprintf(w, "%s%s\n", label, body)
		return
	}

	// wrap the entire body to the given width now, which will give us a set of lines
	lines := wrapText(body, contentWidth)
	if len(lines) == 0 {
		// no lines at all, just print the label and we're done
		_, _ = fmt.Fprintln(w, label)
		return
	}

	// print the first line with the label prefixed
	_, _ = fmt.Fprintln(w, label+lines[0])

	// print the rest of the lines with an indentation the same length as the
	// label so they all line up nicely
	indent := strings.Repeat(" ", labelLen)
	for _, l := range lines[1:] {
		_, _ = fmt.Fprintln(w, indent+l)
	}
}

// wrapText word-wraps s into lines no wider than maxWidth runes. Words longer
// than maxWidth (typically URLs like docs links) go on their own line and
// overflow rather than break - a broken URL is worse than one that wraps.
func wrapText(s string, maxWidth int) []string {
	// break the string up into words first
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	var curLine strings.Builder
	curLen := 0

	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)

		switch {
		case curLen == 0:
			// nothing on the current line yet, add the entire word - we never
			// break a word apart
			curLine.WriteString(word)
			curLen = wordLen

		case curLen+1+wordLen > maxWidth:
			// adding the word to the current line would be too long, save the
			// current line to the output slice, reset the current line builder,
			// then add the word
			lines = append(lines, curLine.String())
			curLine.Reset()
			curLine.WriteString(word)
			curLen = wordLen

		default:
			// there's room for the word, add it to the current line with a
			// space before it
			curLine.WriteByte(' ')
			curLine.WriteString(word)
			curLen += 1 + wordLen
		}
	}

	if curLen > 0 {
		// add the last straggler line to the output slice
		lines = append(lines, curLine.String())
	}
	return lines
}

// JSONPrinter emits the report as pretty-printed JSON. This is much more simple
// than all the text formatting logic in the TextPrinter approach.
type JSONPrinter struct{}

// Print writes a report to the given writer using JSON encoding.
func (p *JSONPrinter) Print(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
