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
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/internal/xpkg"
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

	PackageRoot string   `short:"f" help:"Path to package directory." default:"."`
	Ignore      []string `help:"Paths, specified relative to --package-root, to exclude from the package."`
}

// Run runs the build cmd.
func (c *buildCmd) Run(child *buildChild, logger logging.Logger) error {
	logger = logger.WithValues("Name", child.name)
	root, err := filepath.Abs(c.PackageRoot)
	if err != nil {
		return err
	}

	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		logger.Debug("Failed to build meta scheme for package parser", "error", err)
		return errors.New("cannot build meta scheme for package parser")
	}
	logger.Debug("Successfully built meta scheme for package parser")
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}
	logger.Debug("Successfully built Object scheme for package parser")
	img, err := xpkg.Build(context.Background(),
		parser.NewFsBackend(child.fs, parser.FsDir(root), parser.FsFilters(buildFilters(root, c.Ignore)...)),
		parser.New(metaScheme, objScheme),
		child.linter)
	if err != nil {
		logger.Debug(errBuildPackage, "error", err)
		return errors.Wrap(err, errBuildPackage)
	}
	logger.Debug("Successfully built package")

	hash, err := img.Digest()
	if err != nil {
		logger.Debug(errImageDigest, "error", err)
		return errors.Wrap(err, errImageDigest)
	}
	logger.Debug("Successfully found package digest")
	pkgName := child.name
	if pkgName == "" {
		metaPath := filepath.Join(root, xpkg.MetaFile)
		pkgName, err = xpkg.ParseNameFromMeta(child.fs, metaPath)
		if err != nil {
			logger.Debug(errGetNameFromMeta, "error", err)
			return errors.Wrap(err, errGetNameFromMeta)
		}
		pkgName = xpkg.FriendlyID(pkgName, hash.Hex)
	}

	f, err := child.fs.Create(xpkg.BuildPath(root, pkgName))
	if err != nil {
		logger.Debug(errCreatePackage, "error", err)
		return errors.Wrap(err, errCreatePackage)
	}
	logger.Debug("Successfully created package image file")
	defer func() { _ = f.Close() }()
	if err := tarball.Write(nil, img, f); err != nil {
		logger.Debug("Failed to write package image", "error", err)
		return err
	}
	return nil
}

// default build filters skip directories, empty files, and files without YAML
// extension in addition to any paths specified.
func buildFilters(root string, skips []string) []parser.FilterFn {
	defaultFns := []parser.FilterFn{
		parser.SkipDirs(),
		parser.SkipNotYAML(),
		parser.SkipEmpty(),
	}
	opts := make([]parser.FilterFn, len(skips)+len(defaultFns))
	copy(opts, defaultFns)
	for i, s := range skips {
		opts[i+len(defaultFns)] = parser.SkipPath(filepath.Join(root, s))
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

// AfterApply sets the name and linter for the parent build command.
func (c buildProviderCmd) AfterApply(b *buildChild) error { // nolint:unparam
	b.name = c.Name
	b.linter = xpkg.NewProviderLinter()
	return nil
}
