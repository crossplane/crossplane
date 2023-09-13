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

package v1beta1

import (
	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

// GetCrossplaneConstraints gets the Function package's Crossplane version constraints.
func (f *Function) GetCrossplaneConstraints() *v1.CrossplaneConstraints {
	if f.Spec.MetaSpec.Crossplane == nil {
		return nil
	}

	cc := v1.CrossplaneConstraints{Version: f.Spec.MetaSpec.Crossplane.Version}
	return &cc
}

// GetDependencies gets the Function package's dependencies.
func (f *Function) GetDependencies() []v1.Dependency {
	if f.Spec.MetaSpec.DependsOn == nil {
		return []v1.Dependency{}
	}

	d := make([]v1.Dependency, len(f.Spec.MetaSpec.DependsOn))
	for i, dep := range f.Spec.MetaSpec.DependsOn {
		d[i] = v1.Dependency{
			Provider:      dep.Provider,
			Configuration: dep.Configuration,
			Function:      dep.Function,
			Version:       dep.Version,
		}
	}

	return d
}
