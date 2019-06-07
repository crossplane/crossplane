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

package bucket

import (
	"reflect"

	"github.com/crossplaneio/crossplane/pkg/meta"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

type accountResolver interface {
	resolve(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error
}

type azureAccountResolver struct{}

func (a *azureAccountResolver) resolve(account *v1alpha1.Account, claim corev1alpha1.ResourceClaim) error {
	bucket, ok := claim.(*storagev1alpha1.Bucket)
	if !ok {
		return errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	if bucket.Spec.Name == "" {
		return errors.Errorf("invalid account claim: %s spec, name property is required", claim.GetName())
	}
	// Account name is globally unique, hence we are using name defined in claim
	account.Name = bucket.Spec.Name
	account.Spec.StorageAccountName = bucket.Spec.Name

	return nil
}

var _ accountResolver = &azureAccountResolver{}

// AzureAccountHandler dynamically provisions Azure storage account instances given a resource class.
type AzureAccountHandler struct {
	accountResolver
}

// Find an account instance.
func (h *AzureAccountHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	a := &v1alpha1.Account{}
	if err := c.Get(ctx, n, a); err != nil {
		return nil, errors.Wrapf(err, "cannot find Azure account instance %s", n)
	}
	return a, nil
}

// Provision a new GCS Bucket resource.
func (h *AzureAccountHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := v1alpha1.ParseAccountSpec(class.Parameters)

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = meta.ReferenceTo(class)
	spec.ClaimRef = meta.ReferenceTo(claim)

	account := &v1alpha1.Account{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.AccountKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(claim))},
		},
		Spec: *spec,
	}

	if err := h.resolve(account, claim); err != nil {
		return nil, errors.Wrapf(err, "failed to resolve account claim values")
	}

	if err := c.Create(ctx, account); err != nil {
		return nil, errors.Wrapf(err, "cannot create instance %s/%s", account.GetNamespace(), account.GetName())
	}

	return account, nil
}

// SetBindStatus marks the supplied GCS Bucket as bound or unbound
// in the Kubernetes API.
func (h *AzureAccountHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &v1alpha1.Account{}
	if err := c.Get(ctx, n, i); err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "cannot get account %s", n)
	}
	i.Status.SetBound(bound)
	return errors.Wrapf(c.Update(ctx, i), "cannot update account %s", n)
}

// AzureContainerHandler dynamically provisions Azure storage container instances given a resource class.
type AzureContainerHandler struct{}

// Find an account instance.
func (h *AzureContainerHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	container := &v1alpha1.Container{}
	if err := c.Get(ctx, n, container); err != nil {
		return nil, errors.Wrapf(err, "cannot find Azure container instance %s", n)
	}
	return container, nil
}

// Provision a new GCS Bucket resource.
func (h *AzureContainerHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := v1alpha1.ParseContainerSpec(class.Parameters)

	// In case of Container - the provider reference is the Account instance
	spec.AccountRef = class.ProviderRef

	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = meta.ReferenceTo(class)
	spec.ClaimRef = meta.ReferenceTo(claim)

	container := &v1alpha1.Container{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.ContainerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            string(claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(claim))},
		},
		Spec: *spec,
	}

	// retrieve account object
	acct := &v1alpha1.Account{}
	n := types.NamespacedName{Namespace: class.Namespace, Name: class.ProviderRef.Name}
	if err := c.Get(ctx, n, acct); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve account reference object: %s", n)
	}

	// add account as owner reference to this bucket, so that when account is deleted it triggers the deletion
	// of this bucket claim
	bucket, ok := claim.(*storagev1alpha1.Bucket)
	if !ok {
		return nil, errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}
	meta.AddOwnerReference(bucket, meta.AsOwner(meta.ReferenceTo(acct)))

	if err := c.Update(ctx, bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket claim with account owner reference: %s", bucket.Name)
	}

	if err := c.Create(ctx, container); err != nil {
		return nil, errors.Wrapf(err, "cannot create container instance %s/%s", container.GetNamespace(), container.GetName())
	}

	return container, nil
}

// SetBindStatus marks the supplied GCS Bucket as bound or unbound
// in the Kubernetes API.
func (h *AzureContainerHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &v1alpha1.Container{}
	if err := c.Get(ctx, n, i); err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "cannot get container %s", n)
	}
	i.Status.SetBound(bound)
	return errors.Wrapf(c.Update(ctx, i), "cannot update container %s", n)
}
