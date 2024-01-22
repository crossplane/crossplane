// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

var _ Pkg = &Configuration{}
var _ Pkg = &Provider{}

// Pkg is a description of a Crossplane package.
// +k8s:deepcopy-gen=false
type Pkg interface {
	GetCrossplaneConstraints() *CrossplaneConstraints
	GetDependencies() []Dependency
}

// GetCrossplaneConstraints gets the Configuration package's Crossplane version
// constraints.
func (c *Configuration) GetCrossplaneConstraints() *CrossplaneConstraints {
	return c.Spec.MetaSpec.Crossplane
}

// GetDependencies gets the Configuration package's dependencies.
func (c *Configuration) GetDependencies() []Dependency {
	return c.Spec.MetaSpec.DependsOn
}

// GetCrossplaneConstraints gets the Provider package's Crossplane version
// constraints.
func (c *Provider) GetCrossplaneConstraints() *CrossplaneConstraints {
	return c.Spec.MetaSpec.Crossplane
}

// GetDependencies gets the Provider package's dependencies.
func (c *Provider) GetDependencies() []Dependency {
	return c.Spec.MetaSpec.DependsOn
}
