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

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// AddClaim adds a controller that reconciles RedisCluster resource claims by
// managing ReplicationGroup resources to the supplied Manager.
func AddClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.ManagedKind(v1alpha1.ReplicationGroupGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureReplicationGroup),
			resource.ManagedConfiguratorFn(resource.ConfigureObjectMeta),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", cachev1alpha1.RedisClusterKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.ReplicationGroup{}},
		&handler.EnqueueRequestForOwner{OwnerType: &cachev1alpha1.RedisCluster{}, IsController: true},
	); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.ReplicationGroupGroupVersionKind)
	}

	p := v1alpha1.ReplicationGroupKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &cachev1alpha1.RedisCluster{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", cachev1alpha1.RedisClusterGroupVersionKind)
}

// ConfigureReplicationGroup configures the supplied resource (presumed
// to be a ReplicationGroup) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureReplicationGroup(ctx context.Context, cm resource.Claim, cs *corev1alpha1.ResourceClass, mg resource.Managed) error {
	rc, cmok := cm.(*cachev1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), cachev1alpha1.RedisClusterGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.ReplicationGroup)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.ReplicationGroupGroupVersionKind)
	}

	spec := v1alpha1.NewReplicationGroupSpec(cs.Parameters)

	if err := resolveAWSClassInstanceValues(spec, rc); err != nil {
		return errors.Wrap(err, "cannot resolve AWS class instance values")
	}

	spec.WriteConnectionSecretTo = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = cs.ProviderReference
	spec.ReclaimPolicy = cs.ReclaimPolicy

	i.Spec = *spec

	return nil
}

func resolveAWSClassInstanceValues(spec *v1alpha1.ReplicationGroupSpec, rc *cachev1alpha1.RedisCluster) error {
	var err error
	switch {
	case spec.EngineVersion == "" && rc.Spec.EngineVersion == "":
	// Neither the claim nor its class specified a version. Let AWS pick.

	case spec.EngineVersion == "" && rc.Spec.EngineVersion != "":
		// Only the claim specified a version. Use the latest supported patch
		// version for said claim (minor) version.
		spec.EngineVersion, err = latestSupportedPatchVersion(rc.Spec.EngineVersion)

	case spec.EngineVersion != "" && rc.Spec.EngineVersion == "":
		// Only the class specified a version. Use it.

	case !strings.HasPrefix(spec.EngineVersion, rc.Spec.EngineVersion+"."):
		// Both the claim and its class specified a version, but the class
		// version is not a patch of the claim version.
		err = errors.Errorf("class version %s is not a patch of claim version %s", spec.EngineVersion, rc.Spec.EngineVersion)

	default:
		// Both the claim and its class specified a version, and the class
		// version is a patch of the claim version. Use the class version.
	}

	return errors.Wrap(err, "cannot resolve class claim values")
}

func latestSupportedPatchVersion(minorVersion string) (string, error) {
	p := v1alpha1.LatestSupportedPatchVersion[v1alpha1.MinorVersion(minorVersion)]
	if p == v1alpha1.UnsupportedVersion {
		return "", errors.Errorf("minor version %s is not currently supported", minorVersion)
	}
	return string(p), nil
}
