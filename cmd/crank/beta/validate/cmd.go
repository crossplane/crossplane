/*
Copyright 2024 The Crossplane Authors.

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

// Package validate implements offline schema validation of Crossplane resources.
package validate

import (
	"github.com/alecthomas/kong"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments.
	Extensions string `arg:"" help:"Extensions source which can be a file, directory, or '-' for standard input."`
	Resources  string `arg:"" help:"Resources source which can be a file, directory, or '-' for standard input."`

	// Flags. Keep them in alphabetical order.
	SkipSuccessResults bool `help:"Skip printing success results."`
}

// Help prints out the help for the render command.
func (c *Cmd) Help() string {
	return `
This command validates the provided Crossplane resources against the schemas of the provided extensions (e.g., XRDs and 
CRDs, as well as Providers and Configurations coming soon). The output of the "crossplane beta render" command can be 
piped to this validate command in order to rapidly validate on the outputs of the composition development experience.

All validation is performed offline locally using the Kubernetes API server's validation library, so it does not require 
any Crossplane instance or control plane to be running or configured.

Examples:

  # Validate all resources in the resources.yaml file against the extensions in the extensions.yaml file
  crossplane beta validate extensions.yaml resources.yaml

  # Validate all resources in the resourceDir folder against the extensions in the extensionsDir folder and skip 
  # success logs
  crossplane beta validate extensionsDir/ resourceDir/ --skip-success-logs
 
  # Validate the output of the render command against the extensions in the extensionsDir folder
  crossplane beta render xr.yaml composition.yaml func.yaml | crossplane beta validate extensionsDir/ -
`
}

// Run validate.
func (c *Cmd) Run(_ *kong.Context, _ logging.Logger) error {
	// Load all extensions
	extensionLoader, err := NewLoader(c.Extensions)
	if err != nil {
		return errors.Wrapf(err, "cannot load extensions from %q", c.Extensions)
	}

	extensions, err := extensionLoader.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load extensions from %q", c.Extensions)
	}

	// Load all resources
	resourceLoader, err := NewLoader(c.Resources)
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.Resources)
	}

	resources, err := resourceLoader.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load resources from %q", c.Resources)
	}

	// Convert all extensions to CRDs to extract their OpenAPI schema validators
	crds, err := convertExtensionsToCRDs(extensions)
	if err != nil {
		return errors.Wrapf(err, "cannot convert XRDs to CRDs")
	}

	// Validate all resources against their CRDs and XRDs
	if err := validateResources(resources, crds, c.SkipSuccessResults); err != nil {
		return errors.Wrapf(err, "cannot validate resources")
	}

	return nil
}
