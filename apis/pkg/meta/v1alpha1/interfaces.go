package v1alpha1

import (
	"github.com/crossplane/crossplane/apis/pkg/meta/v1"
)

var _ v1.Pkg = &Function{}

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
