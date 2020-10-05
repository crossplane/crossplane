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

package main

import (
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane/pkg/xpkg"
)

// pushCmd pushes a package.
type pushCmd struct {
	Configuration pushConfigCmd   `cmd:"" help:"Push a Configuration package."`
	Provider      pushProviderCmd `cmd:"" help:"Push a Provider package."`

	Package string `short:"f" help:"Path to package. If not specified and only one package exists in current directory it will be used."`
}

// Run runs the push cmd.
func (c *pushCmd) Run(child *childArg) error {
	tag, err := name.NewTag(child.strVal)
	if err != nil {
		return err
	}

	// If package is not defined, attempt to find single package in current
	// directory.
	if c.Package == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		path, err := xpkg.FindXpkgInDir(afero.NewOsFs(), wd)
		if err != nil {
			return err
		}
		c.Package = path
	}
	img, err := tarball.ImageFromPath(c.Package, nil)
	if err != nil {
		return err
	}
	return remote.Write(tag, img, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// pushConfigCmd pushes a Configuration.
type pushConfigCmd struct {
	Tag strChild `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
}

// Run runs the Configuration push cmd.
func (c *pushConfigCmd) Run() error {
	return nil
}

// pushProviderCmd pushes a Provider.
type pushProviderCmd struct {
	Tag strChild `arg:"" help:"Tag of the package to be pushed. Must be a valid OCI image tag."`
}

// Run runs the Provider push cmd.
func (c *pushProviderCmd) Run() error {
	return nil
}
