// Package validation provides a fake client for validation purposes.
package validation

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// FakeClient is a fake client for validation purposes.
type FakeClient struct {
	client client.Client
}

// NewFakeClient returns a new fake client.
func NewFakeClient(scheme *runtime.Scheme) client.Client {
	return &FakeClient{
		client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}
}

// Get implements client.Client, handling any panics that may occur.
func (f *FakeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	return f.client.Get(ctx, key, obj, opts...)
}

// List implements client.Client, handling any panics that may occur.
func (f *FakeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	return f.client.List(ctx, list, opts...)
}

// Create implements client.Client, handling any panics that may occur.
func (f *FakeClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while creating %T: %v", obj, r)
		}
	}()
	return f.client.Create(ctx, obj, opts...)
}

// Delete implements client.Client, handling any panics that may occur.
func (f *FakeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	return f.client.Delete(ctx, obj, opts...)
}

// Update implements client.Client, handling any panics that may occur.
func (f *FakeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()

	return f.client.Update(ctx, obj, opts...)
}

// Patch implements client.Client, handling any panics that may occur.
func (f *FakeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	return f.client.Patch(ctx, obj, patch, opts...)
}

// DeleteAllOf implements client.Client, handling any panics that may occur.
func (f *FakeClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic from fake client: %v", r)
		}
	}()
	return f.client.DeleteAllOf(ctx, obj, opts...)
}

// Status implements client.Client, returning a NopSubResourceClient.
func (f *FakeClient) Status() client.SubResourceWriter {
	return &NopSubResourceClient{}
}

// SubResource implements client.Client, returning a NopSubResourceClient.
func (f *FakeClient) SubResource(_ string) client.SubResourceClient {
	return &NopSubResourceClient{}
}

// Scheme implements client.Client, returning the underlying scheme.
func (f *FakeClient) Scheme() *runtime.Scheme {
	return f.client.Scheme()
}

// RESTMapper implements client.Client, returning the underlying RESTMapper.
func (f *FakeClient) RESTMapper() meta.RESTMapper {
	return f.client.RESTMapper()
}

// NopSubResourceClient is a NOP SubResourceClient.
type NopSubResourceClient struct{}

var _ client.SubResourceClient = &NopSubResourceClient{}

// Get is a NOP.
func (n *NopSubResourceClient) Get(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceGetOption) error {
	return nil
}

// Create is a NOP.
func (n *NopSubResourceClient) Create(_ context.Context, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
	return nil
}

// Update is a NOP.
func (n *NopSubResourceClient) Update(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
	return nil
}

// Patch is a NOP.
func (n *NopSubResourceClient) Patch(_ context.Context, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
	return nil
}
