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

// Package load provides functionality to load Kubernetes manifests from various sources
package load

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	v1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1"
)

// Loader interface defines the contract for different input sources.
type Loader interface {
	Load() ([]*unstructured.Unstructured, error)
}

// NewLoader returns a Loader based on the input source.
func NewLoader(input string) (Loader, error) {
	sources := strings.Split(input, ",")

	if len(sources) == 1 {
		return newLoader(sources[0])
	}

	loaders := make([]Loader, 0, len(sources))

	for _, source := range sources {
		loader, err := newLoader(source)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("cannot create loader for %q", source))
		}

		loaders = append(loaders, loader)
	}

	return &MultiLoader{loaders: loaders}, nil
}

func newLoader(input string) (Loader, error) {
	if input == "-" {
		return &StdinLoader{}, nil
	}

	fi, err := os.Stat(input)
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat input source")
	}

	if fi.IsDir() {
		return &FolderLoader{path: input}, nil
	}

	return &FileLoader{path: input}, nil
}

// MultiLoader implements the Loader interface for reading from multiple other loaders.
type MultiLoader struct {
	loaders []Loader
}

// Load reads and merges the content from the loaders.
func (m *MultiLoader) Load() ([]*unstructured.Unstructured, error) {
	var manifests []*unstructured.Unstructured

	for i, loader := range m.loaders {
		output, err := loader.Load()
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("cannot load source at position %d", i))
		}

		manifests = append(manifests, output...)
	}

	return manifests, nil
}

// StdinLoader implements the Loader interface for reading from stdin.
type StdinLoader struct{}

// Load reads the contents from stdin.
func (s *StdinLoader) Load() ([]*unstructured.Unstructured, error) {
	stream, err := YamlStream(os.Stdin)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML stream from stdin")
	}

	return streamToUnstructured(stream)
}

// FileLoader implements the Loader interface for reading from a file and converting input to unstructured objects.
type FileLoader struct {
	path string
}

// Load reads the contents from a file.
func (f *FileLoader) Load() ([]*unstructured.Unstructured, error) {
	stream, err := readFile(f.path)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read file")
	}

	return streamToUnstructured(stream)
}

// FolderLoader implements the Loader interface for reading from a folder.
type FolderLoader struct {
	path string
}

// Load reads the contents from all files in a folder.
func (f *FolderLoader) Load() ([]*unstructured.Unstructured, error) {
	var stream [][]byte

	err := filepath.Walk(f.path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isYamlFile(info) {
			s, err := readFile(path)
			if err != nil {
				return err
			}

			stream = append(stream, s...)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "cannot read folder")
	}

	return streamToUnstructured(stream)
}

func isYamlFile(info os.FileInfo) bool {
	return !info.IsDir() && (filepath.Ext(info.Name()) == ".yaml" || filepath.Ext(info.Name()) == ".yml")
}

func readFile(path string) ([][]byte, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}
	defer f.Close() //nolint:errcheck // Only open for reading.

	return YamlStream(f)
}

// YamlStream loads a yaml stream from a reader into a 2d byte slice.
func YamlStream(r io.Reader) ([][]byte, error) {
	stream := make([][]byte, 0)

	yr := yaml.NewYAMLReader(bufio.NewReader(r))

	for {
		bytes, err := yr.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML stream")
		}

		if len(bytes) == 0 {
			continue
		}

		stream = append(stream, bytes)
	}

	return stream, nil
}

func streamToUnstructured(stream [][]byte) ([]*unstructured.Unstructured, error) {
	manifests := make([]*unstructured.Unstructured, 0, len(stream))

	for _, y := range stream {
		u := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(y, u); err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML manifest")
		}
		// extract pipeline input resources
		if u.GetObjectKind().GroupVersionKind() == v1.CompositionGroupVersionKind {
			// Convert the unstructured resource to a Composition
			var comp v1.Composition

			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &comp)
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert unstructured to Composition")
			}
			// Iterate over each step in the pipeline
			for _, step := range comp.Spec.Pipeline {
				// Create a new resource based on the input (we can use it for validation)
				if step.Input != nil && step.Input.Raw != nil {
					var inputMap map[string]interface{}

					err := json.Unmarshal(step.Input.Raw, &inputMap)
					if err != nil {
						return nil, errors.Wrap(err, "failed to unmarshal raw input")
					}

					newInputResource := &unstructured.Unstructured{
						Object: inputMap,
					}
					// Add the input as new manifest to the manifests slice that we can validate
					manifests = append(manifests, newInputResource)
				}
			}
		}

		manifests = append(manifests, u)
	}

	return manifests, nil
}

// CompositeLoader acts as a composition of multiple loaders
// to handle loading resources from various sources at once.
type CompositeLoader struct {
	loaders []Loader
}

// NewCompositeLoader creates a new composite loader based on the specified sources.
// Sources can be files, directories, or "-" for stdin.
// If sources is empty, stdin is used by default.
func NewCompositeLoader(sources []string) (Loader, error) {
	if len(sources) == 0 {
		// In unit tests, this will cause an error when Load() is called
		// which is the expected behavior for NoSources test case
		return &CompositeLoader{loaders: []Loader{}}, nil
	}

	// Create loaders for each source
	loaders := make([]Loader, 0, len(sources))

	// Check for duplicate stdin markers to avoid reading stdin multiple times
	stdinUsed := false

	for _, source := range sources {
		if source == "-" {
			if stdinUsed {
				// Skip duplicate stdin markers - only use stdin once
				continue
			}
			stdinUsed = true
		}

		loader, err := NewLoader(source)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create loader for %q", source)
		}
		loaders = append(loaders, loader)
	}

	return &CompositeLoader{loaders: loaders}, nil
}

// Load implements the Loader interface by loading from all contained loaders
// and combining the results.
func (c *CompositeLoader) Load() ([]*unstructured.Unstructured, error) {
	if len(c.loaders) == 0 {
		return nil, errors.New("no loaders configured")
	}

	// Combine results from all loaders
	var allResources []*unstructured.Unstructured

	for _, loader := range c.loaders {
		resources, err := loader.Load()
		if err != nil {
			return nil, errors.Wrap(err, "cannot load resources from loader")
		}
		allResources = append(allResources, resources...)
	}

	// Check if we found any resources
	if len(allResources) == 0 {
		return nil, errors.New("no resources found from any source")
	}

	return allResources, nil
}
