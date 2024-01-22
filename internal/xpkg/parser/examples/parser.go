// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Package examples contains utilities for parsing examples.
package examples

import (
	"bufio"
	"context"
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"
)

// Examples is the set of metadata and objects in a package.
type Examples struct {
	objects []unstructured.Unstructured
}

// Parser is a Parser implementation for parsing examples.
type Parser struct {
	objScheme parser.ObjectCreaterTyper
}

// NewExamples creates a new Examples object.
func NewExamples() *Examples {
	return &Examples{}
}

// New creates a new Package.
func New() *Parser {
	return &Parser{}
}

// Parse is the underlying logic for parsing examples.
func (p *Parser) Parse(ctx context.Context, reader io.ReadCloser) (*Examples, error) {
	ex := NewExamples()
	if reader == nil {
		return ex, nil
	}
	defer func() { _ = reader.Close() }()
	yr := yaml.NewYAMLReader(bufio.NewReader(reader))
	for {
		bytes, err := yr.Read()
		if err != nil && err != io.EOF {
			return ex, err
		}
		if err == io.EOF {
			break
		}
		if len(bytes) == 0 {
			continue
		}
		var obj unstructured.Unstructured
		if err := k8syaml.Unmarshal(bytes, &obj); err != nil {
			return ex, err
		}
		ex.objects = append(ex.objects, obj)
	}
	return ex, nil
}
