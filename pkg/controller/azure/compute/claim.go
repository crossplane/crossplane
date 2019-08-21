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

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	computev1alpha1 "github.com/crossplaneio/crossplane/apis/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/compute/v1alpha1"
)

// AKSClusterClaimController is responsible for adding the AKSCluster
// claim controller and its corresponding reconciler to the manager with any runtime configuration.
type AKSClusterClaimController struct{}

// SetupWithManager adds a controller that reconciles KubernetesCluster resource claims.
func (c *AKSClusterClaimController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(computev1alpha1.KubernetesClusterGroupVersionKind),
		resource.ClassKind(v1alpha1.AKSClusterClassGroupVersionKind),
		resource.ManagedKind(v1alpha1.AKSClusterGroupVersionKind),
		resource.WithManagedConfigurators(
			resource.ManagedConfiguratorFn(ConfigureAKSCluster),
			resource.NewObjectMetaConfigurator(mgr.GetScheme()),
		))

	name := strings.ToLower(fmt.Sprintf("%s.%s", computev1alpha1.KubernetesClusterKind, controllerName))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		Watches(&source.Kind{Type: &v1alpha1.AKSCluster{}}, &resource.EnqueueRequestForClaim{}).
		For(&computev1alpha1.KubernetesCluster{}).
		WithEventFilter(resource.NewPredicates(resource.HasClassReferenceKind(resource.ClassKind(v1alpha1.AKSClusterClassGroupVersionKind)))).
		Complete(r)
}

// ConfigureAKSCluster configures the supplied resource (presumed to be a
// AKSCluster) using the supplied resource claim (presumed to be a
// KubernetesCluster) and resource class.
func ConfigureAKSCluster(_ context.Context, cm resource.Claim, cs resource.Class, mg resource.Managed) error {
	if _, cmok := cm.(*computev1alpha1.KubernetesCluster); !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), computev1alpha1.KubernetesClusterGroupVersionKind)
	}

	rs, csok := cs.(*v1alpha1.AKSClusterClass)
	if !csok {
		return errors.Errorf("expected resource class %s to be %s", cs.GetName(), v1alpha1.AKSClusterClassGroupVersionKind)
	}

	i, mgok := mg.(*v1alpha1.AKSCluster)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.AKSClusterGroupVersionKind)
	}

	spec := &v1alpha1.AKSClusterSpec{
		ResourceSpec: runtimev1alpha1.ResourceSpec{
			ReclaimPolicy: runtimev1alpha1.ReclaimRetain,
		},
		AKSClusterParameters: rs.SpecTemplate.AKSClusterParameters,
	}

	// NOTE(hasheddan): consider moving defaulting to either CRD or managed reconciler level
	if spec.NodeCount == nil {
		spec.NodeCount = to.IntPtr(v1alpha1.DefaultNodeCount)
	}
	spec.WriteServicePrincipalSecretTo = corev1.LocalObjectReference{Name: fmt.Sprintf("principal-%s", cm.GetUID())}
	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = rs.SpecTemplate.ProviderReference
	spec.ReclaimPolicy = rs.SpecTemplate.ReclaimPolicy

	i.Spec = *spec

	return nil
}
