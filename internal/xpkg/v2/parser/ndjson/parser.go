/*
Copyright 2023 The Crossplane Authors.

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

package ndjson

import (
	"bufio"
	"context"
	ejson "encoding/json"
	"errors"
	"io"
	"unicode"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/internal/xpkg/v2"
	"github.com/crossplane/crossplane/internal/xpkg/v2/scheme"
)

const (
	errBuildMetaScheme   = "failed to build meta scheme for package parser"
	errBuildObjectScheme = "failed to build object scheme for package parser"
)

// PackageParser is a Parser implementation for parsing packages. Specifically,
// is used to parse packages from NDJSON files.
type PackageParser struct {
	metaScheme parser.ObjectCreaterTyper
	objScheme  parser.ObjectCreaterTyper
}

// Package is the set of metadata and objects in a package.
type Package struct {
	pmeta   xpkg.ImageMeta
	meta    []runtime.Object
	objects []runtime.Object
}

// GetImageMeta gets the ImageMeta from the package.
func (p *Package) GetImageMeta() xpkg.ImageMeta {
	return p.pmeta
}

// GetMeta gets metadata from the package.
func (p *Package) GetMeta() []runtime.Object {
	return p.meta
}

// GetObjects gets objects from the package.
func (p *Package) GetObjects() []runtime.Object {
	return p.objects
}

// New returns a new NDJSONPackageParser
func New() (*PackageParser, error) {
	metaScheme, err := scheme.BuildMetaScheme()
	if err != nil {
		return nil, errors.New(errBuildMetaScheme)
	}
	objScheme, err := scheme.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	return newParser(metaScheme, objScheme), nil
}

// NewPackage creates a new Package.
func NewPackage() *Package {
	return &Package{}
}

func newParser(meta, obj parser.ObjectCreaterTyper) *PackageParser {
	return &PackageParser{
		metaScheme: meta,
		objScheme:  obj,
	}
}

// Parse is the underlying logic for parsing packages. It first attempts to
// decode objects recognized by the object scheme, then attempts to decode objects
// recognized by the meta scheme. Objects not recognized by either scheme
// return an error rather than being skipped.
func (p *PackageParser) Parse(_ context.Context, reader io.ReadCloser) (*Package, error) { //nolint:gocyclo
	pkg := NewPackage()
	if reader == nil {
		return pkg, nil
	}
	defer func() { _ = reader.Close() }()
	jr := NewReader(bufio.NewReader(reader))
	dm := json.NewSerializerWithOptions(json.DefaultMetaFactory, p.metaScheme, p.metaScheme, json.SerializerOptions{})
	do := json.NewSerializerWithOptions(json.DefaultMetaFactory, p.objScheme, p.objScheme, json.SerializerOptions{})
	for {
		bytes, err := jr.Read()
		if err != nil && !errors.Is(err, io.EOF) {
			return pkg, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if len(bytes) == 0 {
			continue
		}
		m, _, err := do.Decode(bytes, nil, nil)
		if err != nil {
			o, _, err := dm.Decode(bytes, nil, nil)
			if err != nil {
				// attempt to decode ImageMeta
				var imeta xpkg.ImageMeta
				if err := ejson.Unmarshal(bytes, &imeta); err != nil {
					empty := true
					for _, b := range bytes {
						if !unicode.IsSpace(rune(b)) {
							empty = false
							break
						}
					}
					// If the JSON document only contains Unicode White Space we
					// ignore and do not return an error.
					if empty {
						continue
					}
					return pkg, err
				}
				pkg.pmeta = imeta
				continue
			}
			pkg.meta = append(pkg.meta, o)
			continue
		}
		pkg.objects = append(pkg.objects, m)
	}
	return pkg, nil
}
