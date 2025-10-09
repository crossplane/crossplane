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

package manager

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

// hasPullSecret returns true if the ImageConfig has authentication with a pull secret.
func hasPullSecret(ic *v1beta1.ImageConfig) bool {
	if ic.Spec.Registry == nil {
		return false
	}
	if ic.Spec.Registry.Authentication == nil {
		return false
	}
	return ic.Spec.Registry.Authentication.PullSecretRef.Name != ""
}

// hasRewriteRules returns true if the ImageConfig has image rewrite rules.
func hasRewriteRules(ic *v1beta1.ImageConfig) bool {
	return ic.Spec.RewriteImage != nil
}

// EnqueuePackagesForImageConfig enqueues a reconcile for all packages
// an ImageConfig applies to.
func EnqueuePackagesForImageConfig(kube client.Client, l v1.PackageList) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			return nil
		}
		// We only care about ImageConfigs that have a pull secret or rewrite rules.
		if !hasPullSecret(ic) && !hasRewriteRules(ic) {
			return nil
		}
		// Enqueue all packages matching the prefixes in the ImageConfig.
		pl := l.DeepCopyObject().(v1.PackageList) //nolint:forcetypeassert // Guaranteed to be PackageList.
		if err := kube.List(ctx, pl); err != nil {
			// Nothing we can do if we can't list packages.
			return nil
		}

		var matches []reconcile.Request

		for _, pkg := range pl.GetPackages() {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(pkg.GetSource(), m.Prefix) || strings.HasPrefix(pkg.GetResolvedSource(), m.Prefix) {
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: pkg.GetName()}})
				}
			}
		}

		return matches
	})
}

// EnqueuePackageForTransaction enqueues a reconcile for the package that owns a Transaction.
func EnqueuePackageForTransaction(_ client.Client, _ v1.PackageList) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []reconcile.Request {
		tx, ok := o.(*v1alpha1.Transaction)
		if !ok {
			return nil
		}

		// Find the package that owns this Transaction by looking at labels
		packageName := tx.Labels["pkg.crossplane.io/package"]
		if packageName == "" {
			return nil
		}

		// Enqueue the owning package for reconciliation
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: packageName}},
		}
	})
}
