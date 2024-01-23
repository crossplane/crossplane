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

// Package io is a package for reading and writing files for the migration
// command. Possibly this should be moved as a general package for the cli.
package io

import (
	"io"
	"os"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Read reads the input from the given file or stdin if no file is given.
func Read(fs afero.Fs, inputFile string) ([]byte, error) {
	var data []byte
	var err error

	if inputFile != "-" {
		data, err = afero.ReadFile(fs, inputFile)
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read inputFile")
	}
	return data, nil
}

// WriteObjectYAML writes the given object to the given file or stdout if no
// file is given. The output format is YAML.
func WriteObjectYAML(fs afero.Fs, outputFile string, o runtime.Object) error {
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})

	var output io.Writer

	if outputFile != "" {
		f, err := fs.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return errors.Wrap(err, "Unable to open output file")
		}
		defer func() { _ = f.Close() }()
		output = f
	} else {
		output = os.Stdout
	}

	err := s.Encode(o, output)
	if err != nil {
		return errors.Wrap(err, "Unable to encode output")
	}
	return nil
}
