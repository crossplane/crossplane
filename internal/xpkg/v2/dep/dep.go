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

package dep

import (
	"strings"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
)

// New returns a new v1beta1.Dependency based on the given package name
// Expects names of the form source@version where @version can be
// left blank in order to indicate 'latest'.
func New(pkg string) v1beta1.Dependency {

	// if the passed in ver was blank use the default to pass
	// constraint checks and grab latest semver
	version := image.DefaultVer

	ps := strings.Split(pkg, "@")

	source := ps[0]
	if len(ps) == 2 {
		version = ps[1]
	}

	return v1beta1.Dependency{
		Package:     source,
		Constraints: version,
	}
}

// NewWithType returns a new v1beta1.Dependency based on the given package
// name and PackageType (represented as a string).
// Expects names of the form source@version where @version can be
// left blank in order to indicate 'latest'.
func NewWithType(pkg string, t string) v1beta1.Dependency {
	d := New(pkg)

	d.Type = v1beta1.ProviderPackageType
	if strings.Title(strings.ToLower(t)) == string(v1beta1.ConfigurationPackageType) { //nolint:staticcheck // ignore staticcheck for now
		d.Type = v1beta1.ConfigurationPackageType
	}

	return d
}
