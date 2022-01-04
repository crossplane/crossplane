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

package xpkg

import (
	"github.com/Masterminds/semver"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/internal/version"
)

const (
	errNotExactlyOneMeta         = "not exactly one package meta type"
	errNotMeta                   = "meta type is not a package"
	errNotMetaProvider           = "package meta type is not Provider"
	errNotMetaConfiguration      = "package meta type is not Configuration"
	errNotCRD                    = "object is not a CRD"
	errNotXRD                    = "object is not an XRD"
	errNotComposition            = "object is not a Composition"
	errBadConstraints            = "package version constraints are poorly formatted"
	errCrossplaneIncompatibleFmt = "package is not compatible with Crossplane version (%s)"
)

// NewProviderLinter is a convenience function for creating a package linter for
// providers.
func NewProviderLinter() parser.Linter {
	return parser.NewPackageLinter(parser.PackageLinterFns(OneMeta), parser.ObjectLinterFns(IsProvider, PackageValidSemver), parser.ObjectLinterFns(IsCRD))
}

// NewConfigurationLinter is a convenience function for creating a package linter for
// configurations.
func NewConfigurationLinter() parser.Linter {
	return parser.NewPackageLinter(parser.PackageLinterFns(OneMeta), parser.ObjectLinterFns(IsConfiguration, PackageValidSemver), parser.ObjectLinterFns(parser.Or(IsXRD, IsComposition)))
}

// OneMeta checks that there is only one meta object in the package.
func OneMeta(pkg *parser.Package) error {
	if len(pkg.GetMeta()) != 1 {
		return errors.New(errNotExactlyOneMeta)
	}
	return nil
}

// IsProvider checks that an object is a Provider meta type.
func IsProvider(o runtime.Object) error {
	po, _ := TryConvert(o, &pkgmetav1.Provider{})
	if _, ok := po.(*pkgmetav1.Provider); !ok {
		return errors.New(errNotMetaProvider)
	}
	return nil
}

// IsConfiguration checks that an object is a Configuration meta type.
func IsConfiguration(o runtime.Object) error {
	po, _ := TryConvert(o, &pkgmetav1.Configuration{})
	if _, ok := po.(*pkgmetav1.Configuration); !ok {
		return errors.New(errNotMetaConfiguration)
	}
	return nil
}

// PackageCrossplaneCompatible checks that the current Crossplane version is
// compatible with the package constraints.
func PackageCrossplaneCompatible(v version.Operations) parser.ObjectLinterFn {
	return func(o runtime.Object) error {
		p, ok := TryConvertToPkg(o, &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
		if !ok {
			return errors.New(errNotMeta)
		}

		if p.GetCrossplaneConstraints() == nil {
			return nil
		}
		in, err := v.InConstraints(p.GetCrossplaneConstraints().Version)
		if err != nil {
			return errors.Wrapf(err, errCrossplaneIncompatibleFmt, v.GetVersionString())
		}
		if !in {
			return errors.Errorf(errCrossplaneIncompatibleFmt, v.GetVersionString())
		}
		return nil
	}
}

// PackageValidSemver checks that the package uses valid semver ranges.
func PackageValidSemver(o runtime.Object) error {
	p, ok := TryConvertToPkg(o, &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return errors.New(errNotMeta)
	}

	if p.GetCrossplaneConstraints() == nil {
		return nil
	}
	if _, err := semver.NewConstraint(p.GetCrossplaneConstraints().Version); err != nil {
		return errors.Wrap(err, errBadConstraints)
	}
	return nil
}

// IsCRD checks that an object is a CustomResourceDefinition.
func IsCRD(o runtime.Object) error {
	switch o.(type) {
	case *extv1beta1.CustomResourceDefinition, *extv1.CustomResourceDefinition:
		return nil
	default:
		return errors.New(errNotCRD)
	}
}

// IsXRD checks that an object is a CompositeResourceDefinition.
func IsXRD(o runtime.Object) error {
	if _, ok := o.(*v1.CompositeResourceDefinition); !ok {
		return errors.New(errNotXRD)
	}
	return nil
}

// IsComposition checks that an object is a Composition.
func IsComposition(o runtime.Object) error {
	if _, ok := o.(*v1.Composition); !ok {
		return errors.New(errNotComposition)
	}
	return nil
}
