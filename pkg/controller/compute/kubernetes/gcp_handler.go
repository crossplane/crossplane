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

package kubernetes

import (
	"fmt"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
)

// A GKEClusterHandler handles Kubernetes cluster functionality
type GKEClusterHandler struct{}

// Find GKECluster resource
func (r *GKEClusterHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	instance := &v1alpha1.GKECluster{}
	if err := c.Get(ctx, name, instance); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve %s: %s", v1alpha1.GKEClusterKind, name)
	}
	return instance, nil
}

// Provision a new GKECluster
func (r *GKEClusterHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	// construct GKECluster Spec from class definition
	resourceInstance := v1alpha1.ParseClusterSpec(class.Parameters)

	// assign provider reference and reclaim policy from the resource class
	resourceInstance.ProviderRef = class.ProviderRef
	resourceInstance.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	resourceInstance.ClassRef = class.ObjectReference()
	resourceInstance.ClaimRef = claim.ObjectReference()

	// create and save GKECluster
	cluster := &v1alpha1.GKECluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          map[string]string{labelProviderKey: labelProviderGCP},
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("gke-%s", claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *resourceInstance,
	}

	if err := c.Create(ctx, cluster); err != nil {
		return nil, errors.Wrapf(err, "failed to create cluster %s/%s", cluster.Namespace, cluster.Name)
	}
	return cluster, nil
}

// SetBindStatus updates resource state binding phase
// TODO: this SetBindStatus function could be refactored to 1 common implementation for all providers
func (r GKEClusterHandler) SetBindStatus(name types.NamespacedName, c client.Client, bound bool) error {
	instance := &v1alpha1.GKECluster{}
	err := c.Get(ctx, name, instance)
	if err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "failed to retrieve cluster %s", name)
	}
	instance.Status.SetBound(bound)
	return errors.Wrapf(c.Update(ctx, instance), "failed to update cluster %s", name)
}
