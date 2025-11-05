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
	testDigest      = "sha256:abc123def456"
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
						Name: xpkg.FriendlyID(testPackageName, testDigest),
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
						Name: xpkg.FriendlyID(testPackageName, testDigest),
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
						Name: xpkg.FriendlyID(testPackageName, testDigest),
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			i := &PackageCreator{
				kube: tc.args.kube,
			}

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp, testVersion)

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
						revList := list.(*v1.ProviderRevisionList)
						oldRev := &v1.ProviderRevision{
							ObjectMeta: metav1.ObjectMeta{
								Name: "old-revision",
							},
						}
						oldRev.SetRevision(1)
						oldRev.SetDesiredState(v1.PackageRevisionActive)
						revList.Items = []v1.ProviderRevision{*oldRev}
						return nil
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
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, testPackageName)),
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

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp, testVersion)

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
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if pr, ok := obj.(*v1.ProviderRevision); ok {
							pr.SetName(xpkg.FriendlyID(testPackageName, testDigest))
							pr.SetObservedTLSServerSecretName(ptr.To("test-server-secret"))
							pr.SetObservedTLSClientSecretName(ptr.To("test-client-secret"))
						}
						return nil
					},
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
						return nil, nil
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
					MockGet: test.NewMockGetFn(nil),
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
					MockGet: test.NewMockGetFn(nil),
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

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp, testVersion)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockEstablisher struct {
	MockEstablish func(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
}

func (m *MockEstablisher) Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error) {
	return m.MockEstablish(ctx, objects, parent, control)
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigest))
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.FunctionRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigest))
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ConfigurationRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigest))
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigest))
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
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
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
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch obj := obj.(type) {
						case *v1.ProviderRevision:
							obj.SetName(xpkg.FriendlyID(testPackageName, testDigest))
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

			err := i.Install(context.Background(), tc.args.tx, tc.args.xp, testVersion)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nInstall(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
