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

package engine

import (
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// Invalidatable is an interface for RESTMappers that support cache invalidation.
type Invalidatable interface {
	Invalidate() error
}

// InvalidatableRESTMapper is a RESTMapper wrapper that supports cache invalidation
// for specific GroupVersionKinds. This is useful when CRDs are deleted and recreated
// with different properties (like scope changes) and we need to ensure the RESTMapper
// cache reflects the updated CRD definition.
type InvalidatableRESTMapper struct {
	mu     sync.RWMutex
	mapper meta.RESTMapper
	cfg    *rest.Config
	c      *http.Client
}

// NewInvalidatableRESTMapper creates a new InvalidatableRESTMapper that wraps
// a controller-runtime DynamicRESTMapper with cache invalidation capabilities.
func NewInvalidatableRESTMapper(cfg *rest.Config, c *http.Client) (*InvalidatableRESTMapper, error) {
	mapper, err := apiutil.NewDynamicRESTMapper(cfg, c)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create dynamic REST mapper")
	}

	return &InvalidatableRESTMapper{
		mapper: mapper,
		cfg:    cfg,
		c:      c,
	}, nil
}

// Invalidate invalidates the entire cache by recreating the underlying RESTMapper.
// This ensures that any cached scope information or other properties are refreshed
// from the API server.
func (m *InvalidatableRESTMapper) Invalidate() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Recreate the underlying mapper to ensure fresh cache
	mapper, err := apiutil.NewDynamicRESTMapper(m.cfg, m.c)
	if err != nil {
		return errors.Wrap(err, "cannot recreate dynamic REST mapper for cache invalidation")
	}

	m.mapper = mapper
	return nil
}

// The following methods implement meta.RESTMapper by delegating to the underlying mapper.
// We use a read lock for all read operations to allow concurrent access.

// KindFor returns the Kind for the given resource.
func (m *InvalidatableRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.KindFor(resource)
}

// KindsFor returns all Kinds for the given resource.
func (m *InvalidatableRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.KindsFor(resource)
}

// ResourceFor returns the resource for the given input.
func (m *InvalidatableRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.ResourceFor(input)
}

// ResourcesFor returns all resources for the given input.
func (m *InvalidatableRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.ResourcesFor(input)
}

// RESTMapping returns the RESTMapping for the given GroupKind and optional versions.
func (m *InvalidatableRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.RESTMapping(gk, versions...)
}

// RESTMappings returns all RESTMappings for the given GroupKind and optional versions.
func (m *InvalidatableRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.RESTMappings(gk, versions...)
}

// ResourceSingularizer returns the singular form of the given resource.
func (m *InvalidatableRESTMapper) ResourceSingularizer(resource string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mapper.ResourceSingularizer(resource)
}
