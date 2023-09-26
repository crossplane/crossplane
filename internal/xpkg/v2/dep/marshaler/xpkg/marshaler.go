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

// Package xpkg contains utilities for marshalling packages.
package xpkg

import (
	"archive/tar"
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/spf13/afero"
	"github.com/spf13/afero/tarfs"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"

	xpmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/linter"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/ndjson"
	"github.com/crossplane/crossplane/internal/xpkg/v2/parser/yaml"
	"github.com/crossplane/crossplane/internal/xpkg/v2/scheme"
)

const (
	errFailedToParsePkgYaml         = "failed to parse package yaml"
	errLintPackage                  = "failed to lint package"
	errOpenPackageStream            = "failed to open package stream file"
	errFailedToConvertMetaToPackage = "failed to convert meta to package"
	errInvalidPath                  = "invalid path provided for package lookup"
	errNotExactlyOneMeta            = "not exactly one package meta type"
)

// Marshaler represents a xpkg Marshaler
type Marshaler struct {
	yp parser.Parser
	jp JSONPackageParser
}

// NewMarshaler returns a new Marshaler
func NewMarshaler(opts ...MarshalerOption) (*Marshaler, error) {
	r := &Marshaler{}
	yp, err := yaml.New()
	if err != nil {
		return nil, err
	}

	jp, err := ndjson.New()
	if err != nil {
		return nil, err
	}

	r.yp = yp
	r.jp = jp

	for _, o := range opts {
		o(r)
	}

	return r, nil
}

// MarshalerOption modifies the xpkg Marshaler
type MarshalerOption func(*Marshaler)

// WithYamlParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithYamlParser(p parser.Parser) MarshalerOption {
	return func(r *Marshaler) {
		r.yp = p
	}
}

// WithJSONParser modifies the Marshaler by setting the supplied PackageParser as
// the Resolver's parser.
func WithJSONParser(p JSONPackageParser) MarshalerOption {
	return func(r *Marshaler) {
		r.jp = p
	}
}

// FromImage takes a xpkg.Image and returns a ParsedPackage for consumption by
// upstream callers.
func (r *Marshaler) FromImage(i xpkg.Image) (*ParsedPackage, error) {
	reader := mutate.Extract(i.Image)
	fs := tarfs.New(tar.NewReader(reader))
	pkgYaml, err := fs.Open(xpkg.StreamFile)
	if err != nil {
		return nil, errors.Wrap(err, errOpenPackageStream)
	}

	pkg, err := r.parseYaml(pkgYaml)
	if err != nil {
		return nil, err
	}

	pkg = applyImageMeta(i.Meta, pkg)

	return finalizePkg(pkg)
}

// FromDir takes an afero.Fs and a path to a directory and returns a
// ParsedPackage based on the directories contents for consumption by upstream
// callers.
func (r *Marshaler) FromDir(fs afero.Fs, path string) (*ParsedPackage, error) {
	parts := strings.Split(path, "@")
	if len(parts) != 2 {
		return nil, errors.New(errInvalidPath)
	}

	pkgJSON, err := fs.Open(filepath.Join(path, xpkg.JSONStreamFile))
	if err != nil {
		return nil, err
	}

	pkg, err := r.parseNDJSON(pkgJSON)
	if err != nil {
		return nil, err
	}

	return finalizePkg(pkg)
}

// parseYaml parses the
func (r *Marshaler) parseYaml(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.yp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}
	return processPackage(pkg)
}

func processPackage(pkg linter.Package) (*ParsedPackage, error) {
	metas := pkg.GetMeta()
	if len(metas) != 1 {
		return nil, errors.New(errNotExactlyOneMeta)
	}

	meta := metas[0]
	var linter linter.Linter
	var pkgType v1beta1.PackageType
	if meta.GetObjectKind().GroupVersionKind().Kind == xpmetav1.ConfigurationKind {
		linter = xpkg.NewConfigurationLinter()
		pkgType = v1beta1.ConfigurationPackageType
	} else {
		linter = xpkg.NewProviderLinter()
		pkgType = v1beta1.ProviderPackageType
	}
	if err := linter.Lint(pkg); err != nil {
		return nil, errors.Wrap(err, errLintPackage)
	}

	return &ParsedPackage{
		MetaObj: meta,
		Objs:    pkg.GetObjects(),
		PType:   pkgType,
	}, nil
}

func (r *Marshaler) parseNDJSON(reader io.ReadCloser) (*ParsedPackage, error) {
	pkg, err := r.jp.Parse(context.Background(), reader)
	if err != nil {
		return nil, errors.Wrap(err, errFailedToParsePkgYaml)
	}
	p, err := processPackage(pkg)
	if err != nil {
		return nil, err
	}

	return applyImageMeta(pkg.GetImageMeta(), p), nil
}

func applyImageMeta(m xpkg.ImageMeta, pkg *ParsedPackage) *ParsedPackage {
	pkg.DepName = m.Repo
	pkg.Reg = m.Registry
	pkg.SHA = m.Digest
	pkg.Ver = m.Version

	return pkg
}

func finalizePkg(pkg *ParsedPackage) (*ParsedPackage, error) {
	deps, err := determineDeps(pkg.MetaObj)
	if err != nil {
		return nil, err
	}

	pkg.Deps = deps

	return pkg, nil
}

func determineDeps(o runtime.Object) ([]v1beta1.Dependency, error) {
	pkg, ok := scheme.TryConvertToPkg(o, &xpmetav1.Provider{}, &xpmetav1.Configuration{})
	if !ok {
		return nil, errors.New(errFailedToConvertMetaToPackage)
	}

	out := make([]v1beta1.Dependency, len(pkg.GetDependencies()))
	for i, d := range pkg.GetDependencies() {
		out[i] = convertToV1beta1(d)
	}

	return out, nil
}

func convertToV1beta1(in xpmetav1.Dependency) v1beta1.Dependency {
	betaD := v1beta1.Dependency{
		Constraints: in.Version,
	}
	if in.Provider != nil && in.Configuration == nil {
		betaD.Package = *in.Provider
		betaD.Type = v1beta1.ProviderPackageType
	}

	if in.Configuration != nil && in.Provider == nil {
		betaD.Package = *in.Configuration
		betaD.Type = v1beta1.ConfigurationPackageType
	}

	return betaD
}
