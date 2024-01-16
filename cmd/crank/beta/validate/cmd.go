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
	SkipSuccessLogs bool `help:"Skip printing success logs."`
}

// Help prints out the help for the render command.
func (c *Cmd) Help() string {
	return `
This command validates Crossplane resources based on the extensions (e.g., XRDs, CRDs, providers, or configurations)
provided. Currently, it does not support Crossplane packages like providers, or configurations. 
It can be piped after the "crossplane beta render" command to improve composition authoring. It doesn't talk 
to Crossplane or any control plane. Instead it uses Kubernetes API server's validation library to provide offline schema
validation.

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

	// Convert all XRDs to CRDs if exist
	crds, err := convertToCRDs(extensions)
	if err != nil {
		return errors.Wrapf(err, "cannot convert XRDs to CRDs")
	}

	// Validate all resources against their CRDs and XRDs
	if err := validateResources(resources, crds, c.SkipSuccessLogs); err != nil {
		return errors.Wrapf(err, "cannot validate resources")
	}

	return nil
}
