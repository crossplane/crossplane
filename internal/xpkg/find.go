// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package xpkg

import (
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errNoMatch    = "directory does not contain a compiled crossplane package"
	errMultiMatch = "directory contains multiple compiled crossplane packages"
)

// FindXpkgInDir finds compiled Crossplane packages in a directory.
func FindXpkgInDir(fs afero.Fs, root string) (string, error) {
	f, err := fs.Open(root)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	files, err := f.Readdir(-1)
	if err != nil {
		return "", err
	}
	path := ""
	for _, file := range files {
		// Match only returns an error if XpkgMatchPattern is malformed.
		match, _ := filepath.Match(XpkgMatchPattern, file.Name())
		if !match {
			continue
		}
		if path != "" && match {
			return "", errors.New(errMultiMatch)
		}
		path = file.Name()
	}
	if path == "" {
		return "", errors.New(errNoMatch)
	}
	return path, nil
}
