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
	"bufio"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Loader interface defines the contract for different input sources
type Loader interface {
	Load() ([]*unstructured.Unstructured, error)
}

// NewLoader returns a Loader based on the input source
func NewLoader(input string) (Loader, error) {
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

// StdinLoader implements the Loader interface for reading from stdin
type StdinLoader struct{}

// Load reads the contents from stdin
func (s *StdinLoader) Load() ([]*unstructured.Unstructured, error) {
	stream, err := load(os.Stdin)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML stream from stdin")
	}

	return streamToUnstructured(stream)
}

// FileLoader implements the Loader interface for reading from a file and converting input to unstructured objects
type FileLoader struct {
	path string
}

// Load reads the contents from a file
func (f *FileLoader) Load() ([]*unstructured.Unstructured, error) {
	stream, err := readFile(f.path)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read file")
	}

	return streamToUnstructured(stream)
}

// FolderLoader implements the Loader interface for reading from a folder
type FolderLoader struct {
	path string
}

// Load reads the contents from all files in a folder
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

	return load(f)
}

func load(r io.Reader) ([][]byte, error) {
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
		manifests = append(manifests, u)
	}

	return manifests, nil
}
