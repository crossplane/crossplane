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

package definition

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	pkgv1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

func TestXRDProtectionReconcilerReconcile(t *testing.T) {
	errBoom := errors.New("boom")

	testGVK := schema.GroupVersionKind{
		Group:   "example.com",
		Version: "v1",
		Kind:    "XDatabase",
	}

	type args struct {
		cached  client.Client
		writer  client.Client
		xrdName string
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
		"ListXRsError": {
			reason: "We should return an error if listing composite resources fails",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(errBoom),
				},
				writer:  &test.MockClient{},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListXRsNoMatchDeletesClusterUsages": {
			reason: "When the CRD doesn't exist (NoMatch), we should delete both ClusterUsages",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
						return &kmeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "example.com", Kind: "XDatabase"}}
					},
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoXRsDeletesClusterUsages": {
			reason: "When no composite resources exist, we should delete both ClusterUsages",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoXRsDeleteClusterUsageNotFound": {
			reason: "When no composite resources exist and ClusterUsages are already gone, we should succeed",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"NoXRsDeleteClusterUsageError": {
			reason: "We should return an error if deleting the ClusterUsage fails",
			args: args{
				cached: &test.MockClient{
					MockList: test.NewMockListFn(nil),
				},
				writer: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(errBoom),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"XRsExistApplyXRDClusterUsageError": {
			reason: "We should return an error if applying the XRD ClusterUsage fails",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(errBoom),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"XRsExistGetXRDError": {
			reason: "We should return an error if we can't get the XRD when XRs exist",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: test.NewMockGetFn(errBoom),
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"XRsExistStandaloneXRDAppliesOnlyXRDClusterUsage": {
			reason: "When XRs exist and XRD has no controller owner, only the XRD ClusterUsage should be applied",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if xrd, ok := obj.(*v1.CompositeResourceDefinition); ok {
							xrd.Name = key.Name
							// No owner references - standalone XRD.
							return nil
						}
						return kerrors.NewNotFound(schema.GroupResource{}, "")
					},
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"XRsExistConfigurationOwnedAppliesBothClusterUsages": {
			reason: "When XRs exist and XRD is owned by a ConfigurationRevision, both ClusterUsages should be applied",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.CompositeResourceDefinition:
							o.Name = key.Name
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ConfigurationRevisionKind,
								Name:       "test-config-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ConfigurationRevision:
							o.Name = key.Name
							o.SetLabels(map[string]string{
								pkgv1.LabelParentPackage: "test-configuration",
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
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"XRsExistConfigurationRevisionNotFound": {
			reason: "We should return an error if the ConfigurationRevision is not found",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.CompositeResourceDefinition:
							o.Name = key.Name
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ConfigurationRevisionKind,
								Name:       "test-config-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ConfigurationRevision:
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
				},
				writer: &test.MockClient{
					MockPatch: test.NewMockPatchFn(nil),
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"XRsExistApplyConfigClusterUsageError": {
			reason: "We should return an error if applying the Configuration ClusterUsage fails",
			args: args{
				cached: &test.MockClient{
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						u := list.(*kunstructured.UnstructuredList)
						u.Items = []kunstructured.Unstructured{{}}
						return nil
					},
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.CompositeResourceDefinition:
							o.Name = key.Name
							o.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: pkgv1.SchemeGroupVersion.String(),
								Kind:       pkgv1.ConfigurationRevisionKind,
								Name:       "test-config-revision",
								Controller: ptr.To(true),
							}}
							return nil
						case *pkgv1.ConfigurationRevision:
							o.Name = key.Name
							o.SetLabels(map[string]string{
								pkgv1.LabelParentPackage: "test-configuration",
							})
							return nil
						default:
							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}
					},
				},
				writer: &test.MockClient{
					MockPatch: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						// First patch (XRD ClusterUsage) succeeds,
						// second patch (Config ClusterUsage) fails.
						if cu, ok := obj.(*protectionv1beta1.ClusterUsage); ok {
							if strings.HasPrefix(cu.Name, "config-protection-") {
								return errBoom
							}
						}
						return nil
					},
				},
				xrdName: "test-xrd",
				gvk:     testGVK,
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &XRDProtectionReconciler{
				cached:  tc.args.cached,
				writer:  tc.args.writer,
				xrdName: tc.args.xrdName,
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

func TestXRDClusterUsageName(t *testing.T) {
	type args struct {
		xrdName string
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
			reason: "A short XRD name should produce a deterministic name within Kubernetes limits",
			args: args{
				xrdName: "test-xrd",
			},
			want: want{
				prefix:        "xrd-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
		"LongName": {
			reason: "A long XRD name should produce a deterministic name within Kubernetes limits",
			args: args{
				xrdName: "very-long-composite-resource-definition-name-that-could-potentially-exceed-kubernetes-limits.some-group.example.com",
			},
			want: want{
				prefix:        "xrd-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := XRDClusterUsageName(tc.args.xrdName)

			if len(got) > tc.want.maxLen {
				t.Errorf("\n%s\nXRDClusterUsageName(%q) length = %d, want <= %d", tc.reason, tc.args.xrdName, len(got), tc.want.maxLen)
			}
			if tc.want.deterministic {
				if diff := cmp.Diff(got, XRDClusterUsageName(tc.args.xrdName)); diff != "" {
					t.Errorf("\n%s\nXRDClusterUsageName(%q) is not deterministic: -want, +got:\n%s", tc.reason, tc.args.xrdName, diff)
				}
			}
			if !strings.HasPrefix(got, tc.want.prefix) {
				t.Errorf("\n%s\nXRDClusterUsageName(%q) = %q, want prefix %q", tc.reason, tc.args.xrdName, got, tc.want.prefix)
			}
		})
	}
}

func TestConfigClusterUsageName(t *testing.T) {
	type args struct {
		xrdName string
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
			reason: "A short XRD name should produce a deterministic config protection name within Kubernetes limits",
			args: args{
				xrdName: "test-xrd",
			},
			want: want{
				prefix:        "config-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
		"LongName": {
			reason: "A long XRD name should produce a deterministic config protection name within Kubernetes limits",
			args: args{
				xrdName: "very-long-composite-resource-definition-name-that-could-potentially-exceed-kubernetes-limits.some-group.example.com",
			},
			want: want{
				prefix:        "config-protection-",
				deterministic: true,
				maxLen:        253,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConfigClusterUsageName(tc.args.xrdName)

			if len(got) > tc.want.maxLen {
				t.Errorf("\n%s\nConfigClusterUsageName(%q) length = %d, want <= %d", tc.reason, tc.args.xrdName, len(got), tc.want.maxLen)
			}
			if tc.want.deterministic {
				if diff := cmp.Diff(got, ConfigClusterUsageName(tc.args.xrdName)); diff != "" {
					t.Errorf("\n%s\nConfigClusterUsageName(%q) is not deterministic: -want, +got:\n%s", tc.reason, tc.args.xrdName, diff)
				}
			}
			if !strings.HasPrefix(got, tc.want.prefix) {
				t.Errorf("\n%s\nConfigClusterUsageName(%q) = %q, want prefix %q", tc.reason, tc.args.xrdName, got, tc.want.prefix)
			}
		})
	}
}

func TestXRDProtectionControllerName(t *testing.T) {
	type args struct {
		xrdName string
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
			reason: "The controller name should be prefixed with xrd-protection/",
			args: args{
				xrdName: "test-xrd",
			},
			want: want{
				name: "xrd-protection/test-xrd",
			},
		},
		"FullyQualifiedName": {
			reason: "The controller name should include the full XRD name",
			args: args{
				xrdName: "xdatabases.example.com",
			},
			want: want{
				name: "xrd-protection/xdatabases.example.com",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := XRDProtectionControllerName(tc.args.xrdName)
			if diff := cmp.Diff(tc.want.name, got); diff != "" {
				t.Errorf("\n%s\nXRDProtectionControllerName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolveConfigurationName(t *testing.T) {
	errBoom := errors.New("boom")

	cases := map[string]struct {
		reason string
		c      client.Client
		xrd    *v1.CompositeResourceDefinition
		want   string
		err    error
	}{
		"NoControllerOwner": {
			reason: "An XRD with no controller owner should return empty string (standalone XRD)",
			c:      &test.MockClient{},
			xrd:    &v1.CompositeResourceDefinition{},
			want:   "",
		},
		"NonConfigurationRevisionOwner": {
			reason: "An XRD owned by a non-ConfigurationRevision should return empty string",
			c:      &test.MockClient{},
			xrd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "SomeOtherKind",
						Name:       "test-owner",
						Controller: ptr.To(true),
					}},
				},
			},
			want: "",
		},
		"GetConfigurationRevisionError": {
			reason: "An error getting the ConfigurationRevision should be returned",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(errBoom),
			},
			xrd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       pkgv1.ConfigurationRevisionKind,
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			err: cmpopts.AnyError,
		},
		"NoParentPackageLabel": {
			reason: "A ConfigurationRevision without the parent package label should return an error",
			c: &test.MockClient{
				MockGet: test.NewMockGetFn(nil),
			},
			xrd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       pkgv1.ConfigurationRevisionKind,
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			err: cmpopts.AnyError,
		},
		"Success": {
			reason: "We should return the configuration name from the ConfigurationRevision label",
			c: &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					if rev, ok := obj.(*pkgv1.ConfigurationRevision); ok {
						rev.SetLabels(map[string]string{
							pkgv1.LabelParentPackage: "my-configuration",
						})
					}
					return nil
				},
			},
			xrd: &v1.CompositeResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       pkgv1.ConfigurationRevisionKind,
						Name:       "test-rev",
						Controller: ptr.To(true),
					}},
				},
			},
			want: "my-configuration",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := resolveConfigurationName(context.Background(), tc.c, tc.xrd)

			if diff := cmp.Diff(tc.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nresolveConfigurationName(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("\n%s\nresolveConfigurationName(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildXRDClusterUsage(t *testing.T) {
	type args struct {
		xrdName           string
		xrTypeDescription string
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
				xrdName:           "test-xrd",
				xrTypeDescription: "XDatabase.example.com",
			},
			want: want{
				cu: &protectionv1beta1.ClusterUsage{
					TypeMeta: metav1.TypeMeta{
						APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
						Kind:       protectionv1beta1.ClusterUsageKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: XRDClusterUsageName("test-xrd"),
						Labels: map[string]string{
							"crossplane.io/xrd-protection":    "true",
							"apiextensions.crossplane.io/xrd": "test-xrd",
						},
					},
					Spec: protectionv1beta1.ClusterUsageSpec{
						Of: protectionv1beta1.Resource{
							APIVersion: v1.SchemeGroupVersion.String(),
							Kind:       v1.CompositeResourceDefinitionKind,
							ResourceRef: &protectionv1beta1.ResourceRef{
								Name: "test-xrd",
							},
						},
						Reason: ptr.To("CompositeResourceDefinition has active composite resources of type XDatabase.example.com"),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := buildXRDClusterUsage(tc.args.xrdName, tc.args.xrTypeDescription)
			if diff := cmp.Diff(tc.want.cu, got); diff != "" {
				t.Errorf("\n%s\nbuildXRDClusterUsage(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildConfigClusterUsage(t *testing.T) {
	type args struct {
		xrdName           string
		configName        string
		xrTypeDescription string
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
			reason: "A Configuration ClusterUsage should be built with the correct metadata, labels, and spec",
			args: args{
				xrdName:           "test-xrd",
				configName:        "my-configuration",
				xrTypeDescription: "XDatabase.example.com",
			},
			want: want{
				cu: &protectionv1beta1.ClusterUsage{
					TypeMeta: metav1.TypeMeta{
						APIVersion: protectionv1beta1.SchemeGroupVersion.String(),
						Kind:       protectionv1beta1.ClusterUsageKind,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: ConfigClusterUsageName("test-xrd"),
						Labels: map[string]string{
							"crossplane.io/configuration-protection": "true",
							pkgv1.LabelParentPackage:                 "my-configuration",
							"apiextensions.crossplane.io/xrd":        "test-xrd",
						},
					},
					Spec: protectionv1beta1.ClusterUsageSpec{
						Of: protectionv1beta1.Resource{
							APIVersion: pkgv1.SchemeGroupVersion.String(),
							Kind:       pkgv1.ConfigurationKind,
							ResourceRef: &protectionv1beta1.ResourceRef{
								Name: "my-configuration",
							},
						},
						Reason: ptr.To("Configuration has active composite resources of type XDatabase.example.com"),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := buildConfigClusterUsage(tc.args.xrdName, tc.args.configName, tc.args.xrTypeDescription)
			if diff := cmp.Diff(tc.want.cu, got); diff != "" {
				t.Errorf("\n%s\nbuildConfigClusterUsage(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestXRDResourceMapFunc(t *testing.T) {
	type args struct {
		xrdName string
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
			reason: "XRDResourceMapFunc should return a single reconcile request for the XRD name",
			args: args{
				xrdName: "test-xrd",
			},
			want: want{
				reqs: []reconcile.Request{{
					NamespacedName: types.NamespacedName{Name: "test-xrd"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := XRDResourceMapFunc(tc.args.xrdName)
			got := fn(context.Background(), nil)
			if diff := cmp.Diff(tc.want.reqs, got); diff != "" {
				t.Errorf("\n%s\nXRDResourceMapFunc(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
