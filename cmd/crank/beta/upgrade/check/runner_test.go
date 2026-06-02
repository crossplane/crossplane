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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// errBoom is the generic error injected into mocks across the package's tests.
var errBoom = errors.New("boom")

// MockCheck is a configurable Check used to drive Runner tests without touching
// a real cluster.
var _ Check = &MockCheck{}

type MockCheck struct {
	meta     Meta
	findings []Finding
	err      error
	panicVal any
}

func (m *MockCheck) Meta() Meta { return m.meta }
func (m *MockCheck) Run(context.Context) ([]Finding, error) {
	if m.panicVal != nil {
		panic(m.panicVal)
	}
	return m.findings, m.err
}

// TestCheckMetadata locks the stable, machine-friendly Category identifiers and
// severities that downstream tooling and the non-zero-exit logic depend on, and
// guards against a check shipping with empty human-facing metadata.
func TestCheckMetadata(t *testing.T) {
	// metadata captures the stable identity of a check plus whether its
	// human-facing fields are populated. We compare presence (not the exact
	// long strings) so the test locks the contract without restating the source.
	type metadata struct {
		category       string
		severity       Severity
		hasTitle       bool
		hasDescription bool
		hasRemediation bool
		hasDocs        bool
	}
	// Clients for each check are not set, we expect metadata accessors to not touch the cluster.
	cases := map[string]struct {
		reason string
		check  Check
		want   metadata
	}{
		"NativePatchAndTransform": {
			reason: "Native P&T is an upgrade blocker.",
			check:  &NativePatchAndTransform{},
			want:   metadata{category: "native-patch-and-transform", severity: SeverityIssue, hasTitle: true, hasDescription: true, hasRemediation: true, hasDocs: true},
		},
		"ControllerConfig": {
			reason: "ControllerConfig usage is an upgrade blocker.",
			check:  &ControllerConfigCheck{},
			want:   metadata{category: "controller-config", severity: SeverityIssue, hasTitle: true, hasDescription: true, hasRemediation: true, hasDocs: true},
		},
		"ExternalSecretStores": {
			reason: "External secret stores usage is an upgrade blocker.",
			check:  &ExternalSecretStores{},
			want:   metadata{category: "external-secret-stores", severity: SeverityIssue, hasTitle: true, hasDescription: true, hasRemediation: true, hasDocs: true},
		},
		"CompositeConnectionDetails": {
			reason: "Composite connection details keep working post-upgrade, so this is informational.",
			check:  &CompositeConnectionDetails{},
			want:   metadata{category: "composite-connection-details", severity: SeverityInfo, hasTitle: true, hasDescription: true, hasRemediation: true, hasDocs: true},
		},
		"UnqualifiedPackageSources": {
			reason: "Unqualified package sources are an upgrade blocker.",
			check:  &UnqualifiedPackageSources{},
			want:   metadata{category: "unqualified-package-source", severity: SeverityIssue, hasTitle: true, hasDescription: true, hasRemediation: true, hasDocs: true},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := tc.check.Meta()
			got := metadata{
				category:       m.Category,
				severity:       m.Severity,
				hasTitle:       m.Title != "",
				hasDescription: m.Description != "",
				hasRemediation: m.Remediation != "",
				hasDocs:        len(m.DocsURLs) > 0,
			}
			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(metadata{})); diff != "" {
				t.Errorf("\n%s\ncheck metadata: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReportHasBlockers(t *testing.T) {
	type want struct {
		blockers bool
	}
	cases := map[string]struct {
		reason string
		report Report
		want   want
	}{
		"Empty": {
			reason: "A report with no categories has no blockers.",
			report: Report{},
			want:   want{blockers: false},
		},
		"InfoFindingsOnly": {
			reason: "Info-severity findings are not blockers.",
			report: Report{Categories: []CategoryResult{{
				Severity: SeverityInfo,
				Findings: []Finding{{Resource: ResourceRef{Kind: "X"}}},
			}}},
			want: want{blockers: false},
		},
		"IssueFindings": {
			reason: "Issue-severity findings are blockers.",
			report: Report{Categories: []CategoryResult{{
				Severity: SeverityIssue,
				Findings: []Finding{{Resource: ResourceRef{Kind: "X"}}},
			}}},
			want: want{blockers: true},
		},
		"IncompleteCheck": {
			reason: "A check that errored out is a blocker even with no findings.",
			report: Report{Categories: []CategoryResult{{
				Severity: SeverityIssue,
				Err:      "boom",
			}}},
			want: want{blockers: true},
		},
		"InfoWithError": {
			reason: "An info check that errored out is still a blocker: the user has no answer for that category.",
			report: Report{Categories: []CategoryResult{{
				Severity: SeverityInfo,
				Err:      "boom",
			}}},
			want: want{blockers: true},
		},
		"IssueCategoryNoFindings": {
			reason: "An issue-severity category that produced no findings and no error is clean.",
			report: Report{Categories: []CategoryResult{{
				Severity: SeverityIssue,
			}}},
			want: want{blockers: false},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.report.HasBlockers()
			if diff := cmp.Diff(tc.want.blockers, got); diff != "" {
				t.Errorf("\n%s\nHasBlockers(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunnerRun(t *testing.T) {
	type want struct {
		report Report
	}
	cases := map[string]struct {
		reason string
		checks []Check
		want   want
	}{
		"NoChecks": {
			reason: "A runner with no checks produces an empty report.",
			checks: nil,
			want:   want{report: Report{Categories: []CategoryResult{}}},
		},
		"MetadataAndFindings": {
			reason: "The runner copies each check's metadata onto its CategoryResult and carries its findings.",
			checks: []Check{
				&MockCheck{
					meta: Meta{
						Category:    "cat-a",
						Title:       "Title A",
						Severity:    SeverityIssue,
						Description: "desc",
						Remediation: "fix",
						DocsURLs:    []string{"https://example.com"},
					},
					findings: []Finding{{Resource: ResourceRef{Kind: "Kind", Name: "n"}}},
				},
			},
			want: want{report: Report{Categories: []CategoryResult{{
				Category:    "cat-a",
				Title:       "Title A",
				Severity:    SeverityIssue,
				Description: "desc",
				Remediation: "fix",
				DocsURLs:    []string{"https://example.com"},
				Findings:    []Finding{{Resource: ResourceRef{Kind: "Kind", Name: "n"}}},
			}}}},
		},
		"ErrorBecomesIncomplete": {
			reason: "A check that returns an error has its error recorded on Err, marking the category incomplete.",
			checks: []Check{
				&MockCheck{meta: Meta{Category: "cat-err", Severity: SeverityIssue}, err: errBoom},
			},
			want: want{report: Report{Categories: []CategoryResult{{
				Category: "cat-err",
				Severity: SeverityIssue,
				Err:      "boom",
			}}}},
		},
		"PanicBecomesIncomplete": {
			reason: "A check that panics is recovered into an Err, marking the category incomplete instead of crashing the run.",
			checks: []Check{
				&MockCheck{meta: Meta{Category: "cat-panic", Severity: SeverityIssue}, panicVal: "panic boom"},
			},
			want: want{report: Report{Categories: []CategoryResult{{
				Category: "cat-panic",
				Severity: SeverityIssue,
				Err:      "check panicked: panic boom",
			}}}},
		},
		"PanicDoesNotDropSiblingResults": {
			reason: "A panic in one check must not discard a healthy sibling check's findings.",
			checks: []Check{
				&MockCheck{meta: Meta{Category: "cat-panic", Severity: SeverityIssue}, panicVal: "panic boom"},
				&MockCheck{
					meta:     Meta{Category: "cat-ok", Severity: SeverityIssue},
					findings: []Finding{{Resource: ResourceRef{Kind: "Kind", Name: "n"}}},
				},
			},
			want: want{report: Report{Categories: []CategoryResult{
				{Category: "cat-panic", Severity: SeverityIssue, Err: "check panicked: panic boom"},
				{Category: "cat-ok", Severity: SeverityIssue, Findings: []Finding{{Resource: ResourceRef{Kind: "Kind", Name: "n"}}}},
			}}},
		},
		"PreservesCheckOrder": {
			reason: "Results are indexed by check position, so concurrent execution preserves input order.",
			checks: []Check{
				&MockCheck{meta: Meta{Category: "first"}},
				&MockCheck{meta: Meta{Category: "second"}},
			},
			want: want{report: Report{Categories: []CategoryResult{
				{Category: "first"},
				{Category: "second"},
			}}},
		},
		"SortsFindings": {
			reason: "Findings are sorted by kind, then namespace, then name, then field path.",
			checks: []Check{
				&MockCheck{
					meta: Meta{Category: "sort"},
					findings: []Finding{
						{Resource: ResourceRef{Kind: "B", Name: "a"}},
						{Resource: ResourceRef{Kind: "A", Namespace: "ns2", Name: "a"}},
						{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "z"}},
						{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "a"}, FieldPath: ".y"},
						{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "a"}, FieldPath: ".x"},
					},
				},
			},
			want: want{report: Report{Categories: []CategoryResult{{
				Category: "sort",
				Findings: []Finding{
					{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "a"}, FieldPath: ".x"},
					{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "a"}, FieldPath: ".y"},
					{Resource: ResourceRef{Kind: "A", Namespace: "ns1", Name: "z"}},
					{Resource: ResourceRef{Kind: "A", Namespace: "ns2", Name: "a"}},
					{Resource: ResourceRef{Kind: "B", Name: "a"}},
				},
			}}}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Runner{Checks: tc.checks, Logger: logging.NewNopLogger()}
			got := r.Run(context.Background())
			if diff := cmp.Diff(tc.want.report, got); diff != "" {
				t.Errorf("\n%s\nRunner.Run(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
