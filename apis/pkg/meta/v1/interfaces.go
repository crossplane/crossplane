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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

var (
	_ Pkg = &Configuration{}
	_ Pkg = &Provider{}
	_ Pkg = &Function{}
)

// Pkg is a description of a Crossplane package.
// +k8s:deepcopy-gen=false
type Pkg interface {
	runtime.Object
	metav1.Object

	GetCrossplaneConstraints() *CrossplaneConstraints
	GetDependencies() []Dependency
	GetCapabilities() []string
}

// GetCrossplaneConstraints gets the Configuration package's Crossplane version
// constraints.
func (c *Configuration) GetCrossplaneConstraints() *CrossplaneConstraints {
	return c.Spec.Crossplane
}

// GetDependencies gets the Configuration package's dependencies.
func (c *Configuration) GetDependencies() []Dependency {
	return c.Spec.DependsOn
}

// GetCapabilities gets the Configuration package's capabilities.
func (c *Configuration) GetCapabilities() []string {
	return c.Spec.Capabilities
}

// GetCrossplaneConstraints gets the Provider package's Crossplane version
// constraints.
func (p *Provider) GetCrossplaneConstraints() *CrossplaneConstraints {
	return p.Spec.Crossplane
}

// GetDependencies gets the Provider package's dependencies.
func (p *Provider) GetDependencies() []Dependency {
	return p.Spec.DependsOn
}

// GetCapabilities gets the Provider package's capabilities.
func (p *Provider) GetCapabilities() []string {
	return p.Spec.Capabilities
}

// GetCrossplaneConstraints gets the Function package's Crossplane version constraints.
func (f *Function) GetCrossplaneConstraints() *CrossplaneConstraints {
	return f.Spec.Crossplane
}

// GetDependencies gets the Function package's dependencies.
func (f *Function) GetDependencies() []Dependency {
	return f.Spec.DependsOn
}

// GetCapabilities gets the Function package's capabilities.
func (f *Function) GetCapabilities() []string {
	// If a function doesn't include any capabilities we assume it's a
	// composition function. Composition functions predate the concept of
	// function capabilities, so there are many composition functions that
	// don't explicitly specify any capabilities.
	if f.Spec.Capabilities == nil {
		return []string{FunctionCapabilityComposition}
	}

	return f.Spec.Capabilities
}
