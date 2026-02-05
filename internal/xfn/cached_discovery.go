/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package xfn

import (
	"context"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	kcache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

// A CachedOpenAPIClient wraps a CachedDiscoveryInterface to provide OpenAPI v3
// schema access with cache invalidation support.
type CachedOpenAPIClient struct {
	dc discovery.CachedDiscoveryInterface
}

// NewCachedOpenAPIClient returns a new CachedOpenAPIClient wrapping the
// supplied CachedDiscoveryInterface.
func NewCachedOpenAPIClient(dc discovery.CachedDiscoveryInterface) *CachedOpenAPIClient {
	return &CachedOpenAPIClient{dc: dc}
}

// Paths returns the available OpenAPI v3 schema paths.
func (c *CachedOpenAPIClient) Paths() (map[string]openapi.GroupVersion, error) {
	return c.dc.OpenAPIV3().Paths()
}

// InvalidateOnCRDChanges sets up a handler to invalidate the discovery cache
// when CRDs are created, updated, or deleted. This ensures that schema requests
// return up-to-date OpenAPI schemas. The handler runs until the supplied
// context is cancelled.
func (c *CachedOpenAPIClient) InvalidateOnCRDChanges(ctx context.Context, ca cache.Cache) error {
	i, err := ca.GetInformer(ctx, &extv1.CustomResourceDefinition{})
	if err != nil {
		return errors.Wrap(err, "cannot get informer for CustomResourceDefinitions")
	}

	h, err := i.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ any) { c.dc.Invalidate() },
		UpdateFunc: func(_, _ any) { c.dc.Invalidate() },
		DeleteFunc: func(_ any) { c.dc.Invalidate() },
	})
	if err != nil {
		return errors.Wrap(err, "cannot add event handler to CustomResourceDefinition informer")
	}

	go func() {
		<-ctx.Done()
		_ = i.RemoveEventHandler(h)
	}()

	return nil
}
