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
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ Linter = &PackageLinter{}

var crdBytes = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test`)

var providerBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: test`)

var compBytes = []byte(`apiVersion: apiextensions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: test`)

func TestLinter(t *testing.T) {
	allBytes := bytes.Join([][]byte{crdBytes, compBytes, providerBytes}, []byte("\n---\n"))
	metaScheme, _ := BuildMetaScheme()
	objScheme, _ := BuildObjectScheme()
	p := parser.New(metaScheme, objScheme)
	b := parser.NewEchoBackend(string(allBytes))
	r, _ := b.Init(context.TODO())
	pkg, _ := p.Parse(context.TODO(), r)

	type args struct {
		linter Linter
		pkg    *parser.Package
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
				pkg:    pkg,
			},
		},
		"SuccessfulWithChecks": {
			reason: "Passingchecks for a valid package should always be successful.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(OneMeta), ObjectLinterFns(IsProvider), ObjectLinterFns(Or(IsCRD, IsComposition))),
				pkg:    pkg,
			},
		},
		"ErrorWithChecks": {
			reason: "Passing checks for an invalid package should always fail.",
			args: args{
				linter: NewPackageLinter(PackageLinterFns(OneMeta), ObjectLinterFns(IsProvider), ObjectLinterFns(Or(IsCRD, IsXRD))),
				pkg:    pkg,
			},
			err: fmt.Errorf("object did not pass either check: (%v), (%v)", errNotCRD, errNotXRD),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.linter.Lint(tc.args.pkg)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Lint(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
