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

// Package version contains utilities for working with semantic versions.
package version

import (
	"github.com/Masterminds/semver"
)

var version string

// Operations provides semantic version operations.
type Operations interface {
	GetVersionString() string
	GetSemVer() (*semver.Version, error)
	InConstraints(c string) (bool, error)
}

// Versioner provides semantic version operations.
type Versioner struct {
	version string
}

// New creates a new versioner.
func New() *Versioner {
	return &Versioner{
		version: version,
	}
}

// GetVersionString returns the current Crossplane version as string.
func (v *Versioner) GetVersionString() string {
	return v.version
}

// GetSemVer returns the current Crossplane version as a semantic version.
func (v *Versioner) GetSemVer() (*semver.Version, error) {
	return semver.NewVersion(v.version)
}

// InConstraints is a helper function that checks if the current Crossplane
// version is in the semantic version constraints.
func (v *Versioner) InConstraints(c string) (bool, error) {
	ver, err := v.GetSemVer()
	if err != nil {
		return false, err
	}
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		return false, err
	}
	return constraint.Check(ver), nil
}
