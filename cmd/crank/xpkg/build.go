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
	fs      afero.Fs
	builder *xpkg.Builder
	root    string

	Output       string   `optional:"" short:"o" help:"Path for package output."`
	PackageRoot  string   `short:"f" help:"Path to package directory." default:"."`
	ExamplesRoot string   `short:"e" help:"Path to package examples directory." default:"./examples"`
	Ignore       []string `help:"Paths, specified relative to --package-root, to exclude from the package."`

	EmbedRuntimeImage   string `help:"An OCI image reference to the package's runtime. The package will embed this image." placeholder:"\"example/runtime-image:latest\"" xor:"runtime-image"`
	EmbedRuntimeTarball string `help:"An OCI image tarball of the package's runtime. The package will embed this image." placeholder:"\"example-runtime-image.tar\"" type:"existingfile" xor:"runtime-image"`
}

func (c *buildCmd) Help() string {
	return `
The build command creates a Crossplane package from the local filesystem. A
package is an OCI image containing metadata and configuration manifests that can
be used to extend Crossplane with new functionality.

Crossplane supports configuration, provider and function packages. 

Packages can embed a runtime image. When a package embeds a runtime image
Crossplane can use the same OCI image to install and run the package. For
example a provider package can embed the provider's controller image (its
"runtime"). Crossplane will then use the package as the provider pod's image.

See the Crossplane documentation for more information on building packages:
https://docs.crossplane.io/latest/concepts/packages/#building-a-package
`
}

// GetRuntimeBaseImageOpts returns the controller base image options.
func (c *buildCmd) GetRuntimeBaseImageOpts() ([]xpkg.BuildOpt, error) {
	switch {
	case c.EmbedRuntimeTarball != "":
		img, err := tarball.ImageFromPath(filepath.Clean(c.EmbedRuntimeTarball), nil)
		if err != nil {
			return nil, errors.Wrap(err, errLoadRuntimeTarball)
		}
		return []xpkg.BuildOpt{xpkg.WithBase(img)}, nil
	case c.EmbedRuntimeImage != "":
		ref, err := name.ParseReference(c.EmbedRuntimeImage, name.WithDefaultRegistry(DefaultRegistry))
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
	output := filepath.Clean(c.Output)
	if c.Output == "" {
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
