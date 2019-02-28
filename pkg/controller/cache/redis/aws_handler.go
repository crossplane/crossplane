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

	"github.com/crossplaneio/crossplane/pkg/apis/aws/cache/v1alpha1"
	cachev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/cache/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// ReplicationGroupHandler dynamically provisions ReplicationGroup resources given a resource class.
type ReplicationGroupHandler struct{}

// Find a ReplicationGroup resource.
func (h *ReplicationGroupHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	i := &v1alpha1.ReplicationGroup{}
	err := c.Get(ctx, n, i)
	return i, errors.Wrapf(err, "cannot find replication group %s", n)
}

// Provision a new ReplicationGroup resource.
func (h *ReplicationGroupHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := v1alpha1.NewReplicationGroupSpec(class.Parameters)

	if err := resolveAWSClassInstanceValues(spec, claim); err != nil {
		return nil, errors.Wrap(err, "cannot resolve Azure class instance values")
	}

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = class.ObjectReference()
	spec.ClaimRef = claim.ObjectReference()

	i := &v1alpha1.ReplicationGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.ReplicationGroupKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("redis-%s", claim.GetObjectMeta().GetUID()),
			OwnerReferences: []metav1.OwnerReference{claim.OwnerReference()},
		},
		Spec: *spec,
	}
	i.Status.SetUnbound()

	return i, errors.Wrapf(c.Create(ctx, i), "cannot create instance %s/%s", i.GetNamespace(), i.GetName())
}

// SetBindStatus marks the supplied ReplicationGroup resource as bound or unbound in the
// Kubernetes API.
func (h *ReplicationGroupHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &v1alpha1.ReplicationGroup{}
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

func resolveAWSClassInstanceValues(spec *v1alpha1.ReplicationGroupSpec, claim corev1alpha1.ResourceClaim) error {
	rc, ok := claim.(*cachev1alpha1.RedisCluster)
	if !ok {
		return errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	var err error
	switch {
	case spec.EngineVersion == "" && rc.Spec.EngineVersion == "":
	// Neither the claim nor its class specified a version. Let AWS pick.

	case spec.EngineVersion == "" && rc.Spec.EngineVersion != "":
		// Only the claim specified a version. Use the latest supported patch
		// version for said claim (minor) version.
		spec.EngineVersion, err = latestSupportedPatchVersion(rc.Spec.EngineVersion)

	case spec.EngineVersion != "" && rc.Spec.EngineVersion == "":
		// Only the class specified a version. Use it.

	case !strings.HasPrefix(spec.EngineVersion, rc.Spec.EngineVersion+"."):
		// Both the claim and its class specified a version, but the class
		// version is not a patch of the claim version.
		err = errors.Errorf("class version %s is not a patch of claim version %s", spec.EngineVersion, rc.Spec.EngineVersion)

	default:
		// Both the claim and its class specified a version, and the class
		// version is a patch of the claim version. Use the class version.
	}

	return errors.Wrap(err, "cannot resolve class claim values")
}

func latestSupportedPatchVersion(minorVersion string) (string, error) {
	p := v1alpha1.LatestSupportedPatchVersion[v1alpha1.MinorVersion(minorVersion)]
	if p == v1alpha1.UnsupportedVersion {
		return "", errors.Errorf("minor version %s is not currently supported", minorVersion)
	}
	return string(p), nil
}
