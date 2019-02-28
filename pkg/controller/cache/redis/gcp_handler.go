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
	"strings"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	gcpcachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
	corecontroller "github.com/crossplaneio/crossplane/pkg/controller/core"
)

// CloudMemorystoreInstanceHandler dynamically provisions Cloud Memorystore
// instances given a resource class.
type CloudMemorystoreInstanceHandler struct{}

// Find a CloudMemorystoreInstance instance.
func (h *CloudMemorystoreInstanceHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	i := &gcpcachev1alpha1.CloudMemorystoreInstance{}
	err := c.Get(ctx, n, i)
	return i, errors.Wrapf(err, "cannot find Cloud Memorystore instance %s", n)
}

// Provision a new CloudMemorystoreInstance resource.
func (h *CloudMemorystoreInstanceHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := gcpcachev1alpha1.NewCloudMemorystoreInstanceSpec(class.Parameters)

	if err := resolveGCPClassInstanceValues(spec, claim); err != nil {
		return nil, errors.Wrap(err, "cannot resolve GCP class instance values")
	}

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = class.ObjectReference()
	spec.ClaimRef = claim.ObjectReference()

	i := &gcpcachev1alpha1.CloudMemorystoreInstance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpcachev1alpha1.APIVersion,
			Kind:       gcpcachev1alpha1.CloudMemorystoreInstanceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("redis-%s", claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *spec,
	}
	i.Status.SetUnbound()

	return i, errors.Wrapf(c.Create(ctx, i), "cannot create instance %s/%s", i.GetNamespace(), i.GetName())
}

// SetBindStatus marks the supplied CloudMemorystoreInstance as bound or unbound
// in the Kubernetes API.
func (h *CloudMemorystoreInstanceHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &gcpcachev1alpha1.CloudMemorystoreInstance{}
	if err := c.Get(ctx, n, i); err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "cannot get instance %s", n)
	}
	i.Status.SetUnbound()
	if bound {
		i.Status.SetBound()
	}
	return errors.Wrapf(c.Update(ctx, i), "cannot update instance %s", n)
}

func resolveGCPClassInstanceValues(spec *gcpcachev1alpha1.CloudMemorystoreInstanceSpec, claim corev1alpha1.ResourceClaim) error {
	rc, ok := claim.(*cachev1alpha1.RedisCluster)
	if !ok {
		return errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	var err error
	spec.RedisVersion, err = corecontroller.ResolveClassClaimValues(spec.RedisVersion, toGCPFormat(rc.Spec.EngineVersion))
	return errors.Wrap(err, "cannot resolve class claim values")
}

// toGCPFormat transforms a RedisClusterSpec EngineVersion to a
// CloudMemoryStoreInstanceSpec RedisVersion. The former uses major.minor
// (e.g. 3.2). The latter uses REDIS_MAJOR_MINOR (e.g. REDIS_3_2).
func toGCPFormat(version string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("REDIS_%s", strings.Replace(version, ".", "_", -1))
}
