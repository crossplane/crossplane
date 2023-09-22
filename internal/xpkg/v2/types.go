// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xpkg

// Package represents the types of packages we support.
type Package string

const (
	// Configuration represents a configuration package.
	Configuration Package = "configuration"
	// Provider represents a provider package.
	Provider Package = "provider"
)

// IsValid is a helper function for determining if the Package
// is a valid type of package.
func (p Package) IsValid() bool {
	switch p {
	case Configuration, Provider:
		return true
	}
	return false
}
