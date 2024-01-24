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

	"github.com/spf13/afero"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	metav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/xcrd"
)

const (
	defaultCacheDir = ".crossplane/cache"
	packageFileName = "package.yaml"
	baseLayerLabel  = "base"

	refFmt   = "%s@%s"
	imageFmt = "%s:%s"
)

// Manager defines a Manager for preparing Crossplane packages for validation
type Manager struct {
	fetcher ImageFetcher
	cache   Cache

	crds  []*apiextv1.CustomResourceDefinition
	deps  map[string]bool // One level dependency images
	confs map[string]bool // Configuration images
}

// NewManager returns a new Manager
func NewManager(cacheDir string, fs afero.Fs) *Manager {
	m := &Manager{}

	m.cache = &LocalCache{
		fs:       fs,
		cacheDir: cacheDir,
	}

	m.fetcher = &Fetcher{}

	m.crds = make([]*apiextv1.CustomResourceDefinition, 0)
	m.deps = make(map[string]bool)
	m.confs = make(map[string]bool)

	return m
}

// PrepExtensions converts the unstructured XRDs/CRDs to CRDs and extract package images to add as a dependency
func (m *Manager) PrepExtensions(extensions []*unstructured.Unstructured) error { //nolint:gocyclo // the function itself is not that complex, it just has different cases
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
			xrd := &v1.CompositeResourceDefinition{}
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

			if xrd.Spec.ClaimNames != nil {
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
				return errors.Wrapf(err, "cannot get package image")
			}

			m.deps[image] = true

		case schema.GroupKind{Group: "pkg.crossplane.io", Kind: "Configuration"}:
			paved := fieldpath.Pave(e.Object)
			image, err := paved.GetString("spec.package")
			if err != nil {
				return errors.Wrapf(err, "cannot get package image")
			}

			m.confs[image] = true

		default:
			continue
		}
	}

	return nil
}

// CacheAndLoad finds and caches dependencies and loads them as CRDs
func (m *Manager) CacheAndLoad(cleanCache bool) error {
	if cleanCache {
		if err := m.cache.Flush(); err != nil {
			return errors.Wrapf(err, "cannot flush cache directory")
		}
	}

	if err := m.cache.Init(); err != nil {
		return errors.Wrapf(err, "cannot initialize cache directory")
	}

	if err := m.addDependencies(); err != nil {
		return errors.Wrapf(err, "cannot add package dependencies")
	}

	if err := m.cacheDependencies(); err != nil {
		return errors.Wrapf(err, "cannot cache package dependencies")
	}

	schemas, err := m.cache.Load()
	if err != nil {
		return errors.Wrapf(err, "cannot load cache")
	}

	return m.PrepExtensions(schemas)
}

func (m *Manager) addDependencies() error {
	for image := range m.confs {
		m.deps[image] = true // we need to download the configuration package for the XRDs

		layer, err := m.fetcher.FetchBaseLayer(image)
		if err != nil {
			return errors.Wrapf(err, "cannot download package %s", image)
		}

		_, meta, err := extractPackageContent(*layer)
		if err != nil {
			return errors.Wrapf(err, "cannot extract package file and meta")
		}

		cfg := &metav1.Configuration{}
		if err := yaml.Unmarshal(meta, cfg); err != nil {
			return errors.Wrapf(err, "cannot unmarshal configuration YAML")
		}

		deps := cfg.Spec.MetaSpec.DependsOn
		for _, dep := range deps {
			image := ""
			if dep.Configuration != nil {
				image = *dep.Configuration
			} else if dep.Provider != nil {
				image = *dep.Provider
			}
			image = fmt.Sprintf(imageFmt, image, dep.Version)
			m.deps[image] = true
		}
	}

	return nil
}

func (m *Manager) cacheDependencies() error {
	if err := m.cache.Init(); err != nil {
		return errors.Wrapf(err, "cannot initialize  cache directory")
	}

	for image := range m.deps {
		path, err := m.cache.Exists(image) // returns the path if the image is not cached
		if err != nil {
			return errors.Wrapf(err, "cannot check if cache exists for %s", image)
		}

		if path == "" {
			continue
		}

		fmt.Printf("package schemas does not exist, downloading: %s\n", image)

		layer, err := m.fetcher.FetchBaseLayer(image)
		if err != nil {
			return errors.Wrapf(err, "cannot download package %s", image)
		}

		schemas, _, err := extractPackageContent(*layer)
		if err != nil {
			return errors.Wrapf(err, "cannot extract package file and meta")
		}

		if err := m.cache.Store(schemas, path); err != nil {
			return errors.Wrapf(err, "cannot store base layer")
		}
	}

	return nil
}
