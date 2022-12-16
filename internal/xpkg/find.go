/*
Copyright 2020 The Crossplane Authors.

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
