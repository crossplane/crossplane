// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bufio"
	"io"
	"path/filepath"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// LoadCompositeResource from a YAML manifest.
func LoadCompositeResource(fs afero.Fs, file string) (*composite.Unstructured, error) {
	y, err := afero.ReadFile(fs, file)
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
func LoadComposition(fs afero.Fs, file string) (*apiextensionsv1.Composition, error) {
	y, err := afero.ReadFile(fs, file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read composition file")
	}
	comp := &apiextensionsv1.Composition{}
	if err := yaml.Unmarshal(y, comp); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal composition resource YAML")
	}
	switch gvk := comp.GroupVersionKind(); gvk {
	case apiextensionsv1.CompositionGroupVersionKind:
		return comp, nil
	default:
		return nil, errors.Errorf("not a composition: %s/%s", gvk.Kind, comp.GetName())
	}
}

// TODO(negz): Support optionally loading functions and observed resources from
// a directory of manifests instead of a single stream.

// LoadFunctions from a stream of YAML manifests.
func LoadFunctions(filesys afero.Fs, file string) ([]pkgv1beta1.Function, error) {
	stream, err := LoadYAMLStream(filesys, file)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load YAML stream from file")
	}

	functions := make([]pkgv1beta1.Function, 0, len(stream))
	for _, y := range stream {
		f := &pkgv1beta1.Function{}
		if err := yaml.Unmarshal(y, f); err != nil {
			return nil, errors.Wrap(err, "cannot parse YAML Function manifest")
		}
		switch gvk := f.GroupVersionKind(); gvk {
		case pkgv1beta1.FunctionGroupVersionKind:
			functions = append(functions, *f)
		default:
			return nil, errors.Errorf("not a function: %s/%s", gvk.Kind, f.GetName())
		}
	}

	return functions, nil
}

// LoadObservedResources from a stream of YAML manifests.
func LoadObservedResources(fs afero.Fs, file string) ([]composed.Unstructured, error) {
	stream, err := LoadYAMLStream(fs, file)
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
func LoadYAMLStream(filesys afero.Fs, fileOrDir string) ([][]byte, error) {
	var files []string
	f, err := filesys.Open(fileOrDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}
	info, err := f.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat file")
	}
	if !info.IsDir() {
		files = append(files, fileOrDir)
	} else {
		yamls, err := getYAMLFiles(filesys, fileOrDir)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get YAML files")
		}
		files = append(files, yamls...)
		if len(files) == 0 {
			return nil, errors.Errorf("no YAML files found in %q (.yaml or .yml)", fileOrDir)
		}
	}

	out := make([][]byte, 0)
	for i := range files {
		o, err := LoadYAMLStreamFromFile(filesys, files[i])
		if err != nil {
			return nil, errors.Wrap(err, "cannot load YAML stream from file")
		}
		out = append(out, o...)
	}

	return out, nil
}

// getYAMLFiles returns a list of YAML files from the supplied directory, sorted
// by file name, ignoring any subdirectory.
func getYAMLFiles(fs afero.Fs, dir string) (files []string, err error) {
	// We don't care about nested directories, so we decided to go with a plain
	// ReadDir, instead of a Walk.
	//
	// Previously we used Glob, but the pattern doesn't support the
	// `.{yaml,yml}` syntax, so we would have had to run it twice, merge the
	// results and sort them again. This just felt easier to switch to afero.Walk if
	// we ever decided to support subdirectories.
	entries, err := afero.ReadDir(fs, dir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read directory")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			// We don't care about nested directories.
			continue
		}
		switch filepath.Ext(entry.Name()) {
		case ".yaml", ".yml":
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}

// LoadYAMLStreamFromFile from the supplied file. Returns an array of byte
// arrays, where each byte array is expected to be a YAML manifest.
func LoadYAMLStreamFromFile(fs afero.Fs, file string) ([][]byte, error) {
	out := make([][]byte, 0)
	f, err := fs.Open(file)
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
