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

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	cachev1alpha1 "github.com/crossplaneio/crossplane/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/aws/apis/cache/v1alpha1"
)

// ReplicationGroupClaimController is responsible for adding the ReplicationGroup
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type ReplicationGroupClaimController struct{}

// SetupWithManager adds a controller that reconciles RedisCluster resource claims.
func (c *ReplicationGroupClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(cachev1alpha1.RedisClusterGroupVersionKind),
		resource.ClassKind(v1alpha1.ReplicationGroupClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.ReplicationGroupGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIManagedStatusBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIManagedStatusUnbinder(mgr.GetClient())),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureReplicationGroup),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s.%s",
		cachev1alpha1.RedisClusterKind,
		v1alpha1.ReplicationGroupKind,
		v1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.ReplicationGroup{}}, &resource.EnqueueRequestForClaim{}).
		For(&cachev1alpha1.RedisCluster{}).
		WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.ReplicationGroupClassGroupVersionKind)))).
		Complete(r)
}

// ConfigureReplicationGroup configures the supplied resource (presumed
// to be a ReplicationGroup) using the supplied resource claim (presumed
// to be a RedisCluster) and resource class.
func ConfigureReplicationGroup(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	rc, cmok := cm.(*cachev1alpha1.RedisCluster)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), cachev1alpha1.RedisClusterGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha1.ReplicationGroupClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.ReplicationGroupClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.ReplicationGroup)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.ReplicationGroupGroupVersionKind)
	}

	spec := &v1alpha1.ReplicationGroupSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		ReplicationGroupParameters: rs.SpecTemplate.ReplicationGroupParameters,
	}

	if err := resolveAWSClassInstanceValues(spec, rc); err != nil {
		return errors.Wrap(err, "cannot resolve AWS class instance values")
	}

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

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
