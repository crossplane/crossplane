/*
Copyright 2018 The Crossplane Authors.

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
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// AddClaim adds a controller that reconciles RedisCluster resource claims by
// managing Redis resources to the supplied Manager.
func AddClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.ManagedKind(v1alpha1.RedisGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureRedis),
			resource.ManagedConfiguratorFn(resource.ConfigureObjectMeta),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", cachev1alpha1.RedisClusterKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.Redis{}},
		&handler.EnqueueRequestForOwner{OwnerType: &cachev1alpha1.RedisCluster{}, IsController: true},
	); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.RedisGroupVersionKind)
	}

	p := v1alpha1.RedisKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &cachev1alpha1.RedisCluster{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", cachev1alpha1.RedisClusterGroupVersionKind)
}

// ConfigureRedis configures the supplied resource (presumed
// to be a Redis) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureRedis(_ context.Context, cm resource.Claim, cs *corev1alpha1.ResourceClass, mg resource.Managed) error {
	rc, cmok := cm.(*cachev1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), cachev1alpha1.RedisClusterGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.Redis)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.RedisGroupVersionKind)
	}

	spec := v1alpha1.NewRedisSpec(cs.Parameters)
	if err := resolveAzureClassValues(rc); err != nil {
		return errors.Wrap(err, "cannot resolve Azure class instance values")
	}

	spec.WriteConnectionSecretTo = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = cs.ProviderReference
	spec.ReclaimPolicy = cs.ReclaimPolicy

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
