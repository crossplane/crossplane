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

// Package parser implements a parser for Crossplane packages.
package parser

import (
	"bufio"
	"context"
	"io"
	"strings"
	"unicode"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Lintable defines the common API for lintable packages.
type Lintable interface {
	// GetMeta returns metadata objects of the lintable package, such as
	// Provider, Configuration or Function.
	GetMeta() []runtime.Object
	// GetObjects returns objects of the lintable package.
	GetObjects() []runtime.Object
}

// AnnotatedReadCloser is a wrapper around io.ReadCloser that allows
// implementations to supply additional information about data that is read.
type AnnotatedReadCloser interface {
	io.ReadCloser
	Annotate() any
}

// ObjectCreaterTyper know how to create and determine the type of objects.
type ObjectCreaterTyper interface {
	runtime.ObjectCreater
	runtime.ObjectTyper
}

// Package is the set of metadata and objects in a package.
type Package struct {
	meta    []runtime.Object
	objects []runtime.Object
}

// NewPackage creates a new Package.
func NewPackage() *Package {
	return &Package{}
}

// GetMeta gets metadata from the package.
func (p *Package) GetMeta() []runtime.Object {
	return p.meta
}

// GetObjects gets objects from the package.
func (p *Package) GetObjects() []runtime.Object {
	return p.objects
}

// Parser is a package parser.
type Parser interface {
	Parse(ctx context.Context, rc io.ReadCloser) (*Package, error)
}

// PackageParser is a Parser implementation for parsing packages.
type PackageParser struct {
	metaScheme ObjectCreaterTyper
	objScheme  ObjectCreaterTyper
}

// New returns a new PackageParser.
func New(meta, obj ObjectCreaterTyper) *PackageParser {
	return &PackageParser{
		metaScheme: meta,
		objScheme:  obj,
	}
}

// Parse is the underlying logic for parsing packages. It first attempts to
// decode objects recognized by the meta scheme, then attempts to decode objects
// recognized by the object scheme. Objects not recognized by either scheme
// return an error rather than being skipped.
func (p *PackageParser) Parse(_ context.Context, reader io.ReadCloser) (*Package, error) {
	pkg := NewPackage()
	if reader == nil {
		return pkg, nil
	}

	defer func() { _ = reader.Close() }()

	yr := yaml.NewYAMLReader(bufio.NewReader(reader))
	dm := json.NewSerializerWithOptions(json.DefaultMetaFactory, p.metaScheme, p.metaScheme, json.SerializerOptions{Yaml: true})
	do := json.NewSerializerWithOptions(json.DefaultMetaFactory, p.objScheme, p.objScheme, json.SerializerOptions{Yaml: true})

	for {
		content, err := yr.Read()
		if err != nil && !errors.Is(err, io.EOF) {
			return pkg, err
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if isEmptyYAML(content) {
			continue
		}

		m, _, err := dm.Decode(content, nil, nil)
		if err != nil {
			// NOTE(hasheddan): we only try to decode with object scheme if the
			// error is due the object not being registered in the meta scheme.
			if !runtime.IsNotRegisteredError(err) {
				return pkg, annotateErr(err, reader)
			}

			o, _, err := do.Decode(content, nil, nil)
			if err != nil {
				return pkg, annotateErr(err, reader)
			}

			pkg.objects = append(pkg.objects, o)

			continue
		}

		pkg.meta = append(pkg.meta, m)
	}

	return pkg, nil
}

// isEmptyYAML checks whether the provided YAML can be considered empty. This
// is useful for filtering out empty YAML documents that would otherwise
// cause issues when decoded.
func isEmptyYAML(y []byte) bool {
	for line := range strings.SplitSeq(string(y), "\n") {
		trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
		// We don't want to return an empty document with only separators that
		// have nothing in-between.
		if trimmed != "" && trimmed != "---" && trimmed != "..." && !strings.HasPrefix(trimmed, "#") {
			return false
		}
	}

	return true
}

// annotateErr annotates an error if the reader is an AnnotatedReadCloser.
func annotateErr(err error, reader io.ReadCloser) error {
	if anno, ok := reader.(AnnotatedReadCloser); ok {
		return errors.Wrapf(err, "%+v", anno.Annotate())
	}

	return err
}

// BackendOption modifies the parser backend. Backends may accept options at
// creation time, but must accept them at initialization.
type BackendOption func(Backend)

// Backend provides a source for a parser.
type Backend interface {
	Init(ctx context.Context, o ...BackendOption) (io.ReadCloser, error)
}

// PodLogBackend is a parser backend that uses Kubernetes pod logs as source.
type PodLogBackend struct {
	client    kubernetes.Interface
	name      string
	namespace string
}

// NewPodLogBackend returns a new PodLogBackend.
func NewPodLogBackend(bo ...BackendOption) *PodLogBackend {
	p := &PodLogBackend{}
	for _, o := range bo {
		o(p)
	}

	return p
}

// Init initializes a PodLogBackend.
func (p *PodLogBackend) Init(ctx context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(p)
	}

	logs := p.client.CoreV1().Pods(p.namespace).GetLogs(p.name, &corev1.PodLogOptions{})

	reader, err := logs.Stream(ctx)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// PodName sets the pod name of a PodLogBackend.
func PodName(name string) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}

		pl.name = name
	}
}

// PodNamespace sets the pod namespace of a PodLogBackend.
func PodNamespace(namespace string) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}

		pl.namespace = namespace
	}
}

// PodClient sets the pod client of a PodLogBackend.
func PodClient(client kubernetes.Interface) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}

		pl.client = client
	}
}

// NopBackend is a parser backend with empty source.
type NopBackend struct{}

// NewNopBackend returns a new NopBackend.
func NewNopBackend(...BackendOption) *NopBackend {
	return &NopBackend{}
}

// Init initializes a NopBackend.
func (p *NopBackend) Init(_ context.Context, _ ...BackendOption) (io.ReadCloser, error) {
	return nil, nil
}

// FsBackend is a parser backend that uses a filestystem as source.
type FsBackend struct {
	fs    afero.Fs
	dir   string
	skips []FilterFn
}

// NewFsBackend returns an FsBackend.
func NewFsBackend(fs afero.Fs, bo ...BackendOption) *FsBackend {
	f := &FsBackend{
		fs: fs,
	}
	for _, o := range bo {
		o(f)
	}

	return f
}

// Init initializes an FsBackend.
func (p *FsBackend) Init(_ context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(p)
	}

	return NewFsReadCloser(p.fs, p.dir, p.skips...)
}

// FsDir sets the directory of an FsBackend.
func FsDir(dir string) BackendOption {
	return func(p Backend) {
		f, ok := p.(*FsBackend)
		if !ok {
			return
		}

		f.dir = dir
	}
}

// FsFilters adds FilterFns to an FsBackend.
func FsFilters(skips ...FilterFn) BackendOption {
	return func(p Backend) {
		f, ok := p.(*FsBackend)
		if !ok {
			return
		}

		f.skips = skips
	}
}

// EchoBackend is a parser backend that uses string input as source.
type EchoBackend struct {
	echo string
}

// NewEchoBackend returns a new EchoBackend.
func NewEchoBackend(echo string) Backend {
	return &EchoBackend{
		echo: echo,
	}
}

// Init initializes an EchoBackend.
func (p *EchoBackend) Init(_ context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(p)
	}

	return io.NopCloser(strings.NewReader(p.echo)), nil
}
