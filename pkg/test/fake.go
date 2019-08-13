/*
Copyright 2019 The Crossplane Authors.

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

// A MockGetFn is used to mock client.Client's Get implementation.
type MockGetFn func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error

// A MockListFn is used to mock client.Client's List implementation.
type MockListFn func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error

// A MockCreateFn is used to mock client.Client's Create implementation.
type MockCreateFn func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error

// A MockDeleteFn is used to mock client.Client's Delete implementation.
type MockDeleteFn func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error

// A MockDeleteAllOfFn is used to mock client.Client's Delete implementation.
type MockDeleteAllOfFn func(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error

// A MockUpdateFn is used to mock client.Client's Update implementation.
type MockUpdateFn func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error

// A MockPatchFn is used to mock client.Client's Patch implementation.
type MockPatchFn func(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error

// A MockStatusUpdateFn is used to mock client.Client's StatusUpdate implementation.
type MockStatusUpdateFn func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error

// A MockStatusPatchFn is used to mock client.Client's StatusUpdate implementation.
type MockStatusPatchFn func(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error

// An ObjectFn operates on the supplied Object. You might use an ObjectFn to
// test or update the contents of an Object.
type ObjectFn func(obj runtime.Object) error

// NewMockGetFn returns a MockGetFn that returns the supplied error.
func NewMockGetFn(err error, ofn ...ObjectFn) MockGetFn {
	return func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockListFn returns a MockListFn that returns the supplied error.
func NewMockListFn(err error, ofn ...ObjectFn) MockListFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.ListOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockCreateFn returns a MockCreateFn that returns the supplied error.
func NewMockCreateFn(err error, ofn ...ObjectFn) MockCreateFn {
	return func(_ context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockDeleteFn returns a MockDeleteFn that returns the supplied error.
func NewMockDeleteFn(err error, ofn ...ObjectFn) MockDeleteFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.DeleteOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockDeleteAllOfFn returns a MockDeleteAllOfFn that returns the supplied error.
func NewMockDeleteAllOfFn(err error, ofn ...ObjectFn) MockDeleteAllOfFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.DeleteAllOfOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockUpdateFn returns a MockUpdateFn that returns the supplied error.
func NewMockUpdateFn(err error, ofn ...ObjectFn) MockUpdateFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockPatchFn returns a MockPatchFn that returns the supplied error.
func NewMockPatchFn(err error, ofn ...ObjectFn) MockPatchFn {
	return func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockStatusUpdateFn returns a MockStatusUpdateFn that returns the supplied error.
func NewMockStatusUpdateFn(err error, ofn ...ObjectFn) MockStatusUpdateFn {
	return func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// NewMockStatusPatchFn returns a MockStatusPatchFn that returns the supplied error.
func NewMockStatusPatchFn(err error, ofn ...ObjectFn) MockStatusPatchFn {
	return func(_ context.Context, obj runtime.Object, _ client.Patch, _ ...client.PatchOption) error {
		for _, fn := range ofn {
			if err := fn(obj); err != nil {
				return err
			}
		}
		return err
	}
}

// MockClient implements controller-runtime's Client interface, allowing each
// method to be overridden for testing. The controller-runtime provides a fake
// client, but it is has surprising side effects (e.g. silently calling
// os.Exit(1)) and does not allow us control over the errors it returns.
type MockClient struct {
	MockGet          MockGetFn
	MockList         MockListFn
	MockCreate       MockCreateFn
	MockDelete       MockDeleteFn
	MockDeleteAllOf  MockDeleteAllOfFn
	MockUpdate       MockUpdateFn
	MockPatch        MockPatchFn
	MockStatusUpdate MockStatusUpdateFn
	MockStatusPatch  MockStatusPatchFn
}

// NewMockClient returns a MockClient that does nothing when its methods are
// called.
func NewMockClient() *MockClient {
	return &MockClient{
		MockGet:          NewMockGetFn(nil),
		MockList:         NewMockListFn(nil),
		MockCreate:       NewMockCreateFn(nil),
		MockDelete:       NewMockDeleteFn(nil),
		MockDeleteAllOf:  NewMockDeleteAllOfFn(nil),
		MockUpdate:       NewMockUpdateFn(nil),
		MockPatch:        NewMockPatchFn(nil),
		MockStatusUpdate: NewMockStatusUpdateFn(nil),
		MockStatusPatch:  NewMockStatusPatchFn(nil),
	}
}

// Get calls MockClient's MockGet function.
func (c *MockClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return c.MockGet(ctx, key, obj)
}

// List calls MockClient's MockList function.
func (c *MockClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	return c.MockList(ctx, list, opts...)
}

// Create calls MockClient's MockCreate function.
func (c *MockClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	return c.MockCreate(ctx, obj, opts...)
}

// Delete calls MockClient's MockDelete function.
func (c *MockClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return c.MockDelete(ctx, obj, opts...)
}

// DeleteAllOf calls MockClient's DeleteAllOf function.
func (c *MockClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return c.MockDeleteAllOf(ctx, obj, opts...)
}

// Update calls MockClient's MockUpdate function.
func (c *MockClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return c.MockUpdate(ctx, obj, opts...)
}

// Patch calls MockClient's MockPatch function.
func (c *MockClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.MockPatch(ctx, obj, patch, opts...)
}

// Status returns status writer for status sub-resource
func (c *MockClient) Status() client.StatusWriter {
	return &MockStatusWriter{
		MockUpdate: c.MockStatusUpdate,
		MockPatch:  c.MockStatusPatch,
	}
}

// MockStatusWriter provides mock functionality for status sub-resource
type MockStatusWriter struct {
	MockUpdate MockStatusUpdateFn
	MockPatch  MockStatusPatchFn
}

// Update status sub-resource
func (m *MockStatusWriter) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return m.MockUpdate(ctx, obj, opts...)
}

// Patch mocks the patch method
func (m *MockStatusWriter) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return m.MockPatch(ctx, obj, patch, opts...)
}
