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
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errInvalidPkgName = "invalid package dependency supplied"
)

// ValidDep --
func ValidDep(pkg string) (bool, error) {

	upkg := strings.ReplaceAll(pkg, "@", ":")

	_, err := name.ParseReference(upkg)
	if err != nil {
		return false, errors.Wrap(err, errInvalidPkgName)
	}

	return true, nil
}
