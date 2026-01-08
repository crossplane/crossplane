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

// Package test implements composite resource rendering and testing.
package test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// Cmd arguments and flags for alpha render test subcommand.
type Cmd struct {
	// Arguments.
	TestDir string `arg:"" default:"tests" help:"Directory containing test cases." type:"path"`

	// Flags. Keep them in alphabetical order.
	FunctionAnnotations  []string      `help:"Override function annotations for all functions. Can be repeated." short:"a"`
	FunctionsFile        string        `help:"Path to functions file for function resolution."`
	OutputFile           string        `default:"expected.yaml"                                                  help:"Name of the output file (used when not comparing)."`
	PackageFile          string        `help:"Path to package file for resolving function versions."`
	Timeout              time.Duration `default:"1m"                                                             help:"How long to run before timing out."`
	WriteExpectedOutputs bool          `default:"false"                                                          help:"Write/update expected.yaml files instead of comparing." short:"w"`

	fs afero.Fs
}

// Help prints out the help for the alpha render test command.
func (c *Cmd) Help() string {
	return `
Render composite resources (XRs) and assert results.

This command renders XRs and compares them with expected outputs by default.
Use --write-expected-outputs to generate/update expected.yaml files.

Function resolution (at least one is required):
  - Provide --package-file to resolve functions from a package file
  - Provide --functions-file to load functions from a specific file
  - If both are provided, the functions-file takes precedence over the package file for any overlapping functions
  - If neither is provided, the composition must not reference any functions

Function annotations:
  - Use --function-annotations to override annotations for all functions
  - Useful for setting network configuration, environment variables, etc.

Examples:

    # Compare actual outputs with expected.yaml files (default)
    crossplane alpha render test --functions-file=functions.yaml

	# Generate/update expected.yaml files
    crossplane alpha render test --functions-file=functions.yaml --write-expected-outputs

	# Use package.yaml to auto-resolve function versions
    crossplane alpha render test --package-file=apis/package.yaml

	# Use both: package.yaml for defaults, custom functions file for overrides
    crossplane alpha render test --package-file=apis/package.yaml --functions-file=functions.yaml

	# Use custom Docker network for all functions
    crossplane alpha render test --package-file=apis/package.yaml \
      -a render.crossplane.io/runtime-docker-network=devnet

    # Test a specific directory
    crossplane alpha render test tests/my-test --functions-file=functions.yaml

    # Generate outputs with a different filename
    crossplane alpha render test --functions-file=functions.yaml --write-expected-outputs --output-file=snapshot.yaml
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run alpha render test.
func (c *Cmd) Run(_ *kong.Context, log logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// Run the test
	result, err := Test(ctx, log, Inputs{
		TestDir:              c.TestDir,
		FileSystem:           c.fs,
		WriteExpectedOutputs: c.WriteExpectedOutputs,
		OutputFile:           c.OutputFile,
		PackageFile:          c.PackageFile,
		FunctionsFile:        c.FunctionsFile,
		FunctionAnnotations:  c.FunctionAnnotations,
	})
	if err != nil {
		return err
	}

	if !result.Pass {
		return errors.New("test failed: differences found between expected and actual outputs")
	}
	if !c.WriteExpectedOutputs {
		_, _ = fmt.Fprintln(os.Stdout, "All tests passed")
	}

	return nil
}
