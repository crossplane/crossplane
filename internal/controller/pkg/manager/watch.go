/*
Copyright 2021 The Crossplane Authors.

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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
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
func EnqueuePackagesForImageConfig(kube client.Client, l v1.PackageList, log logging.Logger) handler.EventHandler {
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
			// Nothing we can do, except logging, if we can't list packages.
			log.Debug("Cannot list packages while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request

		for _, pkg := range pl.GetPackages() {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(pkg.GetSource(), m.Prefix) || strings.HasPrefix(pkg.GetResolvedSource(), m.Prefix) {
					log.Debug("Enqueuing package for image config",
						"package-type", fmt.Sprintf("%T", pkg),
						"package-name", pkg.GetName(),
						"image-config-name", ic.GetName())
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: pkg.GetName()}})
				}
			}
		}

		return matches
	})
}
