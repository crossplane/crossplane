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
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/test/e2e/config"
	"github.com/crossplane/crossplane/test/e2e/funcs"
)

// LabelAreaDiff is applied to all features pertaining to the diff command.
const LabelAreaDiff = "diff"

// RunDiff runs the crossplane diff command on the provided resources.
// It returns the output and any error encountered.
func RunDiff(t *testing.T, c *envconf.Config, crankPath string, resourcePaths ...string) (string, error) {
	t.Helper()

	var err error

	// Prepare the command to run
	args := append([]string{"--verbose", "beta", "diff", "-n", namespace}, resourcePaths...)
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
	if err != nil {
		t.Logf("Error running diff command: %v", err)
		t.Logf("stderr: %s", stderr.String())
	}

	return stdout.String(), err
}

// TestCrossplaneDiffCommand tests the functionality of the crossplane diff command.
func TestCrossplaneDiffCommand(t *testing.T) {
	manifests := "test/e2e/manifests/beta/diff"

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
				// Find the crank binary
				crankPath := funcs.FindCrankBinary(t)

				// Run the diff command on a new resource that doesn't exist yet
				output, err := RunDiff(t, c, crankPath, filepath.Join(manifests, "new-xr.yaml"))
				if err != nil {
					t.Fatalf("Error running diff command: %v", err)
				}

				// Verify the output contains the expected text for a new resource
				if !strings.Contains(output, "+++ XNopResource/new-resource") {
					t.Errorf("Expected diff output to show new XNopResource, got: %s", output)
				}

				// Verify the output contains the expected text for a composed resource
				if !strings.Contains(output, "+ NopResource") {
					t.Errorf("Expected diff output to show new NopResource, got: %s", output)
				}

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
				// Find the crank binary
				crankPath := funcs.FindCrankBinary(t)

				// Run the diff command on a modified existing resource
				output, err := RunDiff(t, c, crankPath, filepath.Join(manifests, "modified-xr.yaml"))
				if err != nil {
					t.Fatalf("Error running diff command: %v", err)
				}

				// Verify the output contains the expected text for a modified resource
				if !strings.Contains(output, "~ XNopResource/existing-resource") {
					t.Errorf("Expected diff output to show modified XNopResource, got: %s", output)
				}

				// Verify output contains patch-specific changes
				if !strings.Contains(output, "\"coolField\": \"I'm modified!\"") {
					t.Errorf("Expected diff output to show coolField modification, got: %s", output)
				}

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
