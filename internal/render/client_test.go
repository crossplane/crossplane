/*
Copyright 2026 The Crossplane Authors.

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

package render

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ client.Client = &InMemoryClient{}

func TestInMemoryClientGet(t *testing.T) {
	type args struct {
		key client.ObjectKey
		gvk schema.GroupVersionKind
	}
	type want struct {
		err error
		obj unstructured.Unstructured
	}

	storedCM := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "my-cm",
			"namespace": "default",
		},
		"data": map[string]any{
			"key": "value",
		},
	}}

	cases := map[string]struct {
		reason string
		store  []unstructured.Unstructured
		args   args
		want   want
	}{
		"ResourceExists": {
			reason: "Get should deep-copy the stored resource into the output object.",
			store:  []unstructured.Unstructured{storedCM},
			args: args{
				key: types.NamespacedName{Namespace: "default", Name: "my-cm"},
				gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			},
			want: want{
				obj: storedCM,
			},
		},
		"ResourceNotFound": {
			reason: "Get should return an error when the resource does not exist in the store.",
			args: args{
				key: types.NamespacedName{Namespace: "default", Name: "missing"},
				gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			},
			want: want{
				err: cmpopts.AnyError,
				// The object retains its GVK but is otherwise empty.
				obj: unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewInMemoryClient(runtime.NewScheme(), tc.store...)

			got := &unstructured.Unstructured{}
			got.SetGroupVersionKind(tc.args.gvk)
			err := c.Get(context.Background(), tc.args.key, got)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.obj, *got); diff != "" {
				t.Errorf("\n%s\nGet(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInMemoryClientPatch(t *testing.T) {
	type args struct {
		obj   unstructured.Unstructured
		patch client.Patch
	}
	type want struct {
		err     error
		applied []unstructured.Unstructured
	}

	cm := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "my-cm", "namespace": "default"},
	}}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SSAApplyPatchRecordsApplied": {
			reason: "An SSA Apply patch should record the resource in the applied list.",
			args: args{
				obj:   cm,
				patch: client.Apply, //nolint:staticcheck // The reconciler still uses client.Patch with Apply.
			},
			want: want{
				applied: []unstructured.Unstructured{cm},
			},
		},
		"MergePatchDoesNotRecordApplied": {
			reason: "A non-SSA patch should not record the resource as applied.",
			args: args{
				obj:   cm,
				patch: client.MergeFrom(&cm),
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewInMemoryClient(runtime.NewScheme())
			obj := tc.args.obj.DeepCopy()
			err := c.Patch(context.Background(), obj, tc.args.patch, client.ForceOwnership)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nPatch(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.applied, c.Applied(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nPatch(...): -want applied, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInMemoryClientDelete(t *testing.T) {
	cm := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "my-cm", "namespace": "default"},
	}}

	type want struct {
		deleted []unstructured.Unstructured
		err     error
	}

	cases := map[string]struct {
		reason string
		store  []unstructured.Unstructured
		obj    unstructured.Unstructured
		want   want
	}{
		"DeleteExistingResource": {
			reason: "Deleting an existing resource should remove it from the store and record the deletion.",
			store:  []unstructured.Unstructured{cm},
			obj:    cm,
			want: want{
				deleted: []unstructured.Unstructured{cm},
			},
		},
		"DeleteMissingResource": {
			reason: "Deleting a non-existent resource should succeed with no deletions recorded.",
			obj: unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "missing", "namespace": "default"},
			}},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewInMemoryClient(runtime.NewScheme(), tc.store...)
			obj := tc.obj.DeepCopy()
			err := c.Delete(context.Background(), obj)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.deleted, c.Deleted(), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nDelete(...): -want deleted, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInMemoryClientList(t *testing.T) {
	cm1 := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm-1", "namespace": "default"},
	}}
	cm2 := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm-2", "namespace": "other"},
	}}
	cmLabeled := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm-labeled", "namespace": "default", "labels": map[string]any{"app": "foo"}},
	}}
	secret := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "sec-1", "namespace": "default"},
	}}

	type want struct {
		items []unstructured.Unstructured
		err   error
	}

	cases := map[string]struct {
		reason  string
		store   []unstructured.Unstructured
		listGVK schema.GroupVersionKind
		opts    []client.ListOption
		want    want
	}{
		"ListByGVK": {
			reason:  "List should return all resources matching the GVK, excluding other kinds.",
			store:   []unstructured.Unstructured{cm1, cm2, secret},
			listGVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			want: want{
				items: []unstructured.Unstructured{cm1, cm2},
			},
		},
		"ListByNamespace": {
			reason:  "List should filter results to the requested namespace.",
			store:   []unstructured.Unstructured{cm1, cm2},
			listGVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			opts:    []client.ListOption{client.InNamespace("default")},
			want: want{
				items: []unstructured.Unstructured{cm1},
			},
		},
		"ListByLabels": {
			reason:  "List should filter results by label selector.",
			store:   []unstructured.Unstructured{cm1, cmLabeled},
			listGVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			opts:    []client.ListOption{client.MatchingLabels{"app": "foo"}},
			want: want{
				items: []unstructured.Unstructured{cmLabeled},
			},
		},
		"ListEmpty": {
			reason:  "Listing a GVK with no stored resources should return an empty list.",
			listGVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			want:    want{},
		},
	}

	sortByName := cmpopts.SortSlices(func(a, b unstructured.Unstructured) bool {
		return a.GetName() < b.GetName()
	})

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewInMemoryClient(runtime.NewScheme(), tc.store...)

			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   tc.listGVK.Group,
				Version: tc.listGVK.Version,
				Kind:    tc.listGVK.Kind + "List",
			})

			err := c.List(context.Background(), list, tc.opts...)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nList(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.items, list.Items, cmpopts.EquateEmpty(), sortByName); diff != "" {
				t.Errorf("\n%s\nList(...): -want items, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestInMemoryClientStatusMerge(t *testing.T) {
	type want struct {
		obj unstructured.Unstructured
		err error
	}

	cases := map[string]struct {
		reason    string
		store     []unstructured.Unstructured
		statusObj unstructured.Unstructured
		want      want
	}{
		"MergesStatusIntoStoredSpec": {
			reason: "Status().Update() should replace the status on the stored object while preserving its spec.",
			store: []unstructured.Unstructured{{Object: map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "XR",
				"metadata":   map[string]any{"name": "my-xr"},
				"spec":       map[string]any{"region": "us-east-1"},
			}}},
			statusObj: unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "XR",
				"metadata":   map[string]any{"name": "my-xr"},
				"status":     map[string]any{"phase": "ready"},
			}},
			want: want{
				obj: unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "XR",
					"metadata":   map[string]any{"name": "my-xr"},
					"spec":       map[string]any{"region": "us-east-1"},
					"status":     map[string]any{"phase": "ready"},
				}},
			},
		},
		"NoStoredVersion": {
			reason: "When there is no stored version, the incoming object should be returned as-is.",
			statusObj: unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "XR",
				"metadata":   map[string]any{"name": "new-xr"},
				"status":     map[string]any{"phase": "creating"},
			}},
			want: want{
				obj: unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "example.org/v1",
					"kind":       "XR",
					"metadata":   map[string]any{"name": "new-xr"},
					"status":     map[string]any{"phase": "creating"},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewInMemoryClient(runtime.NewScheme(), tc.store...)

			obj := tc.statusObj.DeepCopy()
			err := c.Status().Update(context.Background(), obj)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStatus().Update(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.obj, *obj); diff != "" {
				t.Errorf("\n%s\nStatus().Update(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
