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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Cmd arguments and flags for render subcommand.
type Cmd struct {
	// Arguments.
	Extensions string `arg:"" help:"Extensions source which can be a file, directory, or '-' for standard input."`
	Resources  string `arg:"" help:"Resources source which can be a file, directory, or '-' for standard input."`

	// Flags. Keep them in alphabetical order.
	CacheDir           string `default:"~/.crossplane/cache"                                                       help:"Absolute path to the cache directory where downloaded schemas are stored."`
	CleanCache         bool   `help:"Clean the cache directory before downloading package schemas."`
	SkipSuccessResults bool   `help:"Skip printing success results."`
	CrossplaneImage    string `help:"Specify the Crossplane image to be used for validating the built-in schemas."`

	fs afero.Fs
}

// Help prints out the help for the validate command.
func (c *Cmd) Help() string {
	return `
This command validates the provided Crossplane resources against the schemas of the provided extensions like XRDs,
CRDs, providers, and configurations. The output of the "crossplane render" command can be
piped to this validate command in order to rapidly validate on the outputs of the composition development experience.

If providers or configurations are provided as extensions, they will be downloaded and loaded as CRDs before performing
validation. If the cache directory is not provided, it will default to "~/.crossplane/cache".
Cache directory can be cleaned before downloading schemas by setting the "clean-cache" flag.

All validation is performed offline locally using the Kubernetes API server's validation library, so it does not require
any Crossplane instance or control plane to be running or configured.

Examples:

  # Validate all resources in the resources.yaml file against the extensions in the extensions.yaml file
  crossplane beta validate extensions.yaml resources.yaml

  # Validate all resources in the resources.yaml file against the extensions in the extensions.yaml file using a specific Crossplane image version
  crossplane beta validate extensions.yaml resources.yaml --crossplane-image=xpkg.upbound.io/crossplane/crossplane:v1.16.0

  # Validate all resources in the resourceDir folder against the extensions in the extensionsDir folder and skip
  # success logs
  crossplane beta validate extensionsDir/ resourceDir/ --skip-success-results

  # Validate the output of the render command against the extensions in the extensionsDir folder
  crossplane render xr.yaml composition.yaml func.yaml --include-full-xr | crossplane beta validate extensionsDir/ -

  # Validate all resources in the resourceDir folder against the extensions in the extensionsDir folder using provided
  # cache directory and clean the cache directory before downloading schemas
  crossplane beta validate extensionsDir/ resourceDir/ --cache-dir .cache --clean-cache
`
}

// AfterApply implements kong.AfterApply.
func (c *Cmd) AfterApply() error {
	c.fs = afero.NewOsFs()
	return nil
}

// Run validate.
func (c *Cmd) Run(k *kong.Context, _ logging.Logger) error {
	if c.Resources == "-" && c.Extensions == "-" {
		return errors.New("cannot use stdin for both extensions and resources")
	}

	if len(c.CrossplaneImage) < 1 {
		c.CrossplaneImage = fmt.Sprintf("%s/crossplane/crossplane:%s", xpkg.DefaultRegistry, version.New().GetVersionString())
	}

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

	if strings.HasPrefix(c.CacheDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		c.CacheDir = filepath.Join(homeDir, c.CacheDir[2:])
	}

	m := NewManager(c.CacheDir, c.fs, k.Stdout, WithCrossplaneImage(c.CrossplaneImage))

	// Convert XRDs/CRDs to CRDs and add package dependencies
	if err := m.PrepExtensions(extensions); err != nil {
		return errors.Wrapf(err, "cannot prepare extensions")
	}

	// Download package base layers to cache and load them as CRDs
	if err := m.CacheAndLoad(c.CleanCache); err != nil {
		return errors.Wrapf(err, "cannot download and load cache")
	}

	// Validate resources against schemas
	if err := SchemaValidation(resources, m.crds, c.SkipSuccessResults, k.Stdout); err != nil {
		return errors.Wrapf(err, "cannot validate resources")
	}

	return nil
}
