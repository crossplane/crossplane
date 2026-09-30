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

package definition

import (
	"context"
	"sync"
	"time"

	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane/v2/internal/circuit"
)

// annotationCompositionResourceName is the annotation that names a composed
// resource the way a function and the ordering graph do.
const annotationCompositionResourceName = "crossplane.io/composition-resource-name"

// dependencyExemptionInterval is how often one composed resource may wake an
// XR through the breaker, once exempt. It matches the breaker's sustained
// rate of one Update event per two seconds.
const dependencyExemptionInterval = 2 * time.Second

// NewPendingDependencyExemption returns a circuit breaker exemption for the
// events composed resource ordering waits on.
//
// Ordering converges one reconcile per wave, and a wave is released by an
// event: a dependency changing, usually becoming ready. That event is the only
// thing that will wake the XR, so if the breaker drops it the next wave waits
// for the half-open probe - thirty seconds by default - and a deep graph that
// would converge in seconds takes many minutes.
//
// An event is exempt when its source is a composed resource that something
// the XR is holding back from creation depends on, according to the XR's
// status.crossplane.pendingResources. That's narrow on purpose. Once the
// dependent is created the source is no longer a dependency of anything
// pending, and its events are metered like any other. And each source may
// wake a given XR through the exemption at most once per
// dependencyExemptionInterval, so a dependency that flaps without ever
// becoming ready can't become an unmetered hot loop.
//
// Legacy XRs don't report pending resources, so nothing about them is exempt.
func NewPendingDependencyExemption(c client.Reader, gvk schema.GroupVersionKind) circuit.Exemption {
	var (
		mu   sync.Mutex
		last = map[string]time.Time{}
	)

	return func(ctx context.Context, obj client.Object, target types.NamespacedName) bool {
		name := obj.GetAnnotations()[annotationCompositionResourceName]
		if name == "" {
			return false
		}

		xr := &kunstructured.Unstructured{}
		xr.SetGroupVersionKind(gvk)
		if err := c.Get(ctx, target, xr); err != nil {
			return false
		}

		if !pendingDependsOn(xr, name) {
			return false
		}

		key := target.String() + "/" + string(obj.GetUID())
		now := time.Now()

		mu.Lock()
		defer mu.Unlock()

		if now.Sub(last[key]) < dependencyExemptionInterval {
			return false
		}
		last[key] = now

		// Forget sources that haven't woken anything for a while, so the map
		// holds only what's currently waking an XR.
		for k, t := range last {
			if now.Sub(t) > time.Minute {
				delete(last, k)
			}
		}

		return true
	}
}

// pendingDependsOn reports whether anything the XR is holding back from
// creation depends on the composed resource with the given composition
// resource name.
func pendingDependsOn(xr *kunstructured.Unstructured, name string) bool {
	pending, _, _ := kunstructured.NestedSlice(xr.Object, "status", "crossplane", "pendingResources")
	for _, p := range pending {
		entry, ok := p.(map[string]any)
		if !ok || entry["operation"] != "Create" {
			continue
		}

		deps, _, _ := kunstructured.NestedSlice(entry, "dependsOn")
		for _, d := range deps {
			dep, ok := d.(map[string]any)
			if !ok {
				continue
			}

			// A dependency on a required resource names a requirement, not a
			// composed resource, so it can't match one.
			if t, _ := dep["type"].(string); t != "" && t != "ComposedResource" {
				continue
			}

			if dep["name"] == name {
				return true
			}
		}
	}

	return false
}
