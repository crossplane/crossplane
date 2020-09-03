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
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

// Parser is a package parser.
type Parser interface {
	Parse(context.Context, ...BackendOption) (*Package, error)
}

// DefaultParser is a Parser implementation with pluggable backends.
type DefaultParser struct {
	backend Backend
}

// New returns a new parser.
func New(backend Backend) *DefaultParser {
	return &DefaultParser{
		backend: backend,
	}
}

// Parse is the underlying parsing logic for parsing Crossplane packages.
func (p *DefaultParser) Parse(ctx context.Context, opts ...BackendOption) (*Package, error) { //nolint:gocyclo
	pkg := NewPackage()
	reader, err := p.backend.Init(ctx, opts...)
	if err != nil || reader == nil {
		return pkg, err
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
		switch u.GroupVersionKind() {
		case pkgmetav1alpha1.ProviderGroupVersionKind:
			provider := &pkgmetav1alpha1.Provider{}
			if err := fromUnstructured(u.UnstructuredContent(), provider); err != nil {
				return pkg, err
			}
			pkg.provider = provider
		case pkgmetav1alpha1.ConfigurationGroupVersionKind:
			configuration := &pkgmetav1alpha1.Configuration{}
			if err := fromUnstructured(u.UnstructuredContent(), configuration); err != nil {
				return pkg, err
			}
			pkg.configuration = configuration
		case apiextensions.SchemeGroupVersion.WithKind("CustomResourceDefinition"):
			crd := &apiextensions.CustomResourceDefinition{}
			if err := fromUnstructured(u.UnstructuredContent(), crd); err != nil {
				return pkg, err
			}
			pkg.customResourceDefinitions[crd.GetName()] = crd
		case apiextensionsv1alpha1.CompositeResourceDefinitionGroupVersionKind:
			xrd := &apiextensionsv1alpha1.CompositeResourceDefinition{}
			if err := fromUnstructured(u.UnstructuredContent(), xrd); err != nil {
				return pkg, err
			}
			pkg.compositeResourceDefinitions[xrd.GetName()] = xrd
		case apiextensionsv1alpha1.CompositionGroupVersionKind:
			comp := &apiextensionsv1alpha1.Composition{}
			if err := fromUnstructured(u.UnstructuredContent(), comp); err != nil {
				return pkg, err
			}
			pkg.compositions[comp.GetName()] = comp
		default:
			// Unrecognized objects will be ignored.
			continue
		}
	}
	return pkg, nil
}

// BackendOption modifies the parser backend. Backends may accept options at
// creation time, but must accept them at initialization.
type BackendOption func(Backend)

// Backend provides a source for the parser.
type Backend interface {
	Init(context.Context, ...BackendOption) (io.ReadCloser, error)
}

// Package includes fields that all meaningful parsers should support.
type Package struct {
	provider                     *pkgmetav1alpha1.Provider
	configuration                *pkgmetav1alpha1.Configuration
	customResourceDefinitions    map[string]*apiextensions.CustomResourceDefinition
	compositeResourceDefinitions map[string]*apiextensionsv1alpha1.CompositeResourceDefinition
	compositions                 map[string]*apiextensionsv1alpha1.Composition
}

// NewPackage returns a new Package with maps initialized.
func NewPackage() *Package {
	return &Package{
		customResourceDefinitions:    map[string]*apiextensions.CustomResourceDefinition{},
		compositeResourceDefinitions: map[string]*apiextensionsv1alpha1.CompositeResourceDefinition{},
		compositions:                 map[string]*apiextensionsv1alpha1.Composition{},
	}
}

// GetProvider gets the package provider manifest.
func (p *Package) GetProvider() *pkgmetav1alpha1.Provider {
	return p.provider
}

// GetConfiguration gets the package configuration manifest.
func (p *Package) GetConfiguration() *pkgmetav1alpha1.Configuration {
	return p.configuration
}

// GetCustomResourceDefinitions gets a package's custom resource definitions.
func (p *Package) GetCustomResourceDefinitions() map[string]*apiextensions.CustomResourceDefinition {
	return p.customResourceDefinitions
}

// GetCompositeResourceDefinitions gets a package's composite resource definitions.
func (p *Package) GetCompositeResourceDefinitions() map[string]*apiextensionsv1alpha1.CompositeResourceDefinition {
	return p.compositeResourceDefinitions
}

// GetCompositions gets a package's compositions.
func (p *Package) GetCompositions() map[string]*apiextensionsv1alpha1.Composition {
	return p.compositions
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

// PodName sets the pod name of a PodLogParser.
func PodName(name string) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}
		pl.name = name
	}
}

// PodNamespace sets the pod namespace of a PodLogParser.
func PodNamespace(namespace string) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}
		pl.namespace = namespace
	}
}

// PodClient sets the pod client of a PodLogParser.
func PodClient(client kubernetes.Interface) BackendOption {
	return func(p Backend) {
		pl, ok := p.(*PodLogBackend)
		if !ok {
			return
		}
		pl.client = client
	}
}

// NopBackend is a package parser that parses nothing.
type NopBackend struct{}

// NewNopBackend returns a new NopBackend.
func NewNopBackend(...BackendOption) *NopBackend {
	return &NopBackend{}
}

// Init initializes a NopBackend.
func (p *NopBackend) Init(ctx context.Context, bo ...BackendOption) (io.ReadCloser, error) {
	return nil, nil
}

// FsBackend is a package parser that uses a filestystem as source.
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

// EchoBackend is a package parser that uses string input as source.
type EchoBackend struct {
	echo string
}

// NewEchoBackend returns a new EchoParser.
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
