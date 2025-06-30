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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	pkgmetav1beta1 "github.com/crossplane/crossplane/apis/pkg/meta/v1beta1"
	"github.com/crossplane/crossplane/internal/version"
	"github.com/crossplane/crossplane/internal/version/fake"
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

	v1alpha1ProvBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: test`)

	v1alpha1ConfBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Configuration
metadata:
  name: test`)

	v1beta1FuncBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1beta1
  kind: Function
  metadata:
    name: test`)

	v1ProvBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: test`)

	v1ConfBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: test`)

	v1FuncBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Function
metadata:
  name: test`)

	v1XRDBytes = []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: test`)

	v2XRDBytes = []byte(`apiVersion: apiextensions.crossplane.io/v2
kind: CompositeResourceDefinition
metadata:
  name: test`)

	v1CompBytes = []byte(`apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: test`)

	v1beta1crd       = &apiextensions.CustomResourceDefinition{}
	_                = yaml.Unmarshal(v1beta1CRDBytes, v1beta1crd)
	v1crd            = &apiextensions.CustomResourceDefinition{}
	_                = yaml.Unmarshal(v1CRDBytes, v1crd)
	v1alpha1ProvMeta = &pkgmetav1alpha1.Provider{}
	_                = yaml.Unmarshal(v1alpha1ProvBytes, v1alpha1ProvMeta)
	v1alpha1ConfMeta = &pkgmetav1alpha1.Configuration{}
	_                = yaml.Unmarshal(v1alpha1ConfBytes, v1alpha1ConfMeta)
	v1beta1FuncMeta  = &pkgmetav1beta1.Function{}
	_                = yaml.Unmarshal(v1beta1FuncBytes, v1beta1FuncMeta)
	v1ProvMeta       = &pkgmetav1.Provider{}
	_                = yaml.Unmarshal(v1ProvBytes, v1ProvMeta)
	v1ConfMeta       = &pkgmetav1.Configuration{}
	_                = yaml.Unmarshal(v1ConfBytes, v1ConfMeta)
	v1FuncMeta       = &pkgmetav1.Function{}
	_                = yaml.Unmarshal(v1FuncBytes, v1FuncMeta)
	v1XRD            = &v1.CompositeResourceDefinition{}
	_                = yaml.Unmarshal(v1XRDBytes, v1XRD)
	v2XRD            = &v2.CompositeResourceDefinition{}
	_                = yaml.Unmarshal(v2XRDBytes, v2XRD)
	v1Comp           = &v1.Composition{}
	_                = yaml.Unmarshal(v1CompBytes, v1Comp)

	meta, _ = BuildMetaScheme()
	obj, _  = BuildObjectScheme()
	p       = parser.New(meta, obj)
)

func TestOneMeta(t *testing.T) {
	oneR := bytes.NewReader(bytes.Join([][]byte{v1beta1CRDBytes, v1alpha1ProvBytes}, []byte("\n---\n")))
	oneMeta, _ := p.Parse(context.TODO(), io.NopCloser(oneR))
	noneR := bytes.NewReader(v1beta1CRDBytes)
	noneMeta, _ := p.Parse(context.TODO(), io.NopCloser(noneR))
	multiR := bytes.NewReader(bytes.Join([][]byte{v1alpha1ProvBytes, v1alpha1ProvBytes}, []byte("\n---\n")))
	multiMeta, _ := p.Parse(context.TODO(), io.NopCloser(multiR))

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
		"v1alpha1": {
			reason: "Should not return error if object is a v1alpha1 provider.",
			obj:    v1alpha1ProvMeta,
		},
		"v1": {
			reason: "Should not return error if object is a v1 provider.",
			obj:    v1ProvMeta,
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
		"v1alpha1": {
			reason: "Should not return error if object is a v1alpha1 configuration.",
			obj:    v1alpha1ConfMeta,
		},
		"v1": {
			reason: "Should not return error if object is a v1 configuration.",
			obj:    v1ConfMeta,
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

func TestIsFunction(t *testing.T) {
	cases := map[string]struct {
		reason string
		obj    runtime.Object
		err    error
	}{
		// Function packages were introduced at v1beta1. There was never a
		// v1alpha1 version of the package metadata.
		"v1beta1": {
			reason: "Should not return error if object is a v1beta1 function.",
			obj:    v1beta1FuncMeta,
		},
		"v1": {
			reason: "Should not return error if object is a v1 function.",
			obj:    v1FuncMeta,
		},
		"ErrNotFunction": {
			reason: "Should return error if object is not function.",
			obj:    v1beta1crd,
			err:    errors.New(errNotMetaFunction),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := IsFunction(tc.obj)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsFunction(...): -want error, +got error:\n%s", tc.reason, diff)
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
				obj: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
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
				obj: v1ProvMeta,
			},
		},
		"ErrInvalidConstraints": {
			reason: "Should return error if constraints are invalid.",
			args: args{
				obj: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
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
			err: errors.Wrapf(errBoom, errFmtCrossplaneIncompatible, "v0.12.0"),
		},
		"ErrOutsideConstraints": {
			reason: "Should return error if Crossplane version outside constraints.",
			args: args{
				obj: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
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
			err: errors.Errorf(errFmtCrossplaneIncompatible, "v0.12.0"),
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
				obj: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
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
				obj: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							Crossplane: &pkgmetav1.CrossplaneConstraints{
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
			obj:    v1alpha1ConfMeta,
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
		"v1": {
			reason: "Should not return error if object is XRD.",
			obj:    v1XRD,
		},
		"v2": {
			reason: "Should not return error if object is XRD.",
			obj:    v2XRD,
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
		"v1": {
			reason: "Should not return error if object is composition.",
			obj:    v1Comp,
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
