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

package parser

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

const (
	errNilReadCloser = "cannot read from nil io.ReadCloser"
)

// Package includes fields that all meaningful parsers should support.
type Package struct {
	meta    []runtime.Object
	objects []runtime.Object
}

// NewPackage returns a new Package with maps initialized.
func NewPackage() *Package {
	return &Package{}
}

// GetMeta gets the package provider manifest.
func (p *Package) GetMeta() []runtime.Object {
	return p.meta
}

// GetObjects gets the package provider manifest.
func (p *Package) GetObjects() []runtime.Object {
	return p.objects
}

// Parser is a package parser.
type Parser interface {
	Parse(context.Context, io.ReadCloser) (*Package, error)
}

// DefaultParser is the default Parser implementation.
type DefaultParser struct {
	metaScheme *runtime.Scheme
	objScheme  *runtime.Scheme
}

// New returns a new DefaultParser.
func New(meta, obj *runtime.Scheme) *DefaultParser {
	return &DefaultParser{
		metaScheme: meta,
		objScheme:  obj,
	}
}

// Parse is the underlying parsing logic for parsing Crossplane packages.
func (p *DefaultParser) Parse(ctx context.Context, reader io.ReadCloser) (*Package, error) { //nolint:gocyclo
	pkg := NewPackage()
	if reader == nil {
		return pkg, nil
	}
	defer func() { _ = reader.Close() }()
	d := yaml.NewYAMLToJSONDecoder(reader)
	for {
		u := &unstructured.Unstructured{}
		err := d.Decode(u)
		if err == io.EOF {
			break
		}
		if err != nil {
			return pkg, err
		}
		if p.metaScheme.Recognizes(u.GroupVersionKind()) {
			pkg.meta = append(pkg.meta, u)
		}
		if p.objScheme.Recognizes(u.GroupVersionKind()) {
			pkg.objects = append(pkg.objects, u)
		}
	}
	return pkg, nil
}

// BackendOption modifies the parser backend. Backends may accept options at
// creation time, but must accept them at initialization.
type BackendOption func(Backend)

// Backend provides a source for a parser.
type Backend interface {
	Init(context.Context, ...BackendOption) (io.ReadCloser, error)
}

// PodLogBackend is a package parser that uses Kubernetes pod logs as source.
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

// NopBackend is a package backend with empty source.
type NopBackend struct{}

// NewNopBackend returns a new NopBackend.
func NewNopBackend(...BackendOption) *NopBackend {
	return &NopBackend{}
}

// Init initializes a NopBackend.
func (p *NopBackend) Init(ctx context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	return nil, nil
}

// FsBackend is a parser backend that uses a filestystem as source.
type FsBackend struct {
	fs    afero.Fs
	dir   string
	skips []FilterFn
}

// NewFsBackend returns a FsBackend.
func NewFsBackend(fs afero.Fs, bo ...BackendOption) *FsBackend {
	f := &FsBackend{
		fs: fs,
	}
	for _, o := range bo {
		o(f)
	}
	return f
}

// Init initializes a FsBackend.
func (p *FsBackend) Init(ctx context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(p)
	}
	return NewFsReadCloser(p.fs, p.dir, p.skips...)
}

// FsDir sets the directory of a FsBackend.
func FsDir(dir string) BackendOption {
	return func(p Backend) {
		f, ok := p.(*FsBackend)
		if !ok {
			return
		}
		f.dir = dir
	}
}

// FsFilters adds FilterFns to a FsBackend.
func FsFilters(skips ...FilterFn) BackendOption {
	return func(p Backend) {
		f, ok := p.(*FsBackend)
		if !ok {
			return
		}
		f.skips = skips
	}
}

// EchoBackend is a backend parser that uses string input as source.
type EchoBackend struct {
	echo string
}

// NewEchoBackend returns a new EchoBackend.
func NewEchoBackend(echo string) Backend {
	return &EchoBackend{
		echo: echo,
	}
}

// Init initializes a EchoBackend.
func (p *EchoBackend) Init(ctx context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	for _, o := range bo {
		o(p)
	}
	return ioutil.NopCloser(strings.NewReader(p.echo)), nil
}
