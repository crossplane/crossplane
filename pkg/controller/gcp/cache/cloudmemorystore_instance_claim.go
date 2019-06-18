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

	"github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/core"
)

// AddClaim adds a controller that reconciles RedisCluster resource claims by
// managing CloudMemorystoreInstance resources to the supplied Manager.
func AddClaim(mgr manager.Manager) error {
	r := core.NewReconciler(mgr,
		core.ClaimKind(v1alpha1.RedisClusterGroupVersionKind),
		core.ResourceKind(gcpcachev1alpha1.CloudMemorystoreInstanceGroupVersionKind),
		core.WithResourceConfigurators(
			core.ResourceConfiguratorFn(ConfigureCloudMemorystoreInstance),
			core.ResourceConfiguratorFn(core.ConfigureObjectMeta),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s.%s", gcpcachev1alpha1.CloudMemorystoreInstanceKind, v1alpha1.RedisClusterKind, v1alpha1.Group))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(
		&source.Kind{Type: &gcpcachev1alpha1.CloudMemorystoreInstance{}},
		&handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.RedisCluster{}, IsController: true},
	); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", gcpcachev1alpha1.CloudMemorystoreInstanceGroupVersionKind)
	}

	p := gcpcachev1alpha1.CloudMemorystoreInstanceKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &v1alpha1.RedisCluster{}},
		&handler.EnqueueRequestForObject{},
		core.NewPredicates(core.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", v1alpha1.RedisClusterGroupVersionKind)
}

// ConfigureCloudMemorystoreInstance configures the supplied resource (presumed
// to be a CloudMemorystoreInstance) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureCloudMemorystoreInstance(ctx context.Context, cm core.Claim, cs *corev1alpha1.ResourceClass, rs core.Resource) error {
	rc, cmok := cm.(*v1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be of kind %s", cm.GetName(), v1alpha1.RedisClusterGroupVersionKind)
	}

	i, rsok := rs.(*gcpcachev1alpha1.CloudMemorystoreInstance)
	if !rsok {
		return errors.Errorf("expected resource %s to be of kind %s", rs.GetName(), gcpcachev1alpha1.CloudMemorystoreInstanceGroupVersionKind)
	}

	spec := gcpcachev1alpha1.NewCloudMemorystoreInstanceSpec(cs.Parameters)
	v, err := core.ResolveClassClaimValues(spec.RedisVersion, toGCPFormat(rc.Spec.EngineVersion))
	if err != nil {
		return errors.Wrap(err, "cannot resolve class claim values")
	}
	spec.RedisVersion = v

	spec.WriteConnectionSecretTo = corev1.LocalObjectReference{Name: string(cs.GetUID())}
	spec.ProviderReference = &corev1.ObjectReference{
		APIVersion: gcpv1alpha1.APIVersion,
		Kind:       gcpv1alpha1.ProviderKind,
		Namespace:  cs.GetNamespace(),
		Name:       cs.ProviderReference.Name,
	}

	i.Spec = *spec

	return nil
}

// toGCPFormat transforms a RedisClusterSpec EngineVersion to a
// CloudMemoryStoreInstanceSpec RedisVersion. The former uses major.minor
// (e.g. 3.2). The latter uses REDIS_MAJOR_MINOR (e.g. REDIS_3_2).
func toGCPFormat(version string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("REDIS_%s", strings.Replace(version, ".", "_", -1))
}
