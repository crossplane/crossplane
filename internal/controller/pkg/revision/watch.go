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

package revision

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

// EnqueuePackageRevisionsForImageConfig enqueues a reconcile for all package
// revisions an ImageConfig applies to.
func EnqueuePackageRevisionsForImageConfig(kube client.Client, l v1.PackageRevisionList, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		ic, ok := o.(*v1beta1.ImageConfig)
		if !ok {
			return nil
		}
		// We only care about ImageConfigs that have a pull secret or rewrite rules.
		if !hasPullSecret(ic) && !hasRewriteRules(ic) {
			return nil
		}
		// Enqueue all package revisions matching the prefixes in the ImageConfig.
		rl := l.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Guaranteed to be PackageRevisionList.
		if err := kube.List(ctx, rl); err != nil {
			// Nothing we can do, except logging, if we can't list
			// package revisions.
			log.Debug("Cannot list package revisions while attempting to enqueue from ImageConfig", "error", err)
			return nil
		}

		var matches []reconcile.Request

		for _, rev := range rl.GetRevisions() {
			for _, m := range ic.Spec.MatchImages {
				if strings.HasPrefix(rev.GetSource(), m.Prefix) || strings.HasPrefix(rev.GetResolvedSource(), m.Prefix) {
					log.Debug("Enqueuing for image config",
						"revision-type", fmt.Sprintf("%T", rev),
						"revision-name", rev.GetName(),
						"image-config-name", ic.GetName())
					matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.GetName()}})
				}
			}
		}

		return matches
	})
}

// EnqueuePackageRevisionsForLock enqueues a reconcile for all package
// revisions when the Lock changes.
func EnqueuePackageRevisionsForLock(kube client.Client, l v1.PackageRevisionList, log logging.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		if _, ok := o.(*v1beta1.Lock); !ok {
			return nil
		}

		rl := l.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Guaranteed to be PackageRevisionList.
		if err := kube.List(ctx, rl); err != nil {
			// Nothing we can do, except logging, if we can't list FunctionRevisions.
			log.Debug("Cannot list package revisions while attempting to enqueue from Lock", "error", err)
			return nil
		}

		var matches []reconcile.Request
		for _, rev := range rl.GetRevisions() {
			matches = append(matches, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.GetName()}})
		}

		return matches
	})
}
