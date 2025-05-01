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

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaDiff is applied to all features pertaining to the diff command.
const LabelAreaDiff = "diff"

// RunDiff runs the crossplane diff command on the provided resources.
// It returns the output and any error encountered.
func RunDiff(t *testing.T, c *envconf.Config, crankPath string, resourcePaths ...string) (string, string, error) {
	t.Helper()

	var err error

	// Prepare the command to run
	args := append([]string{"--verbose", "beta", "diff", "--timeout=2m", "-n", namespace}, resourcePaths...)
	t.Logf("Running command: %s %s", crankPath, strings.Join(args, " "))
	cmd := exec.Command(crankPath, args...)
	cmd.Env = append(cmd.Env, "KUBECONFIG="+c.KubeconfigFile())
	t.Logf("ENV: %s %s", crankPath, strings.Join(cmd.Env, " "))

	// Capture standard output and error
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	return stdout.String(), stderr.String(), err
}

// TestCrossplaneDiffCommand tests the functionality of the crossplane diff command.
func TestCrossplaneDiffCommand(t *testing.T) {
	manifests := "test/e2e/manifests/beta/diff"
	crankPath := "./crank"

	environment.Test(t,
		// Create a test for a new resource - should show all resources being created
		features.NewWithDescription(t.Name()+"WithNewResource", "Test that we can diff against a net-new resource with `crossplane diff`").
			WithLabel(LabelArea, LabelAreaDiff).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			Assess("DiffNewResource", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				t.Helper()

				// Run the diff command on a new resource that doesn't exist yet
				output, log, err := RunDiff(t, c, crankPath, filepath.Join(manifests, "new-xr.yaml"))
				if err != nil {
					t.Fatalf("Error running diff command: %v\nLog output:\n%s", err, log)
				}

				assertDiffMatchesFile(t, output, filepath.Join(manifests, "expect/new-xr.ansi"), log)

				return ctx
			}).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),

		// Create a test for modifying an existing resource
		features.NewWithDescription(t.Name()+"WithExistingResource", "Test that we can diff against an existing resource with `crossplane diff`").
			WithLabel(LabelArea, LabelAreaDiff).
			WithLabel(LabelSize, LabelSizeSmall).
			WithLabel(config.LabelTestSuite, config.TestSuiteDefault).
			WithSetup("CreatePrerequisites", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "setup/*.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "setup/*.yaml"),
				funcs.ResourcesHaveConditionWithin(1*time.Minute, manifests, "setup/definition.yaml", apiextensionsv1.WatchingComposite()),
				funcs.ResourcesHaveConditionWithin(2*time.Minute, manifests, "setup/provider.yaml", pkgv1.Healthy(), pkgv1.Active()),
			)).
			WithSetup("CreateInitialResource", funcs.AllOf(
				funcs.ApplyResources(FieldManager, manifests, "existing-xr.yaml"),
				funcs.ResourcesCreatedWithin(30*time.Second, manifests, "existing-xr.yaml"),
				funcs.ResourcesHaveConditionWithin(5*time.Minute, manifests, "existing-xr.yaml", xpv1.Available()),
			)).
			Assess("DiffModifiedResource", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
				t.Helper()

				// Run the diff command on a modified existing resource
				output, log, err := RunDiff(t, c, crankPath, filepath.Join(manifests, "modified-xr.yaml"))
				if err != nil {
					t.Fatalf("Error running diff command: %v\n Log output:\n%s", err, log)
				}

				assertDiffMatchesFile(t, output, filepath.Join(manifests, "expect/existing-xr.ansi"), log)

				return ctx
			}).
			WithTeardown("DeleteResources", funcs.AllOf(
				funcs.DeleteResources(manifests, "existing-xr.yaml"),
				funcs.ResourcesDeletedWithin(2*time.Minute, manifests, "existing-xr.yaml"),
			)).
			WithTeardown("DeletePrerequisites", funcs.ResourcesDeletedAfterListedAreGone(3*time.Minute, manifests, "setup/*.yaml", nopList)).
			Feature(),
	)
}

// Regular expressions to match the dynamic parts.
var (
	resourceNameRegex        = regexp.MustCompile(`(existing-resource)-[a-z0-9]{5,}`)
	compositionRevisionRegex = regexp.MustCompile(`(xnopresources\.diff\.example\.org)-[a-z0-9]{7,}`)
	ansiEscapeRegex          = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// NormalizeLine replaces dynamic parts with fixed placeholders.
func normalizeLine(line string) string {
	// Remove ANSI escape sequences
	line = ansiEscapeRegex.ReplaceAllString(line, "")

	// Replace resource names with random suffixes
	line = resourceNameRegex.ReplaceAllString(line, "${1}-XXXXX")

	// Replace composition revision refs with random hash
	line = compositionRevisionRegex.ReplaceAllString(line, "${1}-XXXXXXX")

	// Trim trailing whitespace
	line = strings.TrimRight(line, " ")

	return line
}

// parseStringContent converts a string to raw and normalized lines.
func parseStringContent(content string) ([]string, []string) {
	var rawLines []string
	var normalizedLines []string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		rawLine := scanner.Text()
		rawLines = append(rawLines, rawLine)
		normalizedLines = append(normalizedLines, normalizeLine(rawLine))
	}

	return rawLines, normalizedLines
}

// AssertDiffMatchesFile compares a diff output with an expected file, ignoring dynamic parts.
func assertDiffMatchesFile(t *testing.T, actual, expectedSource, log string) {
	t.Helper()

	expected, err := os.ReadFile(expectedSource)
	if err != nil {
		t.Fatalf("Failed to read expected file: %v", err)
	}

	expectedRaw, expectedNormalized := parseStringContent(string(expected))
	actualRaw, actualNormalized := parseStringContent(actual)

	if len(expectedNormalized) != len(actualNormalized) {
		t.Errorf("Line count mismatch: expected %d lines in %s, got %d lines in output",
			len(expectedNormalized), expectedSource,
			len(actualNormalized))
	}

	maxLines := len(expectedNormalized)
	if len(actualNormalized) > maxLines {
		maxLines = len(actualNormalized)
	}

	failed := false
	for i := range maxLines {
		if i >= len(expectedNormalized) {
			t.Errorf("Line %d: Extra line in output: %s",
				i+1, makeStringReadable(actualRaw[i]))
			failed = true
			continue
		}
		if i >= len(actualNormalized) {
			t.Errorf("Line %d: Missing line in output: %s",
				i+1, makeStringReadable(expectedRaw[i]))
			failed = true
			continue
		}
		if expectedNormalized[i] != actualNormalized[i] {
			// ignore white space at end of lines
			// if strings.TrimRight(expectedNormalized[i], " ") == strings.TrimRight(actualNormalized[i], " ") {
			//	continue
			//}

			rawExpected := ""
			if i < len(expectedRaw) {
				rawExpected = expectedRaw[i]
			}

			rawActual := ""
			if i < len(actualRaw) {
				rawActual = actualRaw[i]
			}

			t.Errorf("Line %d mismatch:\n  Expected: %s\n  Actual:   %s\n\n"+
				"Raw Values (with escape chars visible):\n"+
				"  Expected Raw: %s\n"+
				"  Actual Raw:   %s",
				i+1,
				expectedNormalized[i],
				actualNormalized[i],
				makeStringReadable(rawExpected),
				makeStringReadable(rawActual))
			failed = true
		}
	}

	if failed {
		t.Fatalf("Log output:\n%s", log)
	}
}

// makeStringReadable converts a string to a form where all non-printable characters
// (including ANSI escape codes) are converted to visible escape sequences.
func makeStringReadable(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch {
		case r == '\x1b':
			result.WriteString("\\x1b")
		case r == '\n':
			result.WriteString("\\n")
		case r == '\r':
			result.WriteString("\\r")
		case r == '\t':
			result.WriteString("\\t")
		case r == ' ':
			result.WriteString("\\space")
		case !unicode.IsPrint(r):
			result.WriteString(fmt.Sprintf("\\x%02x", r))
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}
