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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/pkg/version"
	"github.com/crossplane/crossplane/pkg/version/fake"
)

var (
	v1beta1CRDBytes = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: test`)

	v1CRDBytes = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test`)

	provBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: test`)

	confBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: test`)

	xrdBytes = []byte(`apiVersion: apiextentions.crossplane.io/v1alpha1
kind: CompositeResourceDefinition
metadata:
  name: test`)

	compBytes = []byte(`apiVersion: apiextentions.crossplane.io/v1alpha1
kind: Composition
metadata:
  name: test`)

	v1beta1crd = &apiextensions.CustomResourceDefinition{}
	_          = yaml.Unmarshal(v1beta1CRDBytes, v1beta1crd)
	v1crd      = &apiextensions.CustomResourceDefinition{}
	_          = yaml.Unmarshal(v1CRDBytes, v1crd)
	provMeta   = &pkgmeta.Provider{}
	_          = yaml.Unmarshal(provBytes, provMeta)
	confMeta   = &pkgmeta.Configuration{}
	_          = yaml.Unmarshal(confBytes, confMeta)
	xrd        = &apiextensionsv1alpha1.CompositeResourceDefinition{}
	_          = yaml.Unmarshal(xrdBytes, xrd)
	comp       = &apiextensionsv1alpha1.Composition{}
	_          = yaml.Unmarshal(compBytes, comp)

	meta, _ = BuildMetaScheme()
	obj, _  = BuildObjectScheme()
	p       = parser.New(meta, obj)
)

