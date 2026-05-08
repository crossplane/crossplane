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

package managed

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

func TestProtectionReconcilerReconcile(t *testing.T) {
	errBoom := errors.New("boom")

	testGVK := schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1",
		Kind:    "Database",
	}

	type args struct {
		cached  client.Client
		writer  client.Client
		mrdName string
		gvk     schema.GroupVersionKind
	}
	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ListMRsError": {
			reason: "We should return an error if listing managed resources fails",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
				writer:  &test.MockClient{},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListMRsNoMatchDeletesClusterUsage": {
			reason: "When the CRD doesn't exist (NoMatch), we should delete the ClusterUsage",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return &kmeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "example.com", Kind: "Database"}}
					},
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoMRsDeletesClusterUsage": {
			reason: "When no managed resources exist, we should delete the ClusterUsage",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoMRsDeleteClusterUsageNotFound": {
			reason: "When no managed resources exist and ClusterUsage is already gone, we should succeed",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoMRsDeleteClusterUsageError": {
			reason: "We should return an error if deleting the ClusterUsage fails",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(errBoom),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRsExistGetMRDError": {
			reason: "We should return an error if we can't get the MRD when MRs exist",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: test.NewMockGetFn(errBoom),
				},
				writer:  &test.MockClient{},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRsExistNoControllerOwner": {
			reason: "When MRD has no controller owner, we should return an error",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if mrd, ok := obj.(*v1alpha1.ManagedResourceDefinition); ok {
							mrd.Name = key.Name
							mrd.Spec.Group = "example.com"
							mrd.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: "Database"}
							// No owner references
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
				},
				writer:  &test.MockClient{},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRsExistProviderRevisionNotFound": {
			reason: "When ProviderRevision is not found, we should return an error",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							o.Name = key.Name
							o.Spec.Group = "example.com"
							o.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: "Database"}
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ProviderRevisionKind,
								Name:       "test-provider-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ProviderRevision:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
				},
				writer:  &test.MockClient{},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MRsExistClusterUsageApplied": {
			reason: "When MRs exist and Provider can be resolved, we should apply a ClusterUsage",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							o.Name = key.Name
							o.Spec.Group = "example.com"
							o.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: "Database"}
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ProviderRevisionKind,
								Name:       "test-provider-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ProviderRevision:
							o.Name = key.Name
							o.SetLabels(map[string]string{
								pkgv1.LabelParentPackage: "test-provider",
							})
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"MRsExistClusterUsageApplyError": {
			reason: "We should return an error if applying the ClusterUsage fails",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.ManagedResourceDefinition:
							o.Name = key.Name
							o.Spec.Group = "example.com"
							o.Spec.Names = extv1.CustomResourceDefinitionNames{Kind: "Database"}
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ProviderRevisionKind,
								Name:       "test-provider-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ProviderRevision:
							o.Name = key.Name
							o.SetLabels(map[string]string{
								pkgv1.LabelParentPackage: "test-provider",
							})
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(errBoom),
				},
				mrdName: "test-mrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &ProtectionReconciler{
				cached:  tc.args.cached,
				writer:  tc.args.writer,
				mrdName: tc.args.mrdName,
				gvk:     tc.args.gvk,
				log:     logging.NewNopLogger(),
			}

			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestClusterUsageName(t *testing.T) {
	type args struct {
		mrdName string
	}
	type want struct {
		prefix        string
		deterministic bool
		maxLen        int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ShortName": {
			reason: "A short MRD name should produce a deterministic name within Kubernetes limits",
			args: args{
				mrdName: "test-mrd",
			},
			want: want{
				prefix:        "provider-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
		"LongName": {
			reason: "A long MRD name should produce a deterministic name within Kubernetes limits",
			args: args{
				mrdName: "very-long-managed-resource-definition-name-that-could-potentially-exceed-kubernetes-limits.some-group.example.com",
			},
			want: want{
				prefix:        "provider-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ClusterUsageName(tc.args.mrdName)

			if len(got) > tc.want.maxLen {
				t.Errorf("\n%s\nClusterUsageName(%q) length = %d, want <= %d", tc.reason, tc.args.mrdName, len(got), tc.want.maxLen)
			}
			if tc.want.deterministic {
				if diff := cmp.Diff(got, ClusterUsageName(tc.args.mrdName)); diff != "" {
					t.Errorf("\n%s\nClusterUsageName(%q) is not deterministic: -want, +got:\n%s", tc.reason, tc.args.mrdName, diff)
				}
			}
			if !strings.HasPrefix(got, tc.want.prefix) {
				t.Errorf("\n%s\nClusterUsageName(%q) = %q, want prefix %q", tc.reason, tc.args.mrdName, got, tc.want.prefix)
			}
		})
	}
}

func TestProtectionControllerName(t *testing.T) {
	type args struct {
		mrdName string
	}
	type want struct {
		name string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"StandardName": {
			reason: "The controller name should be prefixed with protection/",
			args: args{
				mrdName: "test-mrd",
			},
			want: want{
				name: "protection/test-mrd",
			},
		},
		"FullyQualifiedName": {
			reason: "The controller name should include the full MRD name",
			args: args{
				mrdName: "databases.example.com",
			},
			want: want{
				name: "protection/databases.example.com",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ProtectionControllerName(tc.args.mrdName)
			if diff := cmp.Diff(tc.want.name, got); diff != "" {
				t.Errorf("\n%s\nProtectionControllerName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStorageVersion(t *testing.T) {
	cases := map[string]struct {
		mrd  *v1alpha1.ManagedResourceDefinition
		want string
	}{
		"StorageVersionFound": {
			mrd: &v1alpha1.ManagedResourceDefinition{
				Spec: v1alpha1.ManagedResourceDefinitionSpec{
					CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
						Versions: []v1alpha1.CustomResourceDefinitionVersion{
							{Name: "v1alpha1", Storage: false},
							{Name: "v1", Storage: true},
						},
					},
				},
			},
			want: "v1",
		},
		"SingleStorageVersion": {
			mrd: &v1alpha1.ManagedResourceDefinition{
				Spec: v1alpha1.ManagedResourceDefinitionSpec{
					CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
						Versions: []v1alpha1.CustomResourceDefinitionVersion{
							{Name: "v1beta1", Storage: true},
						},
					},
				},
			},
			want: "v1beta1",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := storageVersion(tc.mrd)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("storageVersion(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestResolveProviderName(t *testing.T) {
	errBoom := errors.New("boom")

	cases := map[string]struct {
		reason string
		c      client.Client
		mrd    *v1alpha1.ManagedResourceDefinition
		want   string
		err    error
	}{
		"NoControllerOwner": {
			reason: "An MRD with no controller owner should return an error",
			c:      &test.MockClient{},
			mrd:    &v1alpha1.ManagedResourceDefinition{},
			err:    cmpopts.AnyError,
		},
		"GetProviderRevisionError": {
			reason: "An error getting the ProviderRevision should be returned",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			mrd: &v1alpha1.ManagedResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			err: cmpopts.AnyError,
		},
		"NoParentPackageLabel": {
			reason: "A ProviderRevision without the parent package label should return an error",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			mrd: &v1alpha1.ManagedResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			err: cmpopts.AnyError,
		},
		"Success": {
			reason: "We should return the provider name from the ProviderRevision label",
			c: &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					if rev, ok := obj.(*pkgv1.ProviderRevision); ok {
						rev.SetLabels(map[string]string{
							pkgv1.LabelParentPackage: "my-provider",
						})
					}
					return nil
				},
			},
			mrd: &v1alpha1.ManagedResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			want: "my-provider",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := resolveProviderName(context.Background(), tc.c, tc.mrd)

			if diff := cmp.Diff(tc.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nresolveProviderName(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nresolveProviderName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildClusterUsage(t *testing.T) {
	type args struct {
		mrdName           string
		providerName      string
		mrTypeDescription string
	}
	type want struct {
		cu *protectionv1beta1.ClusterUsage
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"StandardUsage": {
			reason: "A ClusterUsage should be built with the correct metadata, labels, and spec",
			args: args{
				mrdName:           "test-mrd",
				providerName:      "my-provider",
				mrTypeDescription: "Database.example.com",
			},
			want: want{
				cu: &protectionv1beta1.ClusterUsage{
					TypeMeta: metav1.TypeMeta{
						APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
						Kind:       protectionv1beta1.ClusterUsageKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: ClusterUsageName("test-mrd"),
						Labels: map[string]string{
							"crossplane.io/provider-protection": "true",
							pkgv1.LabelParentPackage:            "my-provider",
							"apiextensions.crossplane.io/mrd":   "test-mrd",
						},
					},
					Spec: protectionv1beta1.ClusterUsageSpec{
						Of: protectionv1beta1.Resource{
							APIVersion: pkgv1.SchemeGroupVersion.String(),
							Kind:       pkgv1.ProviderKind,
							ResourceRef: &protectionv1beta1.ResourceRef{
								Name: "my-provider",
							},
						},
						Reason: ptr.To("Provider has active managed resources of type Database.example.com"),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := buildClusterUsage(tc.args.mrdName, tc.args.providerName, tc.args.mrTypeDescription)
			if diff := cmp.Diff(tc.want.cu, got); diff != "" {
				t.Errorf("\n%s\nbuildClusterUsage(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResourceMapFunc(t *testing.T) {
	type args struct {
		mrdName string
	}
	type want struct {
		reqs []reconcile.Request
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EnqueuesFixedRequest": {
			reason: "ResourceMapFunc should return a single reconcile request for the MRD name",
			args: args{
				mrdName: "test-mrd",
			},
			want: want{
				reqs: []reconcile.Request{{
					NamespacedName: types.NamespacedName{Name: "test-mrd"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := ResourceMapFunc(tc.args.mrdName)
			got := fn(context.Background(), nil)
			if diff := cmp.Diff(tc.want.reqs, got); diff != "" {
				t.Errorf("\n%s\nResourceMapFunc(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
