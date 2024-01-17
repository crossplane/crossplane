// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
