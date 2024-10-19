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

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

var (
	_ Pkg = &Configuration{}
	_ Pkg = &Provider{}
	_ Pkg = &Function{}
)

// Pkg is a description of a Crossplane package.
// +k8s:deepcopy-gen=false
type Pkg interface {
	metav1.Object
	GetCrossplaneConstraints() *CrossplaneConstraints
	GetDependencies() []Dependency
	GetReplaces() []string
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

// GetReplaces gets the package sources the Configuration replaces.
func (c *Configuration) GetReplaces() []string {
	return c.Spec.Replaces
}

// GetCrossplaneConstraints gets the Provider package's Crossplane version
// constraints.
func (p *Provider) GetCrossplaneConstraints() *CrossplaneConstraints {
	return p.Spec.MetaSpec.Crossplane
}

// GetDependencies gets the Provider package's dependencies.
func (p *Provider) GetDependencies() []Dependency {
	return p.Spec.MetaSpec.DependsOn
}

// GetReplaces gets the package sources the Provider replaces.
func (p *Provider) GetReplaces() []string {
	return p.Spec.Replaces
}

// GetCrossplaneConstraints gets the Function package's Crossplane version constraints.
func (f *Function) GetCrossplaneConstraints() *CrossplaneConstraints {
	return f.Spec.MetaSpec.Crossplane
}

// GetDependencies gets the Function package's dependencies.
func (f *Function) GetDependencies() []Dependency {
	return f.Spec.DependsOn
}

// GetReplaces gets the package sources the Function replaces.
func (f *Function) GetReplaces() []string {
	return f.Spec.Replaces
}
