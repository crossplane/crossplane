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

package xpkg

import (
	"path"
	"path/filepath"

	"github.com/Masterminds/semver"
	"github.com/pterm/pterm"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/input"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/meta"
)

const (
	errAlreadyExistsFmt   = "directory contains pre-existing meta file: %s"
	errInvalidPackageType = "the provided package type %q is invalid; valid types: configuration,provider"
)

// BeforeApply sets default values in init before assignment and validation.
func (c *initCmd) BeforeApply() error {
	c.prompter = input.NewPrompter()
	return nil
}

// AfterApply constructs and binds Upbound-specific context to any subcommands
// that have Run() methods that receive it.
func (c *initCmd) AfterApply() error { //nolint:gocyclo //the complexity is just over 10, and in part due to the switch statement
	c.fs = afero.NewOsFs()
	root, err := filepath.Abs(c.PackageRoot) // root
	if err != nil {
		return err
	}

	c.root = root
	if err := c.metaFileInRoot(); err != nil {
		return err
	}

	// validate provided package type
	if !xpkg.Package(c.Type).IsValid() {
		return errors.Errorf(errInvalidPackageType, c.Type)
	}

	// common init
	err = c.initCommon()
	if err != nil {
		return err
	}
	// type specific init
	switch c.Type {
	case string(xpkg.Configuration):
		if err := c.initConfigPkg(); err != nil {
			return err
		}
	case string(xpkg.Provider):
		if err := c.initProviderPkg(); err != nil {
			return err
		}
	case string(xpkg.Function):
		if err := c.initFunctionPkg(); err != nil {
			return err
		}
	}

	return nil
}

// buildCmd builds a crossplane package.
type initCmd struct {
	ctx      xpkg.InitContext
	fs       afero.Fs
	prompter input.Prompter
	root     string

	PackageRoot string `optional:"" short:"p" help:"Path to directory to write new package." default:"."`
	Type        string `optional:"" short:"t" help:"Type of package to be initialized." default:"configuration" enum:"configuration,provider,function"`
}

// Run executes the init command.
func (c *initCmd) Run(p pterm.TextPrinter) error {
	fileBody := []byte{}
	var err error

	switch c.Type {
	case string(xpkg.Configuration):
		fileBody, err = meta.NewConfigXPkg(c.ctx)
		if err != nil {
			return err
		}
	case string(xpkg.Provider):
		fileBody, err = meta.NewProviderXPkg(c.ctx)
		if err != nil {
			return err
		}
	case string(xpkg.Function):
		fileBody, err = meta.NewFunctionXPkg(c.ctx)
		if err != nil {
			return err
		}
	}

	writer := xpkg.NewFileWriter(
		xpkg.WithFs(c.fs),
		xpkg.WithRoot(c.root),
		xpkg.WithFileBody(fileBody),
	)

	// write out file named crossplane.yaml to the configured location
	if err := writer.NewMetaFile(); err != nil {
		return err
	}

	p.Printfln("xpkg initialized at %s", path.Join(c.root, xpkg.MetaFile))
	return nil
}

func (c *initCmd) initCommon() error {
	name, err := c.prompter.Prompt("Package name", false)
	if err != nil {
		return err
	}
	c.ctx.Name = name

	xpv, err := c.prompter.Prompt("What version contraints of Crossplane will this package be compatible with? [e.g. v1.0.0, >=v1.0.0-0, etc.]", false)
	if err != nil {
		return err
	}
	// validate semver constraint if set
	if xpv != "" {
		_, err = semver.NewConstraint(xpv)
		if err != nil {
			return err
		}
	}
	c.ctx.XPVersion = xpv

	return nil
}

func (c *initCmd) initConfigPkg() error {
	// dependsOn loop
	include, err := c.prompter.Prompt("Add dependencies? [y/n]", false)
	if err != nil {
		return err
	}

	if input.Yes(include) {
		for {
			provider, err := c.prompter.Prompt("Provider URI [e.g. crossplane/provider-aws]", false)
			if err != nil {
				return err
			}

			version, err := c.prompter.Prompt("Version constraints [e.g. 1.0.0, >=1.0.0-0, etc.]", false)
			if err != nil {
				return err
			}

			// validate semver constraint
			_, err = semver.NewConstraint(version)
			if err != nil {
				return err
			}

			c.ctx.DependsOn = append(c.ctx.DependsOn, v1.Dependency{
				Provider: &provider,
				Version:  version,
			})

			done, err := c.prompter.Prompt("Done? [y/n]", false)
			if err != nil {
				return err
			}
			if input.Yes(done) {
				break
			}
		}
	}
	return nil
}

func (c *initCmd) initProviderPkg() error {
	image, err := c.prompter.Prompt("Controller image", false)
	if err != nil {
		return err
	}
	c.ctx.Image = image

	return nil
}

func (c *initCmd) initFunctionPkg() error {
	image, err := c.prompter.Prompt("Function image", false)
	if err != nil {
		return err
	}
	c.ctx.Image = image

	return nil
}

func (c *initCmd) metaFileInRoot() error {
	// validate if current directory does not contain crossplane.yaml
	exists, err := afero.Exists(c.fs, filepath.Join(c.root, xpkg.MetaFile))
	if err != nil {
		return err
	}

	if exists {
		return errors.Errorf(errAlreadyExistsFmt, xpkg.MetaFile)
	}
	return nil
}
