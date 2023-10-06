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

	xpkgv1 "github.com/crossplane/crossplane/internal/xpkg"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/examples"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/yaml"
)

const (
	errGetNameFromMeta                = "failed to get package name from crossplane.yaml"
	errBuildPackage                   = "failed to build package"
	errImageDigest                    = "failed to get package digest"
	errCreatePackage                  = "failed to create package file"
	errParseControllerImage           = "failed to parse controller image"
	errPullControllerImage            = "failed to pull controller image"
	errLoadControllerTar              = "failed to load controller tar"
	errGettingControllerBaseImageOpts = "failed to get controller base image options"
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

	Output        string   `optional:"" short:"o" help:"Path for package output."`
	Controller    string   `help:"Controller image used as base for package."`
	ControllerTar string   `help:"Path to tar file, an alternative to Controller." xor:"controller" type:"existingfile"`
	PackageRoot   string   `short:"f" help:"Path to package directory." default:"."`
	ExamplesRoot  string   `short:"e" help:"Path to package examples directory." default:"./examples"`
	Ignore        []string `help:"Paths, specified relative to --package-root, to exclude from the package."`
}

func (c *buildCmd) Help() string {
	return `
The build command creates a xpkg compatible OCI image for a Crossplane package
from the local file system. It packages the found YAML files containing Kubernetes-like
object manifests into the meta data layer of the OCI image. The package manager
will use this information to install the package into a Crossplane instance.

Only configuration, provider and function packages are supported at this time. 

Example claims can be specified in the examples directory.

For more generic information, see the xpkg parent command help. Also see the
Crossplane documentation for more information on building packages:

  https://docs.crossplane.io/latest/concepts/packages/#building-a-package

Even more details can be found in the xpkg reference document.`
}

// GetControllerBaseImageOpts returns the controller base image options.
func (c *buildCmd) GetControllerBaseImageOpts() ([]xpkg.BuildOpt, error) {
	switch {
	case c.ControllerTar != "":
		img, err := tarball.ImageFromPath(filepath.Clean(c.ControllerTar), nil)
		if err != nil {
			return nil, errors.Wrap(err, errLoadControllerTar)
		}
		return []xpkg.BuildOpt{xpkg.WithController(img)}, nil
	case c.Controller != "":
		ref, err := name.ParseReference(c.Controller)
		if err != nil {
			return nil, errors.Wrap(err, errParseControllerImage)
		}
		img, err := daemon.Image(ref, daemon.WithContext(context.Background()))
		if err != nil {
			return nil, errors.Wrap(err, errPullControllerImage)
		}
		return []xpkg.BuildOpt{xpkg.WithController(img)}, nil
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
		pkgName := xpkgv1.FriendlyID(pkgMeta.GetName(), hash.Hex)
		output = xpkgv1.BuildPath(c.root, pkgName, xpkgv1.XpkgExtension)
	}
	return output, nil
}

// Run executes the build command.
func (c *buildCmd) Run(logger logging.Logger) error {
	var buildOpts []xpkg.BuildOpt
	controllerBuildOpts, err := c.GetControllerBaseImageOpts()
	if err != nil {
		return errors.Wrap(err, errGettingControllerBaseImageOpts)
	}
	buildOpts = append(buildOpts, controllerBuildOpts...)

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
