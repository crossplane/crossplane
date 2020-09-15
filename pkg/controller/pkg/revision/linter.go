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

package revision

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/parser"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

const (
	errMultipleMeta         = "not exactly one package meta type"
	errNotMetaProvider      = "package meta type is not Provider"
	errNotMetaConfiguration = "package meta type is not Configuration"
	errNotCRD               = "object is not a CRD"
	errNotXRD               = "object is not an XRD"
	errNotComposition       = "object is not a Composition"
)

// A Linter lints packages.
type Linter interface {
	Lint(*parser.Package) error
}

// PackageLinterFn lints an entire package. If function applies a check for
// multiple objects, consider using an ObjectLinterFn
type PackageLinterFn func(*parser.Package) error

// PackageLinterFns is a convenience function to pass multiple PackageLinterFn
// to a function that cannot accept variadic arguments.
func PackageLinterFns(fns ...PackageLinterFn) []PackageLinterFn {
	return fns
}

// ObjectLinterFn lints an object in a package.
type ObjectLinterFn func(runtime.Object) error

// ObjectLinterFns is a convenience function to pass multiple ObjectLinterFn to
// a function that cannot accept variadic arguments.
func ObjectLinterFns(fns ...ObjectLinterFn) []ObjectLinterFn {
	return fns
}

// PackageLinter lints packages by applying package and object linter functions
// to it.
type PackageLinter struct {
	pre       []PackageLinterFn
	perMeta   []ObjectLinterFn
	perObject []ObjectLinterFn
}

// NewPackageLinter creates a new PackageLinter.
func NewPackageLinter(pre []PackageLinterFn, perMeta, perObject []ObjectLinterFn) *PackageLinter {
	return &PackageLinter{
		pre:       pre,
		perMeta:   perMeta,
		perObject: perObject,
	}
}

// Lint executes all linter functions against a package.
func (l *PackageLinter) Lint(pkg *parser.Package) error {
	for _, fn := range l.pre {
		if err := fn(pkg); err != nil {
			return err
		}
	}
	for _, o := range pkg.GetMeta() {
		for _, fn := range l.perMeta {
			if err := fn(o); err != nil {
				return err
			}
		}
	}
	for _, o := range pkg.GetObjects() {
		for _, fn := range l.perObject {
			if err := fn(o); err != nil {
				return err
			}
		}
	}
	return nil
}

// OneMeta checks that there is only one meta object in the package.
func OneMeta(pkg *parser.Package) error {
	if len(pkg.GetMeta()) != 1 {
		return errors.New(errMultipleMeta)
	}
	return nil
}

// IsProvider checks that an object is a Provider meta type.
func IsProvider(o runtime.Object) error {
	if _, ok := o.(*pkgmeta.Provider); !ok {
		return errors.New(errNotMetaProvider)
	}
	return nil
}

// IsConfiguration checks that an object is a Configuration meta type.
func IsConfiguration(o runtime.Object) error {
	if _, ok := o.(*pkgmeta.Configuration); !ok {
		return errors.New(errNotMetaConfiguration)
	}
	return nil
}

// IsCRD checks that an object is a CustomResourceDefinition.
func IsCRD(o runtime.Object) error {
	if _, ok := o.(*v1beta1.CustomResourceDefinition); !ok {
		return errors.New(errNotCRD)
	}
	return nil
}

// IsXRD checks that an object is a CompositeResourceDefinition.
func IsXRD(o runtime.Object) error {
	if _, ok := o.(*apiextensionsv1alpha1.CompositeResourceDefinition); !ok {
		return errors.New(errNotXRD)
	}
	return nil
}

// IsComposition checks that an object is a Composition.
func IsComposition(o runtime.Object) error {
	if _, ok := o.(*apiextensionsv1alpha1.Composition); !ok {
		return errors.New(errNotComposition)
	}
	return nil
}

// Or checks that at least one of the passed linter functions does not return an
// error.
func Or(a, b ObjectLinterFn) ObjectLinterFn {
	return func(o runtime.Object) error {
		aErr := a(o)
		bErr := b(o)
		if aErr == nil || bErr == nil {
			return nil
		}
		return fmt.Errorf("object did not pass either check: (%v), (%v)", aErr, bErr)
	}
}
