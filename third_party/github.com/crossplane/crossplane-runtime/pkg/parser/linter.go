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

package parser

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errNilLinterFn = "linter function is nil"

	errOrFmt = "object did not pass any of the linters with following errors: %s"
)

// A Linter lints packages.
type Linter interface {
	Lint(*Package) error
}

// PackageLinterFn lints an entire package. If function applies a check for
// multiple objects, consider using an ObjectLinterFn.
type PackageLinterFn func(*Package) error

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
func (l *PackageLinter) Lint(pkg *Package) error {
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

// Or checks that at least one of the passed linter functions does not return an
// error.
func Or(linters ...ObjectLinterFn) ObjectLinterFn {
	return func(o runtime.Object) error {
		var errs []string
		for _, l := range linters {
			if l == nil {
				return errors.New(errNilLinterFn)
			}
			err := l(o)
			if err == nil {
				return nil
			}
			errs = append(errs, err.Error())
		}
		return errors.Errorf(errOrFmt, strings.Join(errs, ", "))
	}
}
