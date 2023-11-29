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
	"context"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/parser/examples"
	"github.com/crossplane/crossplane/internal/xpkg/parser/yaml"
)

const (
	errGetNameFromMeta         = "failed to get package name from crossplane.yaml"
	errBuildPackage            = "failed to build package"
	errImageDigest             = "failed to get package digest"
	errCreatePackage           = "failed to create package file"
	errParseRuntimeImageRef    = "failed to parse runtime image reference"
	errPullRuntimeImage        = "failed to pull runtime image"
	errLoadRuntimeTarball      = "failed to load runtime tarball"
	errGetRuntimeBaseImageOpts = "failed to get runtime base image options"
)

// AfterApply constructs and binds context to any subcommands
// that have Run() methods that receive it.
func (c *buildCmd) AfterApply() error {
	c.fs = afero.NewOsFs()

	root, err := filepath.Abs(c.PackageRoot)
	if err != nil {
		return err
	}
	c.root = root

	ex, err := filepath.Abs(c.ExamplesRoot)
	if err != nil {
		return err
	}

	pp, err := yaml.New()
	if err != nil {
		return err
	}

	c.builder = xpkg.New(
		parser.NewFsBackend(
			c.fs,
			parser.FsDir(root),
			parser.FsFilters(
				append(
					buildFilters(root, c.Ignore),
					xpkg.SkipContains(c.ExamplesRoot))...),
		),
		parser.NewFsBackend(
			c.fs,
			parser.FsDir(ex),
			parser.FsFilters(
				buildFilters(ex, c.Ignore)...),
		),
		pp,
		examples.New(),
	)

	return nil
}

// buildCmd builds a crossplane package.
type buildCmd struct {
	// Flags. Keep sorted alphabetically.
	EmbedRuntimeImage        string   `placeholder:"NAME" help:"An OCI image to embed in the package as its runtime." xor:"runtime-image"`
	EmbedRuntimeImageTarball string   `placeholder:"PATH" type:"existingfile" help:"An OCI image tarball to embed in the package as its runtime." xor:"runtime-image"`
	ExamplesRoot             string   `short:"e" type:"path" help:"A directory of example YAML files to include in the package." default:"./examples"`
	Ignore                   []string `placeholder:"PATH" help:"Comma-separated file paths, specified relative to --package-root, to exclude from the package. Wildcards are supported. Directories cannot be excluded."`
	PackageFile              string   `short:"o" type:"path" placeholder:"PATH" help:"The file to write the package to. Defaults to a generated filename in --package-root."`
	PackageRoot              string   `short:"f" type:"existingdir" help:"The directory that contains the package's crossplane.yaml file." default:"."`

	// Internal state. These aren't part of the user-exposed CLI structure.
	fs      afero.Fs
	builder *xpkg.Builder
	root    string
}

func (c *buildCmd) Help() string {
	return `
This command builds a package file from a local directory of files.

Examples:

  # Build a package from the files in the 'package' directory.
  crossplane xpkg build --package-root=package/

  # Build a package that embeds a Provider's controller OCI image built with
  # 'docker build' so that the package can also be used to run the provider.
  # Provider and Function packages support embedding runtime images.
  crossplane xpkg build --embed-runtime-image=cc873e13cdc1
`
}

// GetRuntimeBaseImageOpts returns the controller base image options.
func (c *buildCmd) GetRuntimeBaseImageOpts() ([]xpkg.BuildOpt, error) {
	switch {
	case c.EmbedRuntimeImageTarball != "":
		img, err := tarball.ImageFromPath(filepath.Clean(c.EmbedRuntimeImageTarball), nil)
		if err != nil {
			return nil, errors.Wrap(err, errLoadRuntimeTarball)
		}
		return []xpkg.BuildOpt{xpkg.WithBase(img)}, nil
	case c.EmbedRuntimeImage != "":
		// We intentionally don't override the default registry here. Doing so
		// leads to unintuitive behavior, in that you can't tag your runtime
		// image as some/image:latest then pass that same tag to xpkg build.
		// Instead you'd need to pass index.docker.io/some/image:latest.
		ref, err := name.ParseReference(c.EmbedRuntimeImage)
		if err != nil {
			return nil, errors.Wrap(err, errParseRuntimeImageRef)
		}
		img, err := daemon.Image(ref, daemon.WithContext(context.Background()))
		if err != nil {
			return nil, errors.Wrap(err, errPullRuntimeImage)
		}
		return []xpkg.BuildOpt{xpkg.WithBase(img)}, nil
	}
	return nil, nil

}

// GetOutputFileName prepares output file name.
func (c *buildCmd) GetOutputFileName(meta runtime.Object, hash v1.Hash) (string, error) {
	output := filepath.Clean(c.PackageFile)
	if c.PackageFile == "" {
		pkgMeta, ok := meta.(metav1.Object)
		if !ok {
			return "", errors.New(errGetNameFromMeta)
		}
		pkgName := xpkg.FriendlyID(pkgMeta.GetName(), hash.Hex)
		output = xpkg.BuildPath(c.root, pkgName, xpkg.XpkgExtension)
	}
	return output, nil
}

// Run executes the build command.
func (c *buildCmd) Run(logger logging.Logger) error {
	var buildOpts []xpkg.BuildOpt
	rtBuildOpts, err := c.GetRuntimeBaseImageOpts()
	if err != nil {
		return errors.Wrap(err, errGetRuntimeBaseImageOpts)
	}
	buildOpts = append(buildOpts, rtBuildOpts...)

	img, meta, err := c.builder.Build(context.Background(), buildOpts...)
	if err != nil {
		return errors.Wrap(err, errBuildPackage)
	}

	hash, err := img.Digest()
	if err != nil {
		return errors.Wrap(err, errImageDigest)
	}

	output, err := c.GetOutputFileName(meta, hash)
	if err != nil {
		return err
	}

	f, err := c.fs.Create(output)
	if err != nil {
		return errors.Wrap(err, errCreatePackage)
	}

	defer func() { _ = f.Close() }()
	if err := tarball.Write(nil, img, f); err != nil {
		return err
	}
	logger.Info("xpkg saved", "output", output)
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