func TestOneMeta(t *testing.T) {
	oneR := bytes.NewReader(bytes.Join([][]byte{v1beta1CRDBytes, provBytes}, []byte("\n---\n")))
	oneMeta, _ := p.Parse(context.TODO(), ioutil.NopCloser(oneR))
	noneR := bytes.NewReader(v1beta1CRDBytes)
	noneMeta, _ := p.Parse(context.TODO(), ioutil.NopCloser(noneR))
	multiR := bytes.NewReader(bytes.Join([][]byte{provBytes, provBytes}, []byte("\n---\n")))
	multiMeta, _ := p.Parse(context.TODO(), ioutil.NopCloser(multiR))

	cases := map[string]struct {
		reason string
		pkg    *parser.Package
		err    error
	}{
		"Successful": {
			reason: "Should not return error if only one meta object.",
			pkg:    oneMeta,
		},
		"ErrNoMeta": {
			reason: "Should return error if no meta objects.",
			pkg:    noneMeta,
			err:    errors.New(errNotExactlyOneMeta),
		},
		"ErrMultiMeta": {
			reason: "Should return error if multiple meta objects.",
			pkg:    multiMeta,
			err:    errors.New(errNotExactlyOneMeta),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := OneMeta(tc.pkg)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nOneMeta(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsProvider(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		"Successful": {
			reason: "Should not return error if object is provider.",
			obj:    provMeta,
		},
		"ErrNotProvider": {
			reason: "Should return error if object is not provider.",
			obj:    v1beta1crd,
			err:    errors.New(errNotMetaProvider),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsProvider(tc.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsProvider(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsConfiguration(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		"Successful": {
			reason: "Should not return error if object is configuration.",
			obj:    confMeta,
		},
		"ErrNotConfiguration": {
			reason: "Should return error if object is not configuration.",
			obj:    v1beta1crd,
			err:    errors.New(errNotMetaConfiguration),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsConfiguration(tc.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsConfiguration(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackageCrossplaneCompatible(t *testing.T) {
	crossplaneConstraint := ">v0.13.0"
	errBoom := errors.New("boom")

	type args struct {
		obj runtime.Object
		ver version.Operations
	}
	cases := map[string]struct {
		reason string
		args   args
		err    error
	}{
		"Successful": {
			reason: "Should not return error if Crossplane version within constraints.",
			args: args{
				obj: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						MetaSpec: pkgmeta.MetaSpec{
							Crossplane: &pkgmeta.CrossplaneConstraints{
								Version: crossplaneConstraint,
							},
						},
					},
				},
				ver: &fake.MockVersioner{
					MockInConstraints: fake.NewMockInConstraintsFn(true, nil),
				},
			},
		},
		"SuccessfulNoConstraints": {
			reason: "Should not return error if no constraints provided.",
			args: args{
				obj: confMeta,
			},
		},
		"ErrInvalidConstraints": {
			reason: "Should return error if constraints are invalid.",
			args: args{
				obj: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						MetaSpec: pkgmeta.MetaSpec{
							Crossplane: &pkgmeta.CrossplaneConstraints{
								Version: crossplaneConstraint,
							},
						},
					},
				},
				ver: &fake.MockVersioner{
					MockInConstraints:    fake.NewMockInConstraintsFn(false, errBoom),
					MockGetVersionString: fake.NewMockGetVersionStringFn("v0.12.0"),
				},
			},
			err: errors.Wrapf(errBoom, errCrossplaneIncompatibleFmt, "v0.12.0"),
		},
		"ErrOutsideConstraints": {
			reason: "Should return error if Crossplane version outside constraints.",
			args: args{
				obj: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						MetaSpec: pkgmeta.MetaSpec{
							Crossplane: &pkgmeta.CrossplaneConstraints{
								Version: crossplaneConstraint,
							},
						},
					},
				},
				ver: &fake.MockVersioner{
					MockInConstraints:    fake.NewMockInConstraintsFn(false, nil),
					MockGetVersionString: fake.NewMockGetVersionStringFn("v0.12.0"),
				},
			},
			err: errors.Errorf(errCrossplaneIncompatibleFmt, "v0.12.0"),
		},
		"ErrNotMeta": {
			reason: "Should return error if object is not a meta package type.",
			args: args{
				obj: v1crd,
			},
			err: errors.New(errNotMeta),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := PackageCrossplaneCompatible(tc.args.ver)(tc.args.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPackageCrossplaneCompatible(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackageValidSemver(t *testing.T) {
	validConstraint := ">v0.13.0"
	invalidConstraint := ">a0.13.0"

	type args struct {
		obj runtime.Object
	}
	cases := map[string]struct {
		reason string
		args   args
		err    error
	}{
		"Valid": {
			reason: "Should not return error if constraints are valid.",
			args: args{
				obj: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						MetaSpec: pkgmeta.MetaSpec{
							Crossplane: &pkgmeta.CrossplaneConstraints{
								Version: validConstraint,
							},
						},
					},
				},
			},
		},
		"ErrInvalidConstraints": {
			reason: "Should return error if constraints are invalid.",
			args: args{
				obj: &pkgmeta.Configuration{
					Spec: pkgmeta.ConfigurationSpec{
						MetaSpec: pkgmeta.MetaSpec{
							Crossplane: &pkgmeta.CrossplaneConstraints{
								Version: invalidConstraint,
							},
						},
					},
				},
			},
			err: errors.Wrap(fmt.Errorf("improper constraint: %s", invalidConstraint), errBadConstraints),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := PackageValidSemver(tc.args.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPackageValidSemver(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsCRD(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		"v1beta1": {
			reason: "Should not return error if object is a v1beta1 CRD.",
			obj:    v1beta1crd,
		},
		"v1": {
			reason: "Should not return error if object is a v1 CRD.",
			obj:    v1crd,
		},
		"ErrNotCRD": {
			reason: "Should return error if object is not CRD.",
			obj:    confMeta,
			err:    errors.New(errNotCRD),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsCRD(tc.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsCRD(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsXRD(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		"Successful": {
			reason: "Should not return error if object is XRD.",
			obj:    xrd,
		},
		"ErrNotConfiguration": {
			reason: "Should return error if object is not XRD.",
			obj:    v1beta1crd,
			err:    errors.New(errNotXRD),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsXRD(tc.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsXRD(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsComposition(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		"Successful": {
			reason: "Should not return error if object is composition.",
			obj:    comp,
		},
		"ErrNotComposition": {
			reason: "Should return error if object is not composition.",
			obj:    v1beta1crd,
			err:    errors.New(errNotComposition),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsComposition(tc.obj)

			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsComposition(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
