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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ Linter = &PackageLinter{}

var (
	errBoom = errors.New("boom")

	pkgPass = func(pkg *Package) error {
		return nil
	}
	pkgFail = func(pkg *Package) error {
		return errBoom
	}
	objPass = func(o runtime.Object) error {
		return nil
	}
	objFail = func(o runtime.Object) error {
		return errBoom
	}
)

func TestLinter(t *testing.T) {
	type args struct {
		linter Linter
		pkg    *Package
	}

	cases := map[string]struct {
		reason string
		args   args
		err    error
	}{
		"SuccessfulNoOp": {
			reason: "Passing no checks should always be successful.",
			args: args{
				linter: NewPackageLinter(nil, nil, nil),
				pkg:    NewPackage(),
			},
		},
		"SuccessfulNoObjects": {
			reason: "Passing object linters on empty package should always be successful.",
			args: args{
				linter: NewPackageLinter(nil, ObjectLinterFns(objFail), ObjectLinterFns(objFail)),
				// Object linters do not run if a package has no objects.
				pkg: NewPackage(),
			},
		},
		"SuccessfulWithChecks": {
			reason: "Passing checks for a valid package should always be successful.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(pkgPass), ObjectLinterFns(objPass), ObjectLinterFns(Or(objPass, objFail))),
				pkg: &Package{
					meta:    []runtime.Object{deploy},
					objects: []runtime.Object{crd},
				},
			},
		},
		"ErrorPackageLint": {
			reason: "Passing package linters for an invalid package should always fail.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(pkgFail), ObjectLinterFns(objPass), ObjectLinterFns(objPass)),
				pkg: &Package{
					meta:    []runtime.Object{deploy},
					objects: []runtime.Object{crd},
				},
			},
			err: errBoom,
		},
		"ErrorMetaLint": {
			reason: "Passing meta linters for a package with invalid meta should always fail.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(pkgPass), ObjectLinterFns(objFail), ObjectLinterFns(objPass)),
				pkg: &Package{
					meta:    []runtime.Object{deploy},
					objects: []runtime.Object{crd},
				},
			},
			err: errBoom,
		},
		"ErrorObjectLint": {
			reason: "Passing object linters for a package with invalid objects should always fail.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(pkgPass), ObjectLinterFns(objPass), ObjectLinterFns(Or(objFail, objFail))),
				pkg: &Package{
					meta:    []runtime.Object{deploy},
					objects: []runtime.Object{crd},
				},
			},
			err: errors.Errorf(errOrFmt, errBoom.Error()+", "+errBoom.Error()),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.linter.Lint(tc.args.pkg)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nl.Lint(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

var _ ObjectLinterFn = Or(nil, nil)

func TestOr(t *testing.T) {
	type args struct {
		one ObjectLinterFn
		two ObjectLinterFn
	}

	cases := map[string]struct {
		reason string
		args   args
		err    error
	}{
		"SuccessfulBothPass": {
			reason: "Passing two successful linters should never return error.",
			args: args{
				one: objPass,
				two: objPass,
			},
		},
		"SuccessfulOnePass": {
			reason: "Passing one successful linters should never return error.",
			args: args{
				one: objPass,
				two: objFail,
			},
		},
		"ErrNeitherPass": {
			reason: "Passing two unsuccessful linters should always return error.",
			args: args{
				one: objFail,
				two: objFail,
			},
			err: errors.Errorf(errOrFmt, errBoom.Error()+", "+errBoom.Error()),
		},
		"ErrNilLinter": {
			reason: "Passing a nil linter will should always return error.",
			args: args{
				one: nil,
				two: objPass,
			},
			err: errors.New(errNilLinterFn),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := Or(tc.args.one, tc.args.two)(crd)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nOr(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
