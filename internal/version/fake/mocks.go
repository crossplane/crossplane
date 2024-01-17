// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

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
