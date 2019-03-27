/*
Copyright 2018 The Crossplane Authors.

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

package test

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ client.Client = &MockClient{}

// MockClient implements controller-runtime's Client interface, allowing each
// method to be overridden for testing. The controller-runtime provides a fake
// client, but it is has surprising side effects (e.g. silently calling
// os.Exit(1)) and does not allow us control over the errors it returns.
type MockClient struct {
	MockGet          func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	MockList         func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error
	MockCreate       func(ctx context.Context, obj runtime.Object) error
	MockDelete       func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error
	MockUpdate       func(ctx context.Context, obj runtime.Object) error
	MockStatusUpdate func(ctx context.Context, obj runtime.Object) error
}

// NewMockClient returns a MockClient that does nothing when its methods are
// called.
func NewMockClient() *MockClient {
	return &MockClient{
		//MockStatusClient: &MockStatusClient{
		//	MockUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
		//},
		MockGet:          func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error { return nil },
		MockList:         func(ctx context.Context, opts *client.ListOptions, list runtime.Object) error { return nil },
		MockCreate:       func(ctx context.Context, obj runtime.Object) error { return nil },
		MockDelete:       func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error { return nil },
		MockUpdate:       func(ctx context.Context, obj runtime.Object) error { return nil },
		MockStatusUpdate: func(ctx context.Context, obj runtime.Object) error { return nil },
	}
}

// Get calls MockClient's MockGet function.
func (c *MockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return c.MockGet(ctx, key, obj)
}

// List calls MockClient's MockList function.
func (c *MockClient) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	return c.MockList(ctx, opts, list)
}

// Create calls MockClient's MockCreate function.
func (c *MockClient) Create(ctx context.Context, obj runtime.Object) error {
	return c.MockCreate(ctx, obj)
}

// Delete calls MockClient's MockDelete function.
func (c *MockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error {
	return c.MockDelete(ctx, obj, opts...)
}

// Update calls MockClient's MockUpdate function.
func (c *MockClient) Update(ctx context.Context, obj runtime.Object) error {
	return c.MockUpdate(ctx, obj)
}

// Status returns status writer for status sub-resource
func (c *MockClient) Status() client.StatusWriter {
	return &MockStatusWriter{
		MockUpdate: c.MockStatusUpdate,
	}
}

// MockStatusWriter provides mock functionality for status sub-resource
type MockStatusWriter struct {
	MockUpdate func(ctx context.Context, obj runtime.Object) error
}

// Update status sub-resource
func (m *MockStatusWriter) Update(ctx context.Context, obj runtime.Object) error {
	return m.MockUpdate(ctx, obj)
}
