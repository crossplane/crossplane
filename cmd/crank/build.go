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
	"context"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/pkg/xpkg"
)

const (
	errGetNameFromMeta = "failed to get name from crossplane.yaml"
	errBuildPackage    = "failed to build package"
	errImageDigest     = "failed to get package digest"
	errCreatePackage   = "failed to create package file"
)

// buildCmd builds a package.
type buildCmd struct {
	Configuration buildConfigCmd   `cmd:"" help:"Build a Configuration package."`
	Provider      buildProviderCmd `cmd:"" help:"Build a Provider package."`

	PackageRoot string   `short:"f" help:"Path to crossplane.yaml" default:"."`
	Ignore      []string `help:"Paths, specified relative to --package-root, to exclude from the package."`
}

// Run runs the build cmd.
func (c *buildCmd) Run(child *buildChild) error {
	root, err := filepath.Abs(c.PackageRoot)
	if err != nil {
		return err
	}

	pkgName := child.name
	if pkgName == "" {
		metaPath := filepath.Join(root, xpkg.MetaFile)
		pkgName, err = xpkg.ParseNameFromMeta(child.fs, metaPath)
		if err != nil {
			return errors.Wrap(err, errGetNameFromMeta)
		}
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return errors.New("cannot build meta scheme for package parser")
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}
	img, err := xpkg.Build(context.Background(),
		parser.NewFsBackend(child.fs, parser.FsDir(root), parser.FsFilters(buildFilters(c.Ignore)...)),
		parser.New(metaScheme, objScheme),
		child.linter)
	if err != nil {
		return errors.Wrap(err, errBuildPackage)
	}

	hash, err := img.Digest()
	if err != nil {
		return errors.Wrap(err, errImageDigest)
	}

	f, err := child.fs.Create(xpkg.BuildPath(root, xpkg.FriendlyID(pkgName, hash.Hex)))
	if err != nil {
		return errors.Wrap(err, errCreatePackage)
	}
	defer func() { _ = f.Close() }()
	return tarball.Write(nil, img, f)
}

// default build filters skip directories and files without YAML extension in
// addition to any paths specified.
func buildFilters(skips []string) []parser.FilterFn {
	numOpts := len(skips) + 2
	opts := make([]parser.FilterFn, numOpts)
	opts[0] = parser.SkipDirs()
	opts[1] = parser.SkipNotYAML()
	for i, s := range skips {
		opts[i+2] = parser.SkipPath(s)
	}
	return opts
}

type buildChild struct {
	name   string
	linter parser.Linter
	fs     afero.Fs
}

// buildConfigCmd builds a Configuration.
type buildConfigCmd struct {
	Name string `optional:"" help:"Name of the package to be built. Uses name in crossplane.yaml if not specified. Does not correspond to package tag."`
}

// Run runs the Configuration build cmd.
func (c *buildConfigCmd) Run() error {
	return nil
}

// AfterApply sets the name and linter for the parent build command.
func (c buildConfigCmd) AfterApply(b *buildChild) error { // nolint:unparam
	b.name = c.Name
	b.linter = xpkg.NewConfigurationLinter()
	return nil
}

// buildProviderCmd builds a Provider.
type buildProviderCmd struct {
	Name string `optional:"" help:"Name of the package to be built. Uses name in crossplane.yaml if not specified. Does not correspond to package tag."`
}

// Run runs the Provider build cmd.
func (c *buildProviderCmd) Run() error {
	return nil
}

// AfterApply sets the name and linter for the parent build command.
func (c buildProviderCmd) AfterApply(b *buildChild) error { // nolint:unparam
	b.name = c.Name
	b.linter = xpkg.NewProviderLinter()
	return nil
}
