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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"sigs.k8s.io/yaml"
)

const (
	// MetaFile is the name of a Crossplane package metadata file.
	MetaFile string = "crossplane.yaml"

	// StreamFile is the name of the file in a Crossplane package image that
	// contains its YAML stream.
	StreamFile string = "package.yaml"

	// StreamFileMode determines the permissions on the stream file.
	StreamFileMode os.FileMode = 0o644

	// XpkgExtension is the extension for compiled Crossplane packages.
	XpkgExtension string = ".xpkg"

	// XpkgMatchPattern is the match pattern for identifying compiled Crossplane packages.
	XpkgMatchPattern string = "*" + XpkgExtension
)

func truncate(str string, num int) string {
	t := str
	if len(str) > num {
		t = str[0:num]
	}
	return t
}

// FriendlyID builds a maximum 63 character string made up of the name of a
// package and its image digest.
func FriendlyID(name, hash string) string {
	return strings.Join([]string{truncate(name, 50), truncate(hash, 12)}, "-")
}

// BuildPath builds a path for a compiled Crossplane package. If file name has
// extension it will be replaced.
func BuildPath(path, name string) string {
	full := filepath.Join(path, name)
	ext := filepath.Ext(full)
	return full[0:len(full)-len(ext)] + XpkgExtension
}

// ParseNameFromMeta extracts the package name from its meta file.
func ParseNameFromMeta(fs afero.Fs, path string) (string, error) {
	bs, err := afero.ReadFile(fs, filepath.Clean(path))
	if err != nil {
		return "", err
	}
	pkgName, err := parseNameFromPackage(bs)
	if err != nil {
		return "", err
	}
	return pkgName, nil
}

type metaPkg struct {
	Metadata struct {
		Name string `json:"name"`
	}
}

func parseNameFromPackage(bs []byte) (string, error) {
	p := &metaPkg{}
	err := yaml.Unmarshal(bs, p)
	return p.Metadata.Name, err
}
