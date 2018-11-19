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
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpcomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GCP GKE handler handles Kubernetes cluster functionality
type GKEClusterHandler struct{}

// find GKECluster resource
func (r *GKEClusterHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	instance := &gcpcomputev1alpha1.GKECluster{}
	err := c.Get(ctx, name, instance)
	return instance, err
}

// provision create new GKECluster
func (r *GKEClusterHandler) provision(class *corev1alpha1.ResourceClass, instance *computev1alpha1.KubernetesCluster, c client.Client) (corev1alpha1.Resource, error) {
	// construct GKECluster Spec from class definition
	resourceInstance := gcpcomputev1alpha1.NewGKEClusterSpec(class.Parameters)

	// assign provider reference and reclaim policy from the resource class
	resourceInstance.ProviderRef = class.ProviderRef
	resourceInstance.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	resourceInstance.ClassRef = class.ObjectReference()
	resourceInstance.ClaimRef = instance.ObjectReference()

	// create and save GKECluster
	cluster := &gcpcomputev1alpha1.GKECluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            util.GenerateName(instance.Name + "-"),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *resourceInstance,
	}

	err := c.Create(ctx, cluster)
	return cluster, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
func (r GKEClusterHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	instance := &gcpcomputev1alpha1.GKECluster{}
	err := c.Get(ctx, name, instance)
	if err != nil {
		if gcp.IsErrorNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		instance.Status.Phase = corev1alpha1.BindingStateBound
	} else {
		instance.Status.Phase = corev1alpha1.BindingStateBound
	}
	return c.Update(ctx, instance)
}
