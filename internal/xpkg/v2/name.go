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

package xpkg

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// JSONStreamFile is the name of the file in local Crossplane package
	// that contains the JSON stream representation of the Crossplane package.
	JSONStreamFile string = "package.ndjson"

	// MetaFile is the name of a Crossplane package metadata file.
	MetaFile string = "crossplane.yaml"

	// StreamFile is the name of the file in a Crossplane package image that
	// contains its YAML stream.
	StreamFile string = "package.yaml"

	// StreamFileMode determines the permissions on the stream file.
	StreamFileMode os.FileMode = 0o644

	// XpkgExtension is the extension for compiled Crossplane packages.
	XpkgExtension string = ".xpkg"

	// XpkgMatchPattern is the match pattern for identifying compiled Crossplane
	// packages.
	XpkgMatchPattern string = "*" + XpkgExtension

	// XpkgExamplesFile is the name of the file in a Crossplane package image
	// that contains the examples YAML stream.
	XpkgExamplesFile string = ".up/examples.yaml"

	// AnnotationKey is the key value for xpkg annotations.
	AnnotationKey string = "io.crossplane.xpkg"
	// PackageAnnotation is the annotation value used for the package.yaml
	// layer.
	PackageAnnotation string = "base"
	// ExamplesAnnotation is the annotation value used for the examples.yaml
	// layer.
	ExamplesAnnotation string = "upbound"
)

func truncate(str string, num int) string {
	t := str
	if len(str) > num {
		t = str[0:num]
	}
	return t
}

// FriendlyID builds a valid DNS label string made up of the name of a package
// and its image digest.
func FriendlyID(name, hash string) string {
	return ToDNSLabel(strings.Join([]string{truncate(name, 50), truncate(hash, 12)}, "-"))
}

// ToDNSLabel converts the string to a valid DNS label.
func ToDNSLabel(s string) string { //nolint:gocyclo
	var cut strings.Builder
	for i := range s {
		b := s[i]
		if ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') {
			cut.WriteByte(b)
		}
		if (b == '.' || b == '/' || b == ':' || b == '-') && (i != 0 && i != 62 && i != len(s)-1) {
			cut.WriteByte('-')
		}
		if i == 62 {
			break
		}
	}
	return strings.Trim(cut.String(), "-")
}

// BuildPath builds a path for a compiled Crossplane package. If file name has
// extension it will be replaced.
func BuildPath(path, name string) string {
	full := filepath.Join(path, name)
	return ReplaceExt(full, XpkgExtension)
}

// ReplaceExt replaces the file extension of the given path.
func ReplaceExt(path, ext string) string {
	old := filepath.Ext(path)
	return path[0:len(path)-len(old)] + ext
}
