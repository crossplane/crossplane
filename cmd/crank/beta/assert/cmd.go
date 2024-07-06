/*
Copyright 2023 The Crossplane Authors.

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
package assert

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	"github.com/spf13/afero"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments
	ExpectedResources string `arg:"" help:"A YAML file or directory of YAML files specifying the expected resources."`
	ActualResources   string `arg:"" help:"A YAML file or directory of YAML files specifying the actual resources to compare against the expected resources, or '-' for standard input." `

	// Flags. Keep them in alphabetical order.
	SkipSuccessResults bool `help:"Skip printing success results, showing only failures."`

	fs afero.Fs
}

func (c *Cmd) Help() string {
	return `
	This command compares expected resources against actual resources and reports
	any differences or failures. The output of the "crossplane beta render" command can be 
	piped to this assert command in order to rapidly assert on the outputs of the composition
	development experience.
	
	Examples:
	
      # Assert all resources in the actual.yaml file against the resources in the expected.yaml file
	  crossplane beta assert expected.yaml actual.yaml
	
      # Assert all resources in the actual.yaml file against the resources in the expected.yaml file
      skipping success logs
	  crossplane beta assert  expected.yaml actual.yaml --skip-success-results
	
	  # Assert the output of the render command against the resources in expected.yaml
	  crossplane beta render xr.yaml composition.yaml func.yaml | crossplane beta assert expected.yaml -
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run assert.
func (c *Cmd) Run(k *kong.Context, log logging.Logger) error {
	// Load all resources
	expectedLoader, err := validate.NewLoader(c.ExpectedResources)
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.ExpectedResources)
	}

	expectedResources, err := expectedLoader.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.ExpectedResources)
	}

	actualLoader, err := validate.NewLoader(c.ActualResources)
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.ActualResources)
	}

	actualResources, err := actualLoader.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.ActualResources)
	}

	if err := Assert(expectedResources, actualResources, c.SkipSuccessResults, k.Stdout); err != nil {
		return errors.Wrap(err, "error occurred during assertion")
	}

	return nil
}
