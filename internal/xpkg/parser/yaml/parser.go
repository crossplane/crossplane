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

// Package yaml contains utilities for reading yaml packages.
package yaml

import (
	"errors"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errBuildMetaScheme   = "failed to build meta scheme for package parser"
	errBuildObjectScheme = "failed to build object scheme for package parser"
)

// New returns a new PackageParser that targets yaml files.
func New() (*parser.PackageParser, error) {
	metaScheme, err := xpkg.BuildMetaScheme()
	if err != nil {
		return nil, errors.New(errBuildMetaScheme)
	}
	objScheme, err := xpkg.BuildObjectScheme()
	if err != nil {
		return nil, errors.New(errBuildObjectScheme)
	}

	return parser.New(metaScheme, objScheme), nil
}
