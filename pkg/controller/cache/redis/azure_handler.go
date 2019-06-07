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

package redis

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

// TODO(negz): Name this something that doesn't stutter? redis.RedisHandler is
// an unfortunate name, but we

// RedisHandler dynamically provisions Redis resources given a resource class.
type RedisHandler struct{} // nolint:golint

// Find a Redis resource.
func (h *RedisHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	i := &v1alpha1.Redis{}
	err := c.Get(ctx, n, i)
	return i, errors.Wrapf(err, "cannot find Azure Redis Cache %s", n)
}

// Provision a new Redis resource.
func (h *RedisHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := v1alpha1.NewRedisSpec(class.Parameters)

	if err := resolveAzureClassValues(claim); err != nil {
		return nil, errors.Wrap(err, "cannot resolve Azure class instance values")
	}

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = meta.ReferenceTo(class)
	spec.ClaimRef = meta.ReferenceTo(claim)

	i := &v1alpha1.Redis{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.RedisKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("redis-%s", claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(claim))},
		},
		Spec: *spec,
	}

	return i, errors.Wrapf(c.Create(ctx, i), "cannot create instance %s/%s", i.GetNamespace(), i.GetName())
}

// SetBindStatus marks the supplied Redis resource as bound or unbound in the
// Kubernetes API.
func (h *RedisHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &v1alpha1.Redis{}
	if err := c.Get(ctx, n, i); err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "cannot get instance %s", n)
	}
	i.Status.SetBound(bound)
	return errors.Wrapf(c.Update(ctx, i), "cannot update instance %s", n)
}

func resolveAzureClassValues(claim corev1alpha1.ResourceClaim) error {
	rc, ok := claim.(*cachev1alpha1.RedisCluster)
	if !ok {
		return errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	if rc.Spec.EngineVersion == "" {
		return nil
	}

	// EngineVersion is currently the only option we expose at the claim level,
	// and Azure only supports Redis 3.2.
	if rc.Spec.EngineVersion != v1alpha1.SupportedRedisVersion {
		return errors.Errorf("Azure supports only Redis version %s", v1alpha1.SupportedRedisVersion)
	}

	return nil
}
