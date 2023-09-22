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

package meta

import (
	"encoding/json"
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/manager"
	"github.com/crossplane/crossplane/internal/xpkg/v2/scheme"
)

const (
	errInvalidMetaFile           = "invalid meta type supplied"
	errMetaContainsDupeDep       = "meta file contains duplicate dependency"
	errUnsupportedPackageVersion = "unsupported package version supplied"
)

// Meta provides helpful methods for interacting with a metafile's
// runtime.Object.
type Meta struct {
	// obj is the runtime.Object representation of the meta file.
	obj runtime.Object
}

// New constructs a new Meta given a
func New(obj runtime.Object) *Meta {
	return &Meta{
		obj: obj,
	}
}

// DependsOn returns a slice of v1beta1.Dependency that this workspace depends on.
func (m *Meta) DependsOn() ([]v1beta1.Dependency, error) {
	pkg, ok := scheme.TryConvertToPkg(m.obj, &v1.Provider{}, &v1.Configuration{})
	if !ok {
		return nil, errors.New(errUnsupportedPackageVersion)
	}

	out := make([]v1beta1.Dependency, len(pkg.GetDependencies()))
	for i, d := range pkg.GetDependencies() {
		out[i] = manager.ConvertToV1beta1(d)
	}

	return out, nil
}

// Upsert will add an entry to the meta file, if the meta file exists and
// does not yet have an entry for the given package. If an entry does exist,
// the entry will be updated to the given package version.
func (m *Meta) Upsert(d v1beta1.Dependency) error {
	return upsertDeps(d, m.obj)
}

// Bytes returns the cleaned up byte representation of the meta file obj.
func (m *Meta) Bytes() ([]byte, error) {
	data, err := sigsyaml.Marshal(m.obj)
	if err != nil {
		return nil, err
	}

	// (@tnthornton) workaround for `creationTimestamp: null` in marshaled result.
	// see https://github.com/kubernetes/kubernetes/pull/104857 for inspiration
	t := apimetav1.Time{}

	switch v := m.obj.(type) {
	case *v1alpha1.Configuration:
		t = v.GetCreationTimestamp()
	case *v1.Configuration:
		t = v.GetCreationTimestamp()
	case *v1alpha1.Provider:
		t = v.GetCreationTimestamp()
	case *v1.Provider:
		t = v.GetCreationTimestamp()
	default:
		return nil, errors.New(errInvalidMetaFile)
	}

	if t.Equal(&apimetav1.Time{}) {
		// the timestamp is empty, we need to clean it from the resulting
		// file data
		data, err = cleanNullTs(m.obj)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

// upsertDeps takes a v1beta1.Dependency and a runtime.Object of type that can
// be converted to a v1.Pkg and returns an updated runtime.Object with a slice
// of dependencies that includes the provided dependency d.
func upsertDeps(d v1beta1.Dependency, o runtime.Object) error { //nolint:gocyclo
	p, ok := scheme.TryConvertToPkg(o, &v1.Provider{}, &v1.Configuration{})
	if !ok {
		return errors.New(errUnsupportedPackageVersion)
	}
	deps := p.GetDependencies()

	processed := false
	for i := range deps {
		// modify the underlying slice
		dep := deps[i]
		if dep.Provider != nil && *dep.Provider == d.Package {
			if processed {
				return errors.New(errMetaContainsDupeDep)
			}
			deps[i].Version = d.Constraints
			processed = true
		} else if dep.Configuration != nil && *dep.Configuration == d.Package {
			if processed {
				return errors.New(errMetaContainsDupeDep)
			}
			deps[i].Version = d.Constraints
			processed = true
		}
	}

	if !processed {

		dep := v1.Dependency{
			Version: d.Constraints,
		}

		if d.Type == v1beta1.ProviderPackageType {
			dep.Provider = &d.Package
		} else {
			dep.Configuration = &d.Package
		}

		deps = append(deps, dep)
	}

	switch v := o.(type) {
	case *v1alpha1.Configuration:
		v.Spec.DependsOn = convertToV1alpha1(deps)
	case *v1.Configuration:
		v.Spec.DependsOn = deps
	case *v1alpha1.Provider:
		v.Spec.DependsOn = convertToV1alpha1(deps)
	case *v1.Provider:
		v.Spec.DependsOn = deps
	}

	return nil
}

// cleanNullTs is a helper function for cleaning the erroneous
// `creationTimestamp: null` from the marshaled data that we're
// going to writer to the meta file.
func cleanNullTs(p runtime.Object) ([]byte, error) {
	ob, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	err = json.Unmarshal(ob, &m)
	if err != nil {
		return nil, err
	}
	// remove the erroneous creationTimestamp: null entry
	delete(m["metadata"].(map[string]any), "creationTimestamp")

	return sigsyaml.Marshal(m)
}

func convertToV1alpha1(deps []v1.Dependency) []v1alpha1.Dependency {
	alphaDeps := make([]v1alpha1.Dependency, 0)
	for _, d := range deps {
		alphaDeps = append(alphaDeps, manager.ConvertToV1alpha1(d))
	}
	return alphaDeps
}
