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

	awscomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AWSClusterHandler AWS EKS handler handles Kubernetes cluster functionality
type AWSClusterHandler struct{}

// find EKSCluster resource
func (r *AWSClusterHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	instance := &awscomputev1alpha1.EKSCluster{}
	err := c.Get(ctx, name, instance)
	return instance, err
}

// provision create new EKSCluster
func (r *AWSClusterHandler) provision(class *corev1alpha1.ResourceClass, instance *computev1alpha1.KubernetesCluster, c client.Client) (corev1alpha1.Resource, error) {
	// construct EKSCluster Spec from class definition
	resourceInstance := awscomputev1alpha1.NewEKSClusterSpec(class.Parameters)

	// assign provider reference and reclaim policy from the resource class
	resourceInstance.ProviderRef = class.ProviderRef
	resourceInstance.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	resourceInstance.ClassRef = class.ObjectReference()
	resourceInstance.ClaimRef = instance.ObjectReference()

	// create and save EKSCluster
	cluster := &awscomputev1alpha1.EKSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("eks-%s", instance.UID),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *resourceInstance,
	}
	cluster.Status.SetUnbound()

	err := c.Create(ctx, cluster)
	return cluster, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
func (r AWSClusterHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	instance := &awscomputev1alpha1.EKSCluster{}
	err := c.Get(ctx, name, instance)
	if err != nil {
		if errors.IsNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		instance.Status.SetBound()
	} else {
		instance.Status.SetUnbound()
	}
	return c.Update(ctx, instance)
}
