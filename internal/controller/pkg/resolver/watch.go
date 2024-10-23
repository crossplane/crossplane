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

package resolver

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
)

// A FilterFn returns true if the supplied object should be filtered.
type FilterFn func(o client.Object) bool

// HasPullSecret returns a FilterFn that filters any object that either isn't an
// ImageConfig, or is an ImageConfig that doesn't reference a pull secret.
func HasPullSecret() FilterFn {
	return func(o client.Object) bool {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			// Not an ImageConfig - filter it.
			return true
		}
		// Filter ImageConfigs that don't have a pull secret.
		return ic.Spec.Registry == nil || ic.Spec.Registry.Authentication == nil || ic.Spec.Registry.Authentication.PullSecretRef.Name == ""
	}
}

// ForName enqueues a request for the named object. It only enqueues a request
// if all supplied filter functions return false.
func ForName(name string, fns ...FilterFn) handler.MapFunc {
	return func(_ context.Context, o client.Object) []reconcile.Request {
		for _, filter := range fns {
			if filter(o) {
				return nil
			}
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: name}}}
	}
}
