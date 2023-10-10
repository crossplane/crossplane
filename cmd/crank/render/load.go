package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// LoadCompositeResource from a YAML manifest.
func LoadCompositeResource(file string) (*composite.Unstructured, error) {
	y, err := os.ReadFile(file) //nolint:gosec // Taking this input is intentional.
	if err != nil {
		return nil, errors.Wrap(err, "cannot read composite resource file")
	}
	xr := composite.New()
	return xr, errors.Wrap(yaml.Unmarshal(y, xr), "cannot unmarshal composite resource YAML")
}

// TODO(negz): What if we load a YAML stream of Compositions? We could then
// render out nested XRs too. What would that look like in our output? How would
// we match XRs to Compositions (e.g. selectors, refs etc)

// LoadComposition form a YAML manifest.
func LoadComposition(file string) (*apiextensionsv1.Composition, error) {
	y, err := os.ReadFile(file) //nolint:gosec // Taking this as input is intentional.
	if err != nil {
		return nil, errors.Wrap(err, "cannot read composite resource file")
	}
	comp := &apiextensionsv1.Composition{}
	return comp, errors.Wrap(yaml.Unmarshal(y, comp), "cannot unmarshal composite resource YAML")
}

// TODO(negz): Support optionally loading functions and observed resources from
// a directory of manifests instead of a single stream.

// LoadFunctions from a stream of YAML manifests.
func LoadFunctions(file string) ([]pkgv1beta1.Function, error) {
	stream, err := LoadYAMLStream(file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML stream from file")
	}

	functions := make([]pkgv1beta1.Function, 0, len(stream))
	for _, y := range stream {
		f := &pkgv1beta1.Function{}
		if err := yaml.Unmarshal(y, f); err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML Function manifest")
		}
		functions = append(functions, *f)
	}

	return functions, nil
}

// LoadObservedResources from a stream of YAML manifests.
func LoadObservedResources(file string) ([]composed.Unstructured, error) {
	stream, err := LoadYAMLStream(file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML stream from file")
	}

	observed := make([]composed.Unstructured, 0, len(stream))
	for _, y := range stream {
		cd := composed.New()
		if err := yaml.Unmarshal(y, cd); err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML composed resource manifest")
		}
		observed = append(observed, *cd)
	}

	return observed, nil
}

// LoadYAMLStream from the supplied file or directory. Returns an array of byte
// arrays, where each byte array is expected to be a YAML manifest.
func LoadYAMLStream(fileOrDir string) ([][]byte, error) {
	var files []string
	info, err := os.Stat(fileOrDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat file")
	}
	if !info.IsDir() {
		files = append(files, fileOrDir)
	} else {
		files, err = filepath.Glob(filepath.Join(fileOrDir, "*.{yaml,yml}"))
		if err != nil {
			return nil, errors.Wrap(err, "cannot glob YAML files")
		}
		if len(files) == 0 {
			return nil, errors.Errorf("no YAML files found in %q (.yaml or .yml)", fileOrDir)
		}
	}

	out := make([][]byte, 0)
	for i := range files {
		o, err := LoadYAMLStreamFromFile(files[i])
		if err != nil {
			return nil, errors.Wrap(err, "cannot load YAML stream from file")
		}
		out = append(out, o...)
	}

	return out, nil
}

// LoadYAMLStreamFromFile from the supplied file. Returns an array of byte
// arrays, where each byte array is expected to be a YAML manifest.
func LoadYAMLStreamFromFile(file string) ([][]byte, error) {
	out := make([][]byte, 0)
	f, err := os.Open(file) //nolint:gosec // Taking this input is intentional.
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}
	defer f.Close() //nolint:errcheck // Only open for reading.
	yr := yaml.NewYAMLReader(bufio.NewReader(f))

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
		out = append(out, bytes)
	}
	return out, nil
}
