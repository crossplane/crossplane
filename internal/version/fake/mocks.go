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

// Package fake contains semantic version mocks.
package fake

import (
	"github.com/Masterminds/semver"

	"github.com/crossplane/crossplane/internal/version"
)

var _ version.Operations = &MockVersioner{}

// MockVersioner provides mock version operations.
type MockVersioner struct {
	MockGetVersionString func() string
	MockGetSemVer        func() (*semver.Version, error)
	MockInConstraints    func() (bool, error)
}

// NewMockGetVersionStringFn creates new MockGetVersionString function for MockVersioner.
func NewMockGetVersionStringFn(s string) func() string {
	return func() string { return s }
}

// NewMockGetSemVerFn creates new MockGetSemver function for MockVersioner.
func NewMockGetSemVerFn(s *semver.Version, err error) func() (*semver.Version, error) {
	return func() (*semver.Version, error) { return s, err }
}

// NewMockInConstraintsFn creates new MockInConstraintsString function for MockVersioner.
func NewMockInConstraintsFn(b bool, err error) func() (bool, error) {
	return func() (bool, error) { return b, err }
}

// GetVersionString calls the underlying MockGetVersionString.
func (m *MockVersioner) GetVersionString() string {
	return m.MockGetVersionString()
}

// GetSemVer calls the underlying MockGetSemVer.
func (m *MockVersioner) GetSemVer() (*semver.Version, error) {
	return m.MockGetSemVer()
}

// InConstraints calls the underlying MockInConstraints.
func (m *MockVersioner) InConstraints(_ string) (bool, error) {
	return m.MockInConstraints()
}
