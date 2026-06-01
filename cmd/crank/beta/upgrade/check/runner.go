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
	"fmt"
	"runtime/debug"
	"sort"
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Severity classifies findings by whether they block an upgrade.
type Severity string

const (
	// SeverityIssue marks findings that must be addressed before upgrading,
	// e.g. if you upgrade a control plane that has these findings, you'll be in
	// a broken state after the upgrade. Issue-severity findings cause the
	// command to exit non-zero.
	SeverityIssue Severity = "issue"
	// SeverityInfo marks findings that are informational only. The underlying
	// behavior keeps working after the upgrade; the finding surfaces work the
	// user may want to do for forward-looking migration. Info findings do not
	// cause a non-zero exit.
	SeverityInfo Severity = "info"
)

// ResourceRef identifies the offending resource.
type ResourceRef struct {
	Group     string `json:"group,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// Finding describes one thing the user should address before upgrading, including both the
// offending resource and the offending field path within the resource (if applicable).
type Finding struct {
	Resource  ResourceRef `json:"resource,omitempty"`
	FieldPath string      `json:"fieldPath,omitempty"`
}

// CategoryResult groups findings produced by a single Check. The
// help/remediation content lives here because they apply for all findings in
// the category.
type CategoryResult struct {
	Category    string    `json:"category"`
	Title       string    `json:"title"`
	Severity    Severity  `json:"severity,omitempty"`
	Description string    `json:"description"`
	Remediation string    `json:"remediation,omitempty"`
	DocsURLs    []string  `json:"docsURLs,omitempty"` //nolint:tagliatelle // URLs is a Go initialism; goCamel's suggested "docsUrLs" is a worse JSON key.
	Findings    []Finding `json:"findings,omitempty"`
	Err         string    `json:"error,omitempty"`
}

// Report is the full combined output of a run including all the categories/checks/findings.
type Report struct {
	Categories []CategoryResult `json:"categories"`
}

// HasBlockers reports whether the run produced anything that the user really should address before
// upgrading to v2 and should therefore trigger a non-zero exit: any issue-severity finding, or an
// incomplete check (one that errored out before producing a result). An incomplete check counts
// because the user is left without an answer for that category. Info-severity findings are not
// blockers.
func (r Report) HasBlockers() bool {
	for _, c := range r.Categories {
		if c.Err != "" {
			return true
		}
		if c.Severity != SeverityInfo && len(c.Findings) > 0 {
			return true
		}
	}
	return false
}

// Meta is the static metadata describing a Check: identity, severity, and
// the help/remediation content shown for all of its findings.
type Meta struct {
	// Category is a stable, machine-friendly identifier for the check.
	Category string
	// Title is a short human-readable title shown in output.
	Title string
	// Severity is the severity of findings produced by the check.
	Severity Severity
	// Description explains what the check looks for.
	Description string
	// Remediation is one-line, action-oriented advice for the whole category.
	// Must not contain URLs - see DocsURLs.
	Remediation string
	// DocsURLs are documentation links for the category. Empty when none apply.
	DocsURLs []string
}

// Check is a single upgrade compatibility check.
type Check interface {
	// Meta returns the check's static metadata.
	Meta() Meta
	// Run executes the check and returns any findings.
	Run(ctx context.Context) ([]Finding, error)
}

// Runner executes checks and aggregates results.
type Runner struct {
	Checks []Check
	Logger logging.Logger
}

// Run executes all checks concurrently. A check that errors out is captured
// on its CategoryResult.Err and surfaces as an incomplete check; the Runner
// itself does not fail.
func (r *Runner) Run(ctx context.Context) Report {
	results := make([]CategoryResult, len(r.Checks))
	var wg sync.WaitGroup

	// launch every check on a separate go-routine
	for i, c := range r.Checks {
		wg.Add(1)
		go func(i int, c Check) {
			defer wg.Done()
			m := c.Meta()
			r.Logger.Debug("running check", "category", m.Category)
			res := CategoryResult{
				Category:    m.Category,
				Title:       m.Title,
				Severity:    m.Severity,
				Description: m.Description,
				Remediation: m.Remediation,
				DocsURLs:    m.DocsURLs,
			}

			// Recover any panics in this goroutine into an incomplete-check error so one bad check
			// can't crash the CLI and discard every other check's results. This mirrors how we
			// treat a check that returns an error below.
			defer func() {
				if rec := recover(); rec != nil {
					// Keep the user-facing error concise, but add the full stack in the debug log
					res.Err = fmt.Sprintf("check panicked: %v", rec)
					r.Logger.Debug("check panicked", "category", m.Category, "panic", rec, "stack", string(debug.Stack()))
					results[i] = res
				}
			}()

			// run the check and save any error we encounter, which means the check is incomplete
			findings, err := c.Run(ctx)
			if err != nil {
				res.Err = err.Error()
				r.Logger.Debug("check incomplete", "category", m.Category, "error", err)
			}

			// sort the findings on kind -> namespace -> name -> field path
			sort.SliceStable(findings, func(a, b int) bool {
				if findings[a].Resource.Kind != findings[b].Resource.Kind {
					return findings[a].Resource.Kind < findings[b].Resource.Kind
				}
				if findings[a].Resource.Namespace != findings[b].Resource.Namespace {
					return findings[a].Resource.Namespace < findings[b].Resource.Namespace
				}
				if findings[a].Resource.Name != findings[b].Resource.Name {
					return findings[a].Resource.Name < findings[b].Resource.Name
				}
				return findings[a].FieldPath < findings[b].FieldPath
			})

			res.Findings = findings
			results[i] = res
		}(i, c)
	}

	// wait until all the checks have finished
	wg.Wait()
	return Report{Categories: results}
}
