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
	"testing"

	"github.com/fatih/color"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewPrinter(t *testing.T) {
	type want struct {
		printer Printer
		err     error
	}
	cases := map[string]struct {
		reason string
		format string
		want   want
	}{
		"Text":    {reason: "\"text\" yields a TextPrinter.", format: "text", want: want{printer: &TextPrinter{}}},
		"Empty":   {reason: "An empty format defaults to text.", format: "", want: want{printer: &TextPrinter{}}},
		"JSON":    {reason: "\"json\" yields a JSONPrinter.", format: "json", want: want{printer: &JSONPrinter{}}},
		"Unknown": {reason: "An unknown format is an error.", format: "yaml", want: want{err: cmpopts.AnyError}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := NewPrinter(tc.format)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nNewPrinter(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.printer, p); diff != "" {
				t.Errorf("\n%s\nNewPrinter(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestJSONPrinterRoundTrip(t *testing.T) {
	report := Report{Categories: []CategoryResult{
		{
			Category:    "native-patch-and-transform",
			Title:       "Native patch-and-transform Compositions",
			Severity:    SeverityIssue,
			Description: "desc",
			Remediation: "fix",
			DocsURLs:    []string{"https://example.com"},
			Findings: []Finding{
				{Resource: ResourceRef{Group: "g", Kind: "K", Namespace: "ns", Name: "n"}, FieldPath: ".spec.mode"},
			},
		},
		{Category: "incomplete", Severity: SeverityIssue, Description: "d", Err: "boom"},
	}}

	var buf bytes.Buffer
	if err := (&JSONPrinter{}).Print(&buf, report); err != nil {
		t.Fatalf("Print() unexpected error: %v", err)
	}

	got := Report{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decoding printed JSON: %v", err)
	}
	if diff := cmp.Diff(report, got); diff != "" {
		t.Errorf("\nJSONPrinter round-trips the report unchanged: -want, +got:\n%s", diff)
	}
}

func TestWrapText(t *testing.T) {
	cases := map[string]struct {
		reason   string
		s        string
		maxWidth int
		want     []string
	}{
		"Empty": {
			reason:   "An empty string yields no lines.",
			s:        "",
			maxWidth: 10,
			want:     nil,
		},
		"SingleWord": {
			reason:   "A word shorter than the width stays on one line.",
			s:        "hello",
			maxWidth: 10,
			want:     []string{"hello"},
		},
		"WrapsAtWidth": {
			reason:   "Words wrap once the line would exceed the width.",
			s:        "alpha beta gamma",
			maxWidth: 10,
			want:     []string{"alpha beta", "gamma"},
		},
		"LongWordOverflow": {
			reason:   "A word longer than the width gets its own line rather than being broken.",
			s:        "hi supercalifragilistic",
			maxWidth: 10,
			want:     []string{"hi", "supercalifragilistic"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := wrapText(tc.s, tc.maxWidth)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nwrapText(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWriteWrapped(t *testing.T) {
	// Each word is 9 runes, so word + 1 space = 10. The wrap widths below are
	// multiples of 10 (contentWidth 40), so it's a clean 4 words per line with
	// the break landing at exactly 39 - no off-by-one math when writing wants.
	const body = "wordnine1 wordnine2 wordnine3 wordnine4 wordnine5"

	cases := map[string]struct {
		reason   string
		label    string
		body     string
		maxWidth int
		want     string
	}{
		"WrappingDisabled": {
			reason:   "A non-positive width prints label+body verbatim on one line.",
			label:    "Fix:   ",
			body:     body,
			maxWidth: 0,
			want:     "Fix:   " + body + "\n",
		},
		"NarrowContentNotWrapped": {
			reason: "When the content column is narrower than the wrap floor, the body is printed unwrapped.",
			label:  "Fix:   ",
			body:   body,
			// After the label (7 chars) there are only 13 columns left for the body, which is below
			// the 40-column wrap floor (minWrapWidth), so the body is printed whole instead of wrapped.
			maxWidth: 20,
			want:     "Fix:   " + body + "\n",
		},
		"WrapsWithoutLabel": {
			reason:   "With no label, the body wraps at the given width and no indent on subsequent lines.",
			label:    "",
			body:     body,
			maxWidth: 40,
			want:     "wordnine1 wordnine2 wordnine3 wordnine4\nwordnine5\n",
		},
		"WrapsWithLabelIndent": {
			reason:   "Continuation lines indent under the label.",
			label:    "Fix:   ", // 7 runes; contentWidth 40
			body:     body,
			maxWidth: 47, // 7 (label) + 40 content width: wraps at the same 40 cols as the no-label case, isolating the indent
			want:     "Fix:   wordnine1 wordnine2 wordnine3 wordnine4\n       wordnine5\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			writeWrapped(&buf, tc.label, tc.body, tc.maxWidth)
			if diff := cmp.Diff(tc.want, buf.String()); diff != "" {
				t.Errorf("\n%s\nwriteWrapped(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGroupByKind(t *testing.T) {
	// Names encode each finding's Group+Kind: g1A1 and g1A2 share Group+Kind
	// (g1/A) and differ only by name; g1B differs by Kind; g2A is the same Kind
	// as the g1/A pair but a different Group, so it must not merge with them.
	g1A1 := Finding{Resource: ResourceRef{Group: "g1", Kind: "A", Name: "a1"}}
	g1A2 := Finding{Resource: ResourceRef{Group: "g1", Kind: "A", Name: "a2"}}
	g1B := Finding{Resource: ResourceRef{Group: "g1", Kind: "B", Name: "b"}}
	g2A := Finding{Resource: ResourceRef{Group: "g2", Kind: "A", Name: "a"}}

	cases := map[string]struct {
		reason   string
		findings []Finding
		want     [][]Finding
	}{
		"Empty": {
			reason:   "No findings yields no groups.",
			findings: nil,
			want:     nil,
		},
		"MergesSameGroupKind": {
			reason:   "Findings sharing a Group+Kind collapse into one group.",
			findings: []Finding{g1A1, g1A2},
			want:     [][]Finding{{g1A1, g1A2}},
		},
		"SeparatesSameKindDifferentGroup": {
			reason:   "A matching Kind under a different Group is its own group.",
			findings: []Finding{g1A1, g2A},
			want:     [][]Finding{{g1A1}, {g2A}},
		},
		"PreservesFirstSeenOrder": {
			reason:   "Groups appear in the order each Group+Kind was first seen, even when members are interleaved.",
			findings: []Finding{g1A1, g1B, g1A2, g2A},
			want:     [][]Finding{{g1A1, g1A2}, {g1B}, {g2A}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := groupByKind(tc.findings)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\ngroupByKind(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	cases := map[string]struct {
		reason     string
		categories []CategoryResult
		want       []int // [issues, info, incomplete]
	}{
		"Empty": {
			reason:     "No categories tallies to all zeros.",
			categories: nil,
			want:       []int{0, 0, 0},
		},
		"CountsIssues": {
			reason:     "Findings in a severity issue category count as issues.",
			categories: []CategoryResult{{Severity: SeverityIssue, Findings: []Finding{{}, {}}}},
			want:       []int{2, 0, 0},
		},
		"CountsInfo": {
			reason:     "Findings in an severity info category count as informational, not issues.",
			categories: []CategoryResult{{Severity: SeverityInfo, Findings: []Finding{{}, {}, {}}}},
			want:       []int{0, 3, 0},
		},
		"CountsIncomplete": {
			reason:     "A category that errored counts as one incomplete check.",
			categories: []CategoryResult{{Severity: SeverityIssue, Err: "boom"}},
			want:       []int{0, 0, 1},
		},
		"ErrWithFindingsCountsBoth": {
			reason:     "A category with both an error and findings is counted as incomplete but still has its findings tallied.",
			categories: []CategoryResult{{Severity: SeverityIssue, Err: "boom", Findings: []Finding{{}, {}}}},
			want:       []int{2, 0, 1},
		},
		"Mixed": {
			reason: "Issue, info, and incomplete tallies accumulate independently across categories.",
			categories: []CategoryResult{
				{Severity: SeverityIssue, Findings: []Finding{{}, {}}},
				{Severity: SeverityInfo, Findings: []Finding{{}, {}, {}}},
				{Severity: SeverityIssue, Err: "boom"},
			},
			want: []int{2, 3, 1},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			issues, info, incomplete := summarize(tc.categories)
			got := []int{issues, info, incomplete}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nsummarize() tallies [issues, info, incomplete]: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	cases := map[string]struct {
		reason string
		n      int
		want   string
	}{
		"Zero": {reason: "Zero uses the plural form.", n: 0, want: "0 issues"},
		"One":  {reason: "One uses the singular form.", n: 1, want: "1 issue"},
		"Many": {reason: "More than one uses the plural form.", n: 3, want: "3 issues"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := pluralize(tc.n, "issue", "issues")
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\npluralize(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLinePrefixWriter(t *testing.T) {
	cases := map[string]struct {
		reason string
		writes []string
		want   string
	}{
		"MultiLineSingleWrite": {
			reason: "Each line written in one call gets the prefix; no trailing prefix after a final newline-less segment.",
			writes: []string{"a\nb"},
			want:   "> a\n> b",
		},
		"TrailingNewline": {
			reason: "A trailing newline does not eagerly emit a prefix for a line not yet started.",
			writes: []string{"a\nb\n"},
			want:   "> a\n> b\n",
		},
		"SplitWrites": {
			reason: "A line split across multiple Write calls still gets exactly one prefix; the prefix is not re-emitted mid-line.",
			writes: []string{"a", "b\n"},
			want:   "> ab\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			lw := newLinePrefixWriter(&buf, "> ")
			for _, w := range tc.writes {
				if _, err := lw.Write([]byte(w)); err != nil {
					t.Fatalf("Write() unexpected error: %v", err)
				}
			}
			if diff := cmp.Diff(tc.want, buf.String()); diff != "" {
				t.Errorf("\n%s\nlinePrefixWriter.Write(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTextPrinter(t *testing.T) {
	// Pin colors off so the output is plain text. A bytes.Buffer also disables
	// wrapping (detectBodyWidth returns 0 for a non-*os.File writer), keeping
	// the layout deterministic.
	defer func(prev bool) { color.NoColor = prev }(color.NoColor)
	color.NoColor = true

	type want struct {
		output string
		err    error
	}
	cases := map[string]struct {
		reason string
		report Report
		want   want
	}{
		"Empty": {
			reason: "An empty report prints nothing - no verdict line, no categories.",
			report: Report{},
			want:   want{output: ""},
		},
		"MixedReport": {
			reason: "A healthy category is a single confirmation line; issue/info/incomplete categories render a sidebar body with description, fix, docs, and findings.",
			report: Report{Categories: []CategoryResult{
				{
					Category: "ok",
					Title:    "Healthy check",
				},
				{
					Category: "issue", Title: "Issue check", Severity: SeverityIssue,
					Description: "Something is wrong.",
					Remediation: "Fix it.",
					DocsURLs:    []string{"https://docs.example.com/page"},
					Findings: []Finding{
						{Resource: ResourceRef{Group: "example.org", Kind: "Thing", Name: "n"}, FieldPath: ".spec.x"},
					},
				},
				{
					Category: "info", Title: "Info check", Severity: SeverityInfo,
					Findings: []Finding{{Resource: ResourceRef{Group: "example.org", Kind: "Thing", Name: "m"}}},
				},
				{
					Category: "incomplete",
					Title:    "Incomplete check",
					Severity: SeverityIssue, Err: "it broke",
				},
			}},
			want: want{output: `[✗] 1 issue, 1 informational, 1 incomplete check.

[✓] Healthy check
[✗] Issue check - 1 issue
    │
    │  Something is wrong.
    │  Fix:   Fix it.
    │  Docs:  https://docs.example.com/page
    │
    │    NAME                  FIELD
    │    thing.example.org/n   .spec.x
    └──
[i] Info check - 1 informational finding
    │
    │    NAME                  FIELD
    │    thing.example.org/m   -
    └──
[!] Incomplete check - incomplete check
    │
    │  Error: it broke
    └──
`},
		},
		"NamespacedFindings": {
			reason: "Findings with a namespace render a NAMESPACE column; a finding missing a namespace or field path renders \"-\".",
			report: Report{Categories: []CategoryResult{
				{
					Category: "ess", Title: "Namespaced check", Severity: SeverityIssue,
					Findings: []Finding{
						{Resource: ResourceRef{Group: "example.org", Kind: "Thing", Namespace: "team-a", Name: "n"}, FieldPath: ".spec.x"},
						{Resource: ResourceRef{Group: "example.org", Kind: "Thing", Name: "no-ns"}},
					},
				},
			}},
			want: want{output: `[✗] 2 issues, 0 informational, 0 incomplete checks.

[✗] Namespaced check - 2 issues
    │
    │    NAMESPACE   NAME                      FIELD
    │    team-a      thing.example.org/n       .spec.x
    │    -           thing.example.org/no-ns   -
    └──
`},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			err := (&TextPrinter{}).Print(&buf, tc.report)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPrint(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.output, buf.String()); diff != "" {
				t.Errorf("\n%s\nPrint(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
