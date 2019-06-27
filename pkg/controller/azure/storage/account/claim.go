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

package account

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

// AddClaim adds a controller that reconciles Bucket resource claims by
// managing Bucket resources to the supplied Manager.
func AddClaim(mgr manager.Manager) error {
	r := resource.NewClaimReconciler(mgr,
		resource.ClaimKind(storagev1alpha1.BucketGroupVersionKind),
		resource.ManagedKind(v1alpha1.AccountGroupVersionKind),
		resource.WithManagedBinder(resource.NewAPIStatusManagedBinder(mgr.GetClient())),
		resource.WithManagedFinalizer(resource.NewAPIStatusManagedFinalizer(mgr.GetClient())),
		resource.WithManagedConfigurators(resource.ManagedConfiguratorFn(ConfigureAccount)))

	name := strings.ToLower(fmt.Sprintf("%s.%s", storagev1alpha1.BucketKind, controllerName))
	c, err := controller.New(name, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrapf(err, "cannot create %s controller", name)
	}

	if err := c.Watch(&source.Kind{Type: &v1alpha1.Account{}}, &resource.EnqueueRequestForClaim{}); err != nil {
		return errors.Wrapf(err, "cannot watch for %s", v1alpha1.AccountGroupVersionKind)
	}

	p := v1alpha1.AccountKindAPIVersion
	return errors.Wrapf(c.Watch(
		&source.Kind{Type: &storagev1alpha1.Bucket{}},
		&handler.EnqueueRequestForObject{},
		resource.NewPredicates(resource.ObjectHasProvisioner(mgr.GetClient(), p)),
	), "cannot watch for %s", storagev1alpha1.BucketGroupVersionKind)
}

// ConfigureAccount configures the supplied resource (presumed to be an Account)
// using the supplied resource claim (presumed to be a Bucket) and resource class.
func ConfigureAccount(_ context.Context, cm resource.Claim, cs *corev1alpha1.ResourceClass, mg resource.Managed) error {
	b, cmok := cm.(*storagev1alpha1.Bucket)
	if !cmok {
		return errors.Errorf("expected resource claim %s to be %s", cm.GetName(), storagev1alpha1.BucketGroupVersionKind)
	}

	a, mgok := mg.(*v1alpha1.Account)
	if !mgok {
		return errors.Errorf("expected managed resource %s to be %s", mg.GetName(), v1alpha1.AccountGroupVersionKind)
	}

	if b.Spec.Name == "" {
		return errors.Errorf("invalid account claim: %s spec, name property is required", b.GetName())
	}

	spec := v1alpha1.ParseAccountSpec(cs.Parameters)
	spec.StorageAccountName = b.Spec.Name

	spec.WriteConnectionSecretToReference = corev1.LocalObjectReference{Name: string(cm.GetUID())}
	spec.ProviderReference = cs.ProviderReference
	spec.ReclaimPolicy = cs.ReclaimPolicy

	a.Spec = *spec

	// Accounts do not follow the typical pattern of creating a managed resource
	// named claimkind-claimuuid because their associated container needs a
	// predictably named account from which to load its connection secret.
	// Instead we create an account with the same name as the claim.
	a.SetNamespace(cs.GetNamespace())
	a.SetName(b.GetName())

	// TODO(negz): Don't set this potentially cross-namespace owner reference.
	// We probably want to use the resource's reclaim policy, not Kubernetes
	// garbage collection, to determine whether to delete the managed resource
	// when the claim is deleted per
	// https://github.com/crossplaneio/crossplane/issues/550
	a.SetOwnerReferences([]v1.OwnerReference{meta.AsOwner(meta.ReferenceTo(b, storagev1alpha1.BucketGroupVersionKind))})

	return nil
}
