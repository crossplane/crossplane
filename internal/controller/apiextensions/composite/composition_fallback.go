/*
Copyright 2022 The Crossplane Authors.

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

package composite

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
)

// Error strings.
const (
	errTriggerFn = "cannot determine which Composer to use"
)

// A TriggerFn lets a FallBackComposer know when it should fall back from the
// preferred to the fallback Composer.
type TriggerFn func(ctx context.Context, xr resource.Composite, req CompositionRequest) (bool, error)

// A FallBackComposer wraps two Composers - one preferred and one fallback. If
// the trigger FallBackFn returns true it will use the fallback rather than the
// preferred Composer.
type FallBackComposer struct {
	preferred Composer
	fallback  Composer
	trigger   TriggerFn
}

// NewFallBackComposer returns a Composer that calls the preferred Composer
// unless the supplied TriggerFn triggers a fallback to the fallback Composer.
func NewFallBackComposer(preferred, fallback Composer, fn TriggerFn) *FallBackComposer {
	return &FallBackComposer{
		preferred: preferred,
		fallback:  fallback,
		trigger:   fn,
	}
}

// Compose calls either the preferred or fallback Composer's Compose method
// depending on the result of the TriggerFn.
func (c *FallBackComposer) Compose(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
	fallback, err := c.trigger(ctx, xr, req)
	if err != nil {
		return CompositionResult{}, errors.Wrap(err, errTriggerFn)
	}
	if fallback {
		return c.fallback.Compose(ctx, xr, req)
	}

	return c.preferred.Compose(ctx, xr, req)
}

// FallBackForAnonymousTemplates returns a TriggerFn that triggers a fallback if
// the supplied Composition uses anonymous templates, or the supplied XR
// references composed resources that appear to have been created by an
// anonymous template. We use this to fall back from the PTFComposer to the
// PTComposer when Composition Functions are enabled.
//
// The PTFComposer does not support anonymous templates; it requires named
// resources to map from P&T resources to Composition Function resources. A
// validator ensures that its not possible to create a Composition that uses
// both Composition Functions and anonymous resource templates.
func FallBackForAnonymousTemplates(c client.Reader) TriggerFn {
	return func(ctx context.Context, xr resource.Composite, req CompositionRequest) (bool, error) {
		// Fall back if any templates are unnamed.
		for _, t := range req.Revision.Spec.Resources {
			if t.Name == nil {
				return true, nil
			}
		}

		for _, ref := range xr.GetResourceReferences() {
			r := composed.New(composed.FromReference(ref))
			if err := c.Get(ctx, types.NamespacedName{Name: r.GetName()}, r); err != nil {
				return false, errors.Wrap(resource.IgnoreNotFound(err), errGetComposed)
			}

			if GetCompositionResourceName(r) == "" {
				return true, nil
			}
		}

		return false, nil
	}
}
