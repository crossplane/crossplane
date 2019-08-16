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

package compute

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// GKEClusterClaimController is responsible for adding the GKECluster
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type GKEClusterClaimController struct{}

// SetupWithManager adds a controller that reconciles KubernetesCluster resource claims.
func (c *GKEClusterClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(computev1alpha1.KubernetesClusterGroupVersionKind),
		resource.ClassKind(corev1alpha1.ResourceClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.GKEClusterGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureGKECluster),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", computev1alpha1.KubernetesClusterKind, controllerName))

	p := v1alpha1.GKEClusterKindAPIVersion

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.GKECluster{}}, &resource.EnqueueRequestForClaim{}).
		For(&computev1alpha1.KubernetesCluster{}).
		WithEventFilter(resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p))).
		Complete(r)
}

// ConfigureGKECluster configures the supplied resource (presumed to be a
// GKECluster) using the supplied resource claim (presumed to be a
// KubernetesCluster) and resource class.
func ConfigureGKECluster(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	if _, cmok := cm.(*computev1alpha1.KubernetesCluster); !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), computev1alpha1.KubernetesClusterGroupVersionKind)
	}

	rs, csok := cs.(*corev1alpha1.ResourceClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), corev1alpha1.ResourceClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.GKECluster)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.GKEClusterGroupVersionKind)
	}

	spec := v1alpha1.ParseClusterSpec(rs.Parameters)
	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.ProviderReference
	spec.ReclaimPolicy = rs.ReclaimPolicy

	i.Spec = *spec

	return nil
}
