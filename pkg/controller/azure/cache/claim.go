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

package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cachev1alpha1 "github.com/crossplaneio/crossplane/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// RedisClaimController is responsible for adding the Redis
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type RedisClaimController struct{}

// SetupWithManager adds a controller that reconciles RedisCluster resource claims.
func (c *RedisClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.ClassKind(corev1alpha1.ResourceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.RedisGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureRedis),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", cachev1alpha1.RedisClusterKind, controllerName))

	p := v1alpha1.RedisKindAPIVersion

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.Redis{}}, &resource.EnqueueRequestForClaim{}).
		For(&cachev1alpha1.RedisCluster{}).
		WithEventFilter(resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p))).
		Complete(r)
}

// ConfigureRedis configures the supplied resource (presumed
// to be a Redis) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureRedis(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	rc, cmok := cm.(*cachev1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), cachev1alpha1.RedisClusterGroupVersionKind)
	}

	rs, csok := cs.(*corev1alpha1.ResourceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), corev1alpha1.ResourceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.Redis)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.RedisGroupVersionKind)
	}

	spec := v1alpha1.NewRedisSpec(rs.Parameters)
	if err := resolveAzureClassValues(rc); err != nil {
		return errors.Wrap(err, "cannot resolve Azure class instance values")
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.ProviderReference
	spec.ReclaimPolicy = rs.ReclaimPolicy

	i.Spec = *spec

	return nil
}

func resolveAzureClassValues(rc *cachev1alpha1.RedisCluster) error {
	// EngineVersion is currently the only option we expose at the claim level,
	// and Azure only supports Redis 3.2.
	if rc.Spec.EngineVersion != "" && rc.Spec.EngineVersion != v1alpha1.SupportedRedisVersion {
		return errors.Errorf("Azure supports only Redis version %s", v1alpha1.SupportedRedisVersion)
	}
	return nil
}
