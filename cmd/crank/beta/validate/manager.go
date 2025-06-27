/*
Copyright 2024 The Crossplane Authors.

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

package validate

import (
	"fmt"
	"io"

	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	packageFileName = "package.yaml"
	baseLayerLabel  = "base"

	refFmt   = "%s@%s"
	imageFmt = "%s:%s"
)

// Manager defines a Manager for preparing Crossplane packages for validation.
type Manager struct {
	fetcher ImageFetcher
	cache   Cache
	writer  io.Writer

	crds  []*extv1.CustomResourceDefinition
	deps  map[string]bool                  // Dependency images
	confs map[string]*metav1.Configuration // Configuration images
}

// Option defines an option for the Manager.
type Option func(*Manager)

// WithCrossplaneImage sets the Crossplane image to use for fetching CRDs.
func WithCrossplaneImage(image string) Option {
	return func(m *Manager) {
		if image == "" {
			return
		}

		m.deps[image] = true
	}
}

// NewManager returns a new Manager.
func NewManager(cacheDir string, fs afero.Fs, w io.Writer, opts ...Option) *Manager {
	m := &Manager{}

	m.cache = &LocalCache{
		fs:       fs,
		cacheDir: cacheDir,
	}

	m.fetcher = &Fetcher{}
	m.writer = w
	m.crds = make([]*extv1.CustomResourceDefinition, 0)
	m.deps = make(map[string]bool)
	m.confs = make(map[string]*metav1.Configuration)

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// PrepExtensions converts the unstructured XRDs/CRDs to CRDs and extract package images to add as a dependency.
func (m *Manager) PrepExtensions(extensions []*unstructured.Unstructured) error { //nolint:gocognit // the function itself is not that complex, it just has different cases
	for _, e := range extensions {
		switch e.GroupVersionKind().GroupKind() {
		case schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:
			crd := &extv1.CustomResourceDefinition{}

			bytes, err := e.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "cannot marshal CRD to JSON")
			}

			if err := yaml.Unmarshal(bytes, crd); err != nil {
				return errors.Wrap(err, "cannot unmarshal CRD YAML")
			}

			m.crds = append(m.crds, crd)

		case schema.GroupKind{Group: "apiextensions.crossplane.io", Kind: "CompositeResourceDefinition"}:
			xrd := &v2.CompositeResourceDefinition{}

			bytes, err := e.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "cannot marshal XRD to JSON")
			}

			if err := yaml.Unmarshal(bytes, xrd); err != nil {
				return errors.Wrap(err, "cannot unmarshal XRD YAML")
			}

			crd, err := xcrd.ForCompositeResource(xrd)
			if err != nil {
				return errors.Wrapf(err, "cannot derive composite CRD from XRD %q", xrd.GetName())
			}

			m.crds = append(m.crds, crd)

			if xrd.Spec.ClaimNames != nil { //nolint:staticcheck // we are still supporting v1 XRD
				claimCrd, err := xcrd.ForCompositeResourceClaim(xrd)
				if err != nil {
					return errors.Wrapf(err, "cannot derive claim CRD from XRD %q", xrd.GetName())
				}

				m.crds = append(m.crds, claimCrd)
			}

		case schema.GroupKind{Group: "pkg.crossplane.io", Kind: "Provider"}:
			paved := fieldpath.Pave(e.Object)

			image, err := paved.GetString("spec.package")
			if err != nil {
				return errors.Wrapf(err, "cannot get provider package image")
			}

			m.deps[image] = true

		case schema.GroupKind{Group: "pkg.crossplane.io", Kind: "Function"}:
			paved := fieldpath.Pave(e.Object)

			image, err := paved.GetString("spec.package")
			if err != nil {
				return errors.Wrapf(err, "cannot get function package image")
			}

			m.deps[image] = true

		case schema.GroupKind{Group: "pkg.crossplane.io", Kind: "Configuration"}:
			paved := fieldpath.Pave(e.Object)

			image, err := paved.GetString("spec.package")
			if err != nil {
				return errors.Wrapf(err, "cannot get package image")
			}

			m.confs[image] = nil

		case schema.GroupKind{Group: "meta.pkg.crossplane.io", Kind: "Configuration"}:
			meta, err := e.MarshalJSON()
			if err != nil {
				return errors.Wrap(err, "cannot marshal configuration to JSON")
			}

			cfg := &metav1.Configuration{}
			if err := yaml.Unmarshal(meta, cfg); err != nil {
				return errors.Wrapf(err, "cannot unmarshal configuration YAML")
			}

			m.confs[cfg.Name] = cfg

		default:
			continue
		}
	}

	return nil
}

// CacheAndLoad finds and caches dependencies and loads them as CRDs.
func (m *Manager) CacheAndLoad(cleanCache bool) error {
	if cleanCache {
		if err := m.cache.Flush(); err != nil {
			return errors.Wrapf(err, "cannot flush cache directory")
		}
	}

	if err := m.cache.Init(); err != nil {
		return errors.Wrapf(err, "cannot initialize cache directory")
	}

	if err := m.addDependencies(m.confs); err != nil {
		return errors.Wrapf(err, "cannot add package dependencies")
	}

	if err := m.cacheDependencies(); err != nil {
		return errors.Wrapf(err, "cannot cache package dependencies")
	}

	schemas, err := m.loadDependencies()
	if err != nil {
		return errors.Wrapf(err, "cannot load cache")
	}

	return m.PrepExtensions(schemas)
}

func (m *Manager) addDependencies(confs map[string]*metav1.Configuration) error { //nolint:gocognit // no extra func
	if len(confs) == 0 {
		return nil
	}

	deepConfs := make(map[string]*metav1.Configuration)

	for image := range confs {
		cfg := m.confs[image]

		if cfg == nil {
			m.deps[image] = true // we need to download the configuration package for the XRDs

			layer, err := m.fetcher.FetchBaseLayer(image)
			if err != nil {
				return errors.Wrapf(err, "cannot download package %s", image)
			}

			_, meta, err := extractPackageContent(*layer)
			if err != nil {
				return errors.Wrapf(err, "cannot extract package file and meta")
			}

			if err := yaml.Unmarshal(meta, &cfg); err != nil {
				return errors.Wrapf(err, "cannot unmarshal configuration YAML")
			}

			m.confs[image] = cfg // update the configuration
		}

		deps := cfg.Spec.DependsOn
		for _, dep := range deps {
			image := ""

			switch {
			case dep.Package != nil:
				image = *dep.Package
			case dep.Configuration != nil:
				image = *dep.Configuration
			case dep.Provider != nil:
				image = *dep.Provider
			case dep.Function != nil:
				image = *dep.Function
			}

			if len(image) > 0 {
				if _, err := regv1.NewHash(dep.Version); err == nil {
					// digest
					image = fmt.Sprintf(refFmt, image, dep.Version)
				} else {
					// tag
					image = fmt.Sprintf(imageFmt, image, dep.Version)
				}

				m.deps[image] = true

				if _, ok := m.confs[image]; !ok && dep.Configuration != nil {
					deepConfs[image] = nil
					m.confs[image] = nil
				}
			}
		}
	}

	return m.addDependencies(deepConfs)
}

func (m *Manager) cacheDependencies() error {
	if err := m.cache.Init(); err != nil {
		return errors.Wrapf(err, "cannot initialize cache directory")
	}

	for image := range m.deps {
		path, err := m.cache.Exists(image) // returns the path if the image is not cached
		if err != nil {
			return errors.Wrapf(err, "cannot check if cache exists for %s", image)
		}

		if path == "" {
			continue
		}

		if _, err := fmt.Fprintln(m.writer, "schemas does not exist, downloading: ", image); err != nil {
			return errors.Wrapf(err, errWriteOutput)
		}

		var schemas [][]byte
		// handling for packages
		layer, err := m.fetcher.FetchBaseLayer(image)
		switch {
		case IsErrBaseLayerNotFound(err):
			// We fall back to fetching the image if the base layer is not found
			layers, err := m.fetcher.FetchImage(image)
			if err != nil {
				return errors.Wrapf(err, "cannot extract crds")
			}

			schemas, err = extractPackageCRDs(layers)
			if err != nil {
				return errors.Wrapf(err, "cannot find crds")
			}
		case err != nil:
			return errors.Wrapf(err, "cannot download package %s", image)
		default:
			schemas, _, err = extractPackageContent(*layer)
			if err != nil {
				return errors.Wrapf(err, "cannot extract package file and meta")
			}
		}

		if err := m.cache.Store(schemas, path); err != nil {
			return errors.Wrapf(err, "cannot store base layer")
		}
	}

	return nil
}

func (m *Manager) loadDependencies() ([]*unstructured.Unstructured, error) {
	schemas := make([]*unstructured.Unstructured, 0)

	for dep := range m.deps {
		cachedSchema, err := m.cache.Load(dep)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot load cache for %s", dep)
		}

		schemas = append(schemas, cachedSchema...)
	}

	return schemas, nil
}
