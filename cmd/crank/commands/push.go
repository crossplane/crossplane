/*
Copyright 2020 The Crossplane Authors.

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

// Package commands implements Crossplane CLI commands.
package commands

import (
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errGetwd           = "failed to get working directory while searching for package"
	errFindPackageinWd = "failed to find a package in current working directory"
)

// PushCmd pushes a package.
type PushCmd struct {
	Configuration PushConfigCmd   `cmd:"" help:"Push a Configuration package."`
	Provider      PushProviderCmd `cmd:"" help:"Push a Provider package."`

	Package string `short:"f" help:"Path to package. If not specified and only one package exists in current directory it will be used."`
}

// Run runs the push cmd.
func (c *PushCmd) Run(child *PushChild, logger logging.Logger) error {
	logger = logger.WithValues("tag", child.Tag)
	tag, err := name.NewTag(child.Tag)
	if err != nil {
		logger.Debug("Failed to create tag for package", "error", err)
		return err
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if c.Package == "" {
		logger.Debug("Trying to find package in current directory")
		wd, err := os.Getwd()
		if err != nil {
			logger.Debug("Failed to find package in directory", "error", errors.Wrap(err, errGetwd))
			return errors.Wrap(err, errGetwd)
		}
		path, err := xpkg.FindXpkgInDir(child.FS, wd)
		if err != nil {
			logger.Debug("Failed to find package in directory", "error", errors.Wrap(err, errFindPackageinWd))
			return errors.Wrap(err, errFindPackageinWd)
		}
		c.Package = path
		logger.Debug("Found package in directory", "path", path)
	}
	img, err := tarball.ImageFromPath(c.Package, nil)
	if err != nil {
		logger.Debug("Failed to create image from package tarball", "error", err)
		return err
	}
	if err := remote.Write(tag, img, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		logger.Debug("Failed to push created image to remote location", "error", err)
		return err
	}
	return nil
}

// PushChild provides context to the Push commands' hooks.
type PushChild struct {
	Tag string
	FS  afero.Fs
}

// PushConfigCmd pushes a Configuration.
type PushConfigCmd struct {
	Tag string `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
}

// AfterApply sets the tag for the parent push command.
func (c PushConfigCmd) AfterApply(p *PushChild) error {
	p.Tag = c.Tag
	return nil
}

// PushProviderCmd pushes a Provider.
type PushProviderCmd struct {
	Tag string `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
}

// AfterApply sets the tag for the parent push command.
func (c PushProviderCmd) AfterApply(p *PushChild) error {
	p.Tag = c.Tag
	return nil
}
