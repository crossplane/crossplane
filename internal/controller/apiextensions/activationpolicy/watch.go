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

package activationpolicy

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// EnqueueActivationPolicyForManagedResourceDefinition enqueues a reconcile for policies that apply to a managed resource definition.
func EnqueueActivationPolicyForManagedResourceDefinition(kube client.Client, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		mrd, ok := o.(*v1alpha1.ManagedResourceDefinition)
		if !ok {
			return nil
		}

		policies := &v1alpha1.ManagedResourceActivationPolicyList{}
		if err := kube.List(ctx, policies); err != nil {
			// Nothing we can do, except logging, if we can't list managed resource activation policies
			log.Debug("Cannot list managed resource activation policies while attempting to enqueue requests", "error", err)
			return nil
		}

		var matches []reconcile.Request

		for _, policy := range policies.Items {
			if policy.Activates(mrd.GetName()) {
				log.Debug("Enqueuing for managed resource definition",
					"mrd-name", mrd.GetName(),
					"policy-name", policy.GetName())
				matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.GetName()}})
			}
		}

		return matches
	})
}
