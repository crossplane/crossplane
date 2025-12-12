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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

const (
	testDigest      = "sha256:abc123def456789012345678901234567890123456789012345678901234abcd"
	testDigestHex   = "abc123def456789012345678901234567890123456789012345678901234abcd" // Hex part without algorithm prefix
	testSource      = "xpkg.io/test/provider-test"
	testPackageName = "test-provider-test" // DNS label from xpkg.ToDNSLabel(testSource repository)
	testVersion     = "v0.1.0"

	testProviderMeta      = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"test"}}`
	testConfigurationMeta = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Configuration","metadata":{"name":"test"}}`
	testFunctionMeta      = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Function","metadata":{"name":"test"}}`
)

func TestNewPackageAndRevision(t *testing.T) {
	type args struct {
		xp *xpkg.Package
	}
	type want struct {
		pkg v1.Package
		rev v1.PackageRevision
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderPackage": {
			reason: "Should create Provider and ProviderRevision",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPackageName,
					},
				},
				rev: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: xpkg.FriendlyID(testPackageName, testDigestHex),
					},
				},
				err: nil,
			},
		},
		"ConfigurationPackage": {
			reason: "Should create Configuration and ConfigurationRevision",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testConfigurationMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkg: &v1.Configuration{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPackageName,
					},
				},
				rev: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: xpkg.FriendlyID(testPackageName, testDigestHex),
					},
				},
				err: nil,
			},
		},
		"FunctionPackage": {
			reason: "Should create Function and FunctionRevision",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testFunctionMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkg: &v1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Name: testPackageName,
					},
				},
				rev: &v1.FunctionRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: xpkg.FriendlyID(testPackageName, testDigestHex),
					},
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkg, rev, err := NewPackageAndRevision(tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nNewPackageAndRevision(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pkg, pkg); diff != "" {
				t.Errorf("%s\nNewPackageAndRevision(...): -want package, +got package:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.rev, rev); diff != "" {
				t.Errorf("%s\nNewPackageAndRevision(...): -want revision, +got revision:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewPackageAndRevisionList(t *testing.T) {
	type args struct {
		xp *xpkg.Package
	}
	type want struct {
		pkgList v1.PackageList
		revList v1.PackageRevisionList
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderLists": {
			reason: "Should create ProviderList and ProviderRevisionList",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkgList: &v1.ProviderList{},
				revList: &v1.ProviderRevisionList{},
				err:     nil,
			},
		},
		"ConfigurationLists": {
			reason: "Should create ConfigurationList and ConfigurationRevisionList",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testConfigurationMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkgList: &v1.ConfigurationList{},
				revList: &v1.ConfigurationRevisionList{},
				err:     nil,
			},
		},
		"FunctionLists": {
			reason: "Should create FunctionList and FunctionRevisionList",
			args: args{
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testFunctionMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				pkgList: &v1.FunctionList{},
				revList: &v1.FunctionRevisionList{},
				err:     nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkgList, revList, err := NewPackageAndRevisionList(tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nNewPackageAndRevisionList(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pkgList, pkgList); diff != "" {
				t.Errorf("%s\nNewPackageAndRevisionList(...): -want package list, +got package list:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.revList, revList); diff != "" {
				t.Errorf("%s\nNewPackageAndRevisionList(...): -want revision list, +got revision list:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInstallPackage(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
		xp   *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should create or update Package successfully",
			args: args{
				kube: &test.MockClient{
					MockList:         test.NewMockListFn(nil),
					MockGet:          test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, testPackageName)),
					MockCreate:       test.NewMockCreateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"ErrorOnCreate": {
			reason: "Should return error when CreateOrUpdate fails",
			args: args{
				kube: &test.MockClient{
					MockList:   test.NewMockListFn(nil),
					MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, testPackageName)),
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"UseExistingPackage": {
			reason: "Should use existing package with matching source repository",
			args: args{
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						// Return an existing package with different name but matching source
						pkgList := list.(*v1.ProviderList)
						existing := &v1.Provider{
							ObjectMeta: metav1.ObjectMeta{
								Name: "custom-provider-name",
							},
						}
						existing.Spec.Package = testSource + ":v1.0.0"
						pkgList.Items = []v1.Provider{*existing}
						return nil
					},
					MockGet:          test.NewMockGetFn(nil),
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &PackageCreator{
				kube: tc.args.kube,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInstallPackageRevision(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
		xp   *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CreateFirstRevision": {
			reason: "Should create first revision with revision number 1",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.Provider:
							obj.SetName(testPackageName)
							obj.Spec.RevisionActivationPolicy = ptr.To(v1.AutomaticActivation)
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						// Return empty list for first revision
						return nil
					},
					MockCreate:       test.NewMockCreateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"DeactivateOldRevisions": {
			reason: "Should deactivate old revisions when creating new one",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.Provider:
							obj.SetName(testPackageName)
							obj.Spec.RevisionActivationPolicy = ptr.To(v1.AutomaticActivation)
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *v1.ProviderList:
							// Return empty package list for FindExistingPackage
							return nil
						case *v1.ProviderRevisionList:
							oldRev := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "old-revision",
								},
							}
							oldRev.SetRevision(1)
							oldRev.SetDesiredState(v1.PackageRevisionActive)
							list.Items = []v1.ProviderRevision{*oldRev}
							return nil
						default:
							return nil
						}
					},
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockCreate:       test.NewMockCreateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"PackageNotFound": {
			reason: "Should return error when package doesn't exist",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, testPackageName)),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListRevisionsError": {
			reason: "Should return error when listing revisions fails",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.Provider:
							obj.SetName(testPackageName)
							return nil
						default:
							return nil
						}
					},
					MockList: test.NewMockListFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &RevisionCreator{
				kube: tc.args.kube,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInstallObjects(t *testing.T) {
	errBoom := errors.New("boom")
	simpleCRD := `{
		"apiVersion":"apiextensions.k8s.io/v1",
		"kind":"CustomResourceDefinition",
		"metadata":{"name":"test.example.com"},
		"spec":{
			"group":"example.com",
			"names":{"kind":"Test","plural":"tests"},
			"versions":[{
				"name":"v1",
				"served":true,
				"storage":true,
				"schema":{"openAPIV3Schema":{"type":"object"}}
			}]
		}
	}`

	type args struct {
		kube        client.Client
		establisher Establisher
		tx          *v1alpha1.Transaction
		xp          *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulEstablish": {
			reason: "Should establish control of package objects",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if pr, ok := obj.(*v1.ProviderRevision); ok {
							pr.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							pr.SetObservedTLSServerSecretName(ptr.To("test-server-secret"))
							pr.SetObservedTLSClientSecretName(ptr.To("test-client-secret"))
						}
						return nil
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				establisher: &MockEstablisher{
					MockEstablish: func(_ context.Context, objs []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
						wantLabels := map[string]string{v1alpha1.LabelTransactionName: "tx-test"}
						for _, obj := range objs {
							if mo, ok := obj.(metav1.Object); ok {
								if diff := cmp.Diff(wantLabels, mo.GetLabels()); diff != "" {
									t.Errorf("object labels: -want, +got:\n%s", diff)
								}
							}
						}
						return []xpv1.TypedReference{
							{
								APIVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Name:       "test.example.com",
							},
						}, nil
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta, simpleCRD),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"EstablishError": {
			reason: "Should return error when establish fails",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(nil),
				},
				establisher: &MockEstablisher{
					MockEstablish: func(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
						return nil, errBoom
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta, simpleCRD),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"NoObjects": {
			reason: "Should succeed when package has no objects",
			args: args{
				kube: &test.MockClient{
					MockList:         test.NewMockListFn(nil),
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				establisher: &MockEstablisher{
					MockEstablish: func(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
						return nil, nil
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &ObjectInstaller{
				kube:    tc.args.kube,
				objects: tc.args.establisher,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockEstablisher struct {
	MockEstablish      func(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
	MockReleaseObjects func(ctx context.Context, parent v1.PackageRevision) error
}

func (m *MockEstablisher) Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error) {
	return m.MockEstablish(ctx, objects, parent, control)
}

func (m *MockEstablisher) ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error {
	return m.MockReleaseObjects(ctx, parent)
}

func TestBootstrapRuntime(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube      client.Client
		namespace string
		tx        *v1alpha1.Transaction
		xp        *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderSuccess": {
			reason: "Should bootstrap provider runtime successfully",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							obj.SetDesiredState(v1.PackageRevisionActive)
							obj.SetTLSServerSecretName(ptr.To("test-server-secret"))
							obj.SetTLSClientSecretName(ptr.To("test-client-secret"))
							obj.SetOwnerReferences([]metav1.OwnerReference{{Name: testPackageName}})
							obj.SetLabels(map[string]string{v1.LabelParentPackage: testPackageName})
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
					MockCreate:       test.NewMockCreateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				namespace: "test-namespace",
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"FunctionSuccess": {
			reason: "Should bootstrap function runtime successfully",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.FunctionRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							obj.SetDesiredState(v1.PackageRevisionActive)
							obj.SetTLSServerSecretName(ptr.To("test-server-secret"))
							obj.SetOwnerReferences([]metav1.OwnerReference{{Name: testPackageName}})
							obj.SetLabels(map[string]string{v1.LabelParentPackage: testPackageName})
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
					MockCreate:       test.NewMockCreateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				namespace: "test-namespace",
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testFunctionMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"ConfigurationNoOp": {
			reason: "Should be no-op for configuration (no runtime)",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ConfigurationRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
				},
				namespace: "test-namespace",
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testConfigurationMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"InactiveRevision": {
			reason: "Should skip bootstrap for inactive revision",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							obj.SetDesiredState(v1.PackageRevisionInactive)
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
				},
				namespace: "test-namespace",
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"RevisionNotFound": {
			reason: "Should return error when revision doesn't exist",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				namespace: "test-namespace",
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"CreateServiceError": {
			reason: "Should return error when service creation fails",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigestHex))
							obj.SetDesiredState(v1.PackageRevisionActive)
							obj.SetTLSServerSecretName(ptr.To("test-server-secret"))
							obj.SetTLSClientSecretName(ptr.To("test-client-secret"))
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						}
					},
					MockCreate: test.NewMockCreateFn(errBoom),
				},
				namespace: "test-namespace",
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &RuntimeBootstrapper{
				kube:      tc.args.kube,
				namespace: tc.args.namespace,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRevisionStatusUpdater(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
		xp   *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should set RevisionHealthy condition successfully",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if pr, ok := obj.(*v1.ProviderRevision); ok {
							pr.SetName(key.Name)
							pr.SetDesiredState(v1.PackageRevisionActive)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: nil,
			},
		},
		"RevisionNotFound": {
			reason: "Should return error when revision doesn't exist",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "test-rev")),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"StatusUpdateError": {
			reason: "Should return error when status update fails",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if pr, ok := obj.(*v1.ProviderRevision); ok {
							pr.SetName(key.Name)
							pr.SetDesiredState(v1.PackageRevisionActive)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := NewRevisionStatusUpdater(tc.args.kube)

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackageStatusUpdater(t *testing.T) {
	errBoom := errors.New("boom")
	testResolvedSource := "registry.io/test/provider-test@sha256:resolved"
	testImageConfigs := []xpkg.ImageConfig{
		{
			Name:   "test-image-config-1",
			Reason: xpkg.ImageConfigReasonRewrite,
		},
		{
			Name:   "test-image-config-2",
			Reason: xpkg.ImageConfigReasonSetPullSecret,
		},
	}

	type args struct {
		kube client.Client
		tx   *v1alpha1.Transaction
		xp   *xpkg.Package
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should set Package status fields successfully",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if pkg, ok := obj.(*v1.Provider); ok {
							pkg.SetName(key.Name)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package:             NewTestPackage(t, testProviderMeta),
					Digest:              testDigest,
					Source:              testSource,
					ResolvedSource:      testResolvedSource,
					AppliedImageConfigs: testImageConfigs,
				},
			},
			want: want{
				err: nil,
			},
		},
		"PackageNotFound": {
			reason: "Should return error when package doesn't exist",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet:  test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, testPackageName)),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"StatusUpdateError": {
			reason: "Should return error when status update fails",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if pkg, ok := obj.(*v1.Provider); ok {
							pkg.SetName(key.Name)
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &PackageStatusUpdater{
				kube: tc.args.kube,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReleaseObjects(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		kube client.Client
		obj  Establisher
		tx   *v1alpha1.Transaction
		xp   *xpkg.Package
	}
	type want struct {
		err          error
		releaseCount int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoExistingPackage": {
			reason: "Should succeed when no existing package found",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				obj: &MockEstablisher{},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err:          nil,
				releaseCount: 0,
			},
		},
		"ReleaseInactiveRevisions": {
			reason: "Should release objects from inactive revisions only",
			args: args{
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *v1.ProviderList:
							existing := &v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name: testPackageName,
								},
							}
							existing.Spec.Package = testSource + ":v1.0.0"
							list.Items = []v1.Provider{*existing}
							return nil
						case *v1.ProviderRevisionList:
							activeRev := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "active-revision",
								},
							}
							activeRev.SetDesiredState(v1.PackageRevisionActive)
							activeRev.SetObjects([]xpv1.TypedReference{{Name: "obj1"}})

							inactiveRev := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "inactive-revision",
								},
							}
							inactiveRev.SetDesiredState(v1.PackageRevisionInactive)
							inactiveRev.SetObjects([]xpv1.TypedReference{{Name: "obj2"}})

							list.Items = []v1.ProviderRevision{*activeRev, *inactiveRev}
							return nil
						default:
							return nil
						}
					},
				},
				obj: &MockEstablisher{
					MockReleaseObjects: func(_ context.Context, rev v1.PackageRevision) error {
						if rev.GetDesiredState() != v1.PackageRevisionInactive {
							t.Errorf("ReleaseObjects called on active revision %s", rev.GetName())
						}
						return nil
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err:          nil,
				releaseCount: 1,
			},
		},
		"ReleaseInactiveRevisionsRegardlessOfObjectRefs": {
			reason: "Should release inactive revisions even if they have no object references, since CRDs may still have controller refs",
			args: args{
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *v1.ProviderList:
							existing := &v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name: testPackageName,
								},
							}
							existing.Spec.Package = testSource + ":v1.0.0"
							list.Items = []v1.Provider{*existing}
							return nil
						case *v1.ProviderRevisionList:
							inactiveRevWithObjs := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "inactive-with-objects",
								},
							}
							inactiveRevWithObjs.SetDesiredState(v1.PackageRevisionInactive)
							inactiveRevWithObjs.SetObjects([]xpv1.TypedReference{{Name: "obj1"}})

							inactiveRevNoObjs := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "inactive-no-objects",
								},
							}
							inactiveRevNoObjs.SetDesiredState(v1.PackageRevisionInactive)

							list.Items = []v1.ProviderRevision{*inactiveRevWithObjs, *inactiveRevNoObjs}
							return nil
						default:
							return nil
						}
					},
				},
				obj: &MockEstablisher{
					MockReleaseObjects: func(_ context.Context, _ v1.PackageRevision) error {
						return nil
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err:          nil,
				releaseCount: 2,
			},
		},
		"ListPackagesError": {
			reason: "Should return error when listing packages fails",
			args: args{
				kube: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
				obj: &MockEstablisher{},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListRevisionsError": {
			reason: "Should return error when listing revisions fails",
			args: args{
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch list.(type) {
						case *v1.ProviderList:
							return nil
						case *v1.ProviderRevisionList:
							return errBoom
						default:
							return nil
						}
					},
				},
				obj: &MockEstablisher{},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ReleaseObjectsError": {
			reason: "Should return error when ReleaseObjects fails",
			args: args{
				kube: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						switch list := list.(type) {
						case *v1.ProviderList:
							existing := &v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name: testPackageName,
								},
							}
							existing.Spec.Package = testSource + ":v1.0.0"
							list.Items = []v1.Provider{*existing}
							return nil
						case *v1.ProviderRevisionList:
							inactiveRev := &v1.ProviderRevision{
								ObjectMeta: metav1.ObjectMeta{
									Name: "inactive-revision",
								},
							}
							inactiveRev.SetDesiredState(v1.PackageRevisionInactive)
							inactiveRev.SetObjects([]xpv1.TypedReference{{Name: "obj1"}})
							list.Items = []v1.ProviderRevision{*inactiveRev}
							return nil
						default:
							return nil
						}
					},
				},
				obj: &MockEstablisher{
					MockReleaseObjects: func(_ context.Context, _ v1.PackageRevision) error {
						return errBoom
					},
				},
				tx: &v1alpha1.Transaction{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tx-test",
					},
				},
				xp: &xpkg.Package{
					Package: NewTestPackage(t, testProviderMeta),
					Digest:  testDigest,
					Source:  testSource,
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			releaseCount := 0
			if tc.args.obj != nil {
				if me, ok := tc.args.obj.(*MockEstablisher); ok && me.MockReleaseObjects != nil {
					origRelease := me.MockReleaseObjects
					me.MockReleaseObjects = func(ctx context.Context, rev v1.PackageRevision) error {
						releaseCount++
						return origRelease(ctx, rev)
					}
				}
			}

			i := NewObjectReleaser(tc.args.kube, tc.args.obj)
			err := i.Install(context.Background(), tc.args.tx, tc.args.xp)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if tc.want.err == nil && releaseCount != tc.want.releaseCount {
				t.Errorf("%s\nInstall(...): want %d releases, got %d", tc.reason, tc.want.releaseCount, releaseCount)
			}
		})
	}
}

