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

	azurecomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AKSClusterHandler handles Kubernetes cluster functionality
type AKSClusterHandler struct{}

// Find AKSCluster resource
func (r *AKSClusterHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	instance := &azurecomputev1alpha1.AKSCluster{}
	err := c.Get(ctx, name, instance)
	return instance, err
}

// provision a new AKSCluster
func (r *AKSClusterHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	// construct AKSCluster Spec from class definition
	resourceInstance := azurecomputev1alpha1.NewAKSClusterSpec(class.Parameters)

	// assign provider reference and reclaim policy from the resource class
	resourceInstance.ProviderRef = class.ProviderRef
	resourceInstance.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	resourceInstance.ClassRef = class.ObjectReference()
	resourceInstance.ClaimRef = claim.ObjectReference()

	// create and save AKSCluster
	cluster := &azurecomputev1alpha1.AKSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("aks-%s", claim.GetObjectMeta().GetUID()),
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *resourceInstance,
	}
	cluster.Status.SetUnbound()

	err := c.Create(ctx, cluster)
	return cluster, err
}

// SetBindStatus updates resource state binding phase
// TODO: this SetBindStatus function could be refactored to 1 common implementation for all providers
func (r AKSClusterHandler) SetBindStatus(name types.NamespacedName, c client.Client, bound bool) error {
	instance := &azurecomputev1alpha1.AKSCluster{}
	err := c.Get(ctx, name, instance)
	if err != nil {
		if errors.IsNotFound(err) && !bound {
			return nil
		}
		return err
	}
	if bound {
		instance.Status.SetBound()
	} else {
		instance.Status.SetUnbound()
	}
	return c.Update(ctx, instance)
}
