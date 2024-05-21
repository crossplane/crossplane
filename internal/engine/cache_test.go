/*
Copyright 2024 The Crossplane Authors.

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

package engine

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ cache.Cache = &MockCache{}

type MockCache struct {
	cache.Cache

	MockGet                func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	MockList               func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	MockGetInformer        func(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error)
	MockGetInformerForKind func(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error)
	MockRemoveInformer     func(ctx context.Context, obj client.Object) error
}

func (m *MockCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return m.MockGet(ctx, key, obj, opts...)
}

func (m *MockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.MockList(ctx, list, opts...)
}

func (m *MockCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return m.MockGetInformer(ctx, obj, opts...)
}

func (m *MockCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return m.MockGetInformerForKind(ctx, gvk, opts...)
}

func (m *MockCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	return m.MockRemoveInformer(ctx, obj)
}

func TestActiveInformers(t *testing.T) {
	c := &MockCache{
		MockGet: func(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			return nil
		},
		MockList: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
			return nil
		},
		MockGetInformer: func(_ context.Context, _ client.Object, _ ...cache.InformerGetOption) (cache.Informer, error) {
			return nil, nil
		},
		MockGetInformerForKind: func(_ context.Context, _ schema.GroupVersionKind, _ ...cache.InformerGetOption) (cache.Informer, error) {
			return nil, nil
		},
		MockRemoveInformer: func(_ context.Context, _ client.Object) error { return nil },
	}

	itc := TrackInformers(c, runtime.NewScheme())

	ctx := context.Background()

	// We intentionally call methods twice to cover the code paths where we
	// don't start tracking an informer because we already track it (and vice
	// versa for remove).

	// Get a GVK
	get := &unstructured.Unstructured{}
	get.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.crossplane.io",
		Version: "v1",
		Kind:    "Get",
	})
	_ = itc.Get(ctx, client.ObjectKeyFromObject(get), get)
	_ = itc.Get(ctx, client.ObjectKeyFromObject(get), get)

	// List a GVK
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.crossplane.io",
		Version: "v1",
		Kind:    "ListList", // It's a list list!
	})
	_ = itc.List(ctx, list)
	_ = itc.List(ctx, list)

	// Get an informer
	getinf := &unstructured.Unstructured{}
	getinf.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.crossplane.io",
		Version: "v1",
		Kind:    "GetInformer",
	})
	_, _ = itc.GetInformer(ctx, getinf)
	_, _ = itc.GetInformer(ctx, getinf)

	// Get an informer by GVK
	getgvk := schema.GroupVersionKind{
		Group:   "test.crossplane.io",
		Version: "v1",
		Kind:    "GetInformerForKind",
	}
	_, _ = itc.GetInformerForKind(ctx, getgvk)
	_, _ = itc.GetInformerForKind(ctx, getgvk)

	// Get a GVK, then remove its informer.
	remove := &unstructured.Unstructured{}
	remove.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.crossplane.io",
		Version: "v1",
		Kind:    "RemoveMe",
	})
	_ = itc.Get(ctx, client.ObjectKeyFromObject(remove), remove)
	_ = itc.RemoveInformer(ctx, remove)
	_ = itc.RemoveInformer(ctx, remove)

	want := []schema.GroupVersionKind{
		{
			Group:   "test.crossplane.io",
			Version: "v1",
			Kind:    "Get",
		},
		{
			Group:   "test.crossplane.io",
			Version: "v1",
			Kind:    "List",
		},
		{
			Group:   "test.crossplane.io",
			Version: "v1",
			Kind:    "GetInformer",
		},
		{
			Group:   "test.crossplane.io",
			Version: "v1",
			Kind:    "GetInformerForKind",
		},
	}

	got := itc.ActiveInformers()
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b schema.GroupVersionKind) bool { return a.String() > b.String() })); diff != "" {
		t.Errorf("\nitc.ActiveInformers(...): -want, +got:\n%s", diff)
	}
}