func TestAsImageConfigRefs(t *testing.T) {
	type args struct {
		configs []xpkg.ImageConfig
	}
	type want struct {
		refs []v1.ImageConfigRef
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilConfigs": {
			reason: "Should return nil for nil input",
			args: args{
				configs: nil,
			},
			want: want{
				refs: nil,
			},
		},
		"EmptyConfigs": {
			reason: "Should return nil for empty input",
			args: args{
				configs: []xpkg.ImageConfig{},
			},
			want: want{
				refs: nil,
			},
		},
		"SingleConfig": {
			reason: "Should convert single config correctly",
			args: args{
				configs: []xpkg.ImageConfig{
					{
						Name:   "test-config",
						Reason: xpkg.ImageConfigReasonRewrite,
					},
				},
			},
			want: want{
				refs: []v1.ImageConfigRef{
					{
						Name:   "test-config",
						Reason: v1.ImageConfigReasonRewrite,
					},
				},
			},
		},
		"MultipleConfigs": {
			reason: "Should convert multiple configs correctly",
			args: args{
				configs: []xpkg.ImageConfig{
					{
						Name:   "config-1",
						Reason: xpkg.ImageConfigReasonRewrite,
					},
					{
						Name:   "config-2",
						Reason: xpkg.ImageConfigReasonSetPullSecret,
					},
				},
			},
			want: want{
				refs: []v1.ImageConfigRef{
					{
						Name:   "config-1",
						Reason: v1.ImageConfigReasonRewrite,
					},
					{
						Name:   "config-2",
						Reason: v1.ImageConfigReasonSetPullSecret,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			refs := AsImageConfigRefs(tc.args.configs)

			if diff := cmp.Diff(tc.want.refs, refs); diff != "" {
				t.Errorf("%s\nAsImageConfigRefs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
