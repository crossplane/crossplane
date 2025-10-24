/*
Copyright 2025 The Crossplane Authors.

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

package transaction

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

func TestValidatorChain(t *testing.T) {
	errBoom := errors.New("boom")

	errValidator := func(_ context.Context, _ *v1alpha1.Transaction) error {
		return errBoom
	}
	noError := func(_ context.Context, _ *v1alpha1.Transaction) error {
		return nil
	}

	type args struct {
		validators ValidatorChain
		tx         *v1alpha1.Transaction
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyChain": {
			reason: "Empty validator chain should return no error",
			args: args{
				validators: ValidatorChain{},
				tx:         &v1alpha1.Transaction{},
			},
			want: want{
				err: nil,
			},
		},
		"AllPass": {
			reason: "All validators passing should return no error",
			args: args{
				validators: ValidatorChain{
					ValidatorFunc(noError),
					ValidatorFunc(noError),
				},
				tx: &v1alpha1.Transaction{},
			},
			want: want{
				err: nil,
			},
		},
		"FirstFails": {
			reason: "First validator failing should stop chain and return error",
			args: args{
				validators: ValidatorChain{
					ValidatorFunc(errValidator),
					ValidatorFunc(noError),
				},
				tx: &v1alpha1.Transaction{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SecondFails": {
			reason: "Second validator failing should return error",
			args: args{
				validators: ValidatorChain{
					ValidatorFunc(noError),
					ValidatorFunc(errValidator),
				},
				tx: &v1alpha1.Transaction{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.validators.Validate(context.Background(), tc.args.tx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nValidate(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSchemaValidatorValidate(t *testing.T) {
	errBoom := errors.New("boom")

	providerMeta := `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"provider-test"}}`

	simpleCRD := `{
		"apiVersion":"apiextensions.k8s.io/v1",
		"kind":"CustomResourceDefinition",
		"metadata":{"name":"buckets.s3.aws.crossplane.io"},
		"spec":{
			"group":"s3.aws.crossplane.io",
			"names":{"kind":"Bucket","plural":"buckets"},
			"versions":[{
				"name":"v1",
				"served":true,
				"storage":true,
				"schema":{"openAPIV3Schema":{"type":"object"}}
			}]
		}
	}`

	crdWithField := `{
		"apiVersion":"apiextensions.k8s.io/v1",
		"kind":"CustomResourceDefinition",
		"metadata":{"name":"buckets.s3.aws.crossplane.io"},
		"spec":{
			"group":"s3.aws.crossplane.io",
			"names":{"kind":"Bucket","plural":"buckets"},
			"versions":[{
				"name":"v1",
				"served":true,
				"storage":true,
				"schema":{"openAPIV3Schema":{
					"type":"object",
					"properties":{
						"spec":{
							"type":"object",
							"properties":{"field1":{"type":"string"}}
						}
					}
				}}
			}]
		}
	}`

	crdWithoutField := `{
		"apiVersion":"apiextensions.k8s.io/v1",
		"kind":"CustomResourceDefinition",
		"metadata":{"name":"buckets.s3.aws.crossplane.io"},
		"spec":{
			"group":"s3.aws.crossplane.io",
			"names":{"kind":"Bucket","plural":"buckets"},
			"versions":[{
				"name":"v1",
				"served":true,
				"storage":true,
				"schema":{"openAPIV3Schema":{
					"type":"object",
					"properties":{"spec":{"type":"object"}}
				}}
			}]
		}
	}`

	type args struct {
		kube client.Client
		pkg  xpkg.Client
		tx   *v1alpha1.Transaction
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoProposedPackages": {
			reason: "Should return no error when there are no proposed packages",
			args: args{
				kube: &test.MockClient{},
				pkg:  &MockPackageClient{},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"PackageFetchError": {
			reason: "Should return error when package fetch fails",
			args: args{
				kube: &test.MockClient{},
				pkg: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return nil, errBoom
					},
				},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{
							{Source: "xpkg.io/test/pkg", Version: "v1.0.0"},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NewCRD": {
			reason: "Should return no error for new CRDs that don't exist in cluster",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "buckets.s3.aws.crossplane.io")),
				},
				pkg: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{
							//nolint:contextcheck // Background context is fine for parsing test data.
							Package: NewTestPackage(t, providerMeta, simpleCRD),
						}, nil
					},
				},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{
							{Source: "xpkg.io/test/pkg", Version: "v1.0.0"},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"ClusterGetError": {
			reason: "Should return error when getting existing CRD fails",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
				pkg: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{
							//nolint:contextcheck // Background context is fine for parsing test data.
							Package: NewTestPackage(t, providerMeta, simpleCRD),
						}, nil
					},
				},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{
							{Source: "xpkg.io/test/pkg", Version: "v1.0.0"},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"CompatibleCRD": {
			reason: "Should return no error for compatible CRD changes",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						crd := obj.(*extv1.CustomResourceDefinition)
						crd.SetName("buckets.s3.aws.crossplane.io")
						crd.Spec = extv1.CustomResourceDefinitionSpec{
							Group: "s3.aws.crossplane.io",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Bucket",
								Plural: "buckets",
							},
							Versions: []extv1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &extv1.CustomResourceValidation{
										OpenAPIV3Schema: &extv1.JSONSchemaProps{Type: "object"},
									},
								},
							},
						}
						return nil
					}),
				},
				pkg: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{
							//nolint:contextcheck // Background context is fine for parsing test data.
							Package: NewTestPackage(t, providerMeta, simpleCRD),
						}, nil
					},
				},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{
							{Source: "xpkg.io/test/pkg", Version: "v1.0.0"},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"IncompatibleCRD": {
			reason: "Should return error for incompatible CRD changes (field removal)",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						pkg := NewTestPackage(t, providerMeta, crdWithField)
						existing := pkg.GetObjects()[0].(*extv1.CustomResourceDefinition)
						*obj.(*extv1.CustomResourceDefinition) = *existing
						return nil
					}),
				},
				pkg: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{
							//nolint:contextcheck // Background context is fine for parsing test data.
							Package: NewTestPackage(t, providerMeta, crdWithoutField),
						}, nil
					},
				},
				tx: &v1alpha1.Transaction{
					Status: v1alpha1.TransactionStatus{
						ProposedLockPackages: []v1beta1.LockPackage{
							{Source: "xpkg.io/test/pkg", Version: "v1.0.0"},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewSchemaValidator(tc.args.kube, tc.args.pkg)
			err := v.Validate(context.Background(), tc.args.tx)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nValidate(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

type ValidatorFunc func(context.Context, *v1alpha1.Transaction) error

func (f ValidatorFunc) Validate(ctx context.Context, tx *v1alpha1.Transaction) error {
	return f(ctx, tx)
}

type MockPackageClient struct {
	MockGet          func(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error)
	MockListVersions func(ctx context.Context, source string, opts ...xpkg.GetOption) ([]string, error)
}

func (m *MockPackageClient) Get(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error) {
	return m.MockGet(ctx, ref, opts...)
}

func (m *MockPackageClient) ListVersions(ctx context.Context, source string, opts ...xpkg.GetOption) ([]string, error) {
	return m.MockListVersions(ctx, source, opts...)
}

func NewTestParser(t *testing.T) parser.Parser {
	t.Helper()
	meta, err := xpkg.BuildMetaScheme()
	if err != nil {
		t.Fatalf("failed to build meta scheme: %v", err)
	}
	obj, err := xpkg.BuildObjectScheme()
	if err != nil {
		t.Fatalf("failed to build object scheme: %v", err)
	}
	return parser.New(meta, obj)
}

func NewTestPackage(t *testing.T, metaJSON string, objectsJSON ...string) *parser.Package {
	t.Helper()

	p := NewTestParser(t)

	var allJSON strings.Builder
	allJSON.WriteString("---\n")
	allJSON.WriteString(metaJSON)
	for _, objJSON := range objectsJSON {
		allJSON.WriteString("\n---\n")
		allJSON.WriteString(objJSON)
	}

	pkg, err := p.Parse(context.Background(), io.NopCloser(strings.NewReader(allJSON.String())))
	if err != nil {
		t.Fatalf("failed to parse test package: %v", err)
	}

	return pkg
}
