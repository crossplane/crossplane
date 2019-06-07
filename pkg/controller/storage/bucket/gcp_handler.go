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
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/storage/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

// GCSBucketHandler dynamically provisions GCS Bucket instances given a resource class.
type GCSBucketHandler struct{}

// Find a Bucket instance.
func (h *GCSBucketHandler) Find(n types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	i := &v1alpha1.Bucket{}
	if err := c.Get(ctx, n, i); err != nil {
		return nil, errors.Wrapf(err, "cannot find gcs bucket instance %s", n)
	}
	return i, nil
}

// Provision a new GCS Bucket resource.
func (h *GCSBucketHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	spec := v1alpha1.ParseBucketSpec(class.Parameters)

	spec.ProviderRef = class.ProviderRef
	spec.ReclaimPolicy = class.ReclaimPolicy
	spec.ClassRef = meta.ReferenceTo(class)
	spec.ClaimRef = meta.ReferenceTo(claim)

	bucket := &v1alpha1.Bucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.BucketKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("gcs-%s", claim.GetUID()),
			OwnerReferences: []metav1.OwnerReference{meta.AsOwner(meta.ReferenceTo(claim))},
		},
		Spec: *spec,
	}

	if err := resolveGCSBucketClaimValues(bucket, claim); err != nil {
		return nil, errors.Wrapf(err, "failed to resolve GCSBucket spec values")
	}

	return bucket, errors.Wrapf(c.Create(ctx, bucket), "cannot create instance %s/%s", bucket.GetNamespace(), bucket.GetName())
}

// SetBindStatus marks the supplied GCS Bucket as bound or unbound in the Kubernetes API.
func (h *GCSBucketHandler) SetBindStatus(n types.NamespacedName, c client.Client, bound bool) error {
	i := &v1alpha1.Bucket{}
	if err := c.Get(ctx, n, i); err != nil {
		if kerrors.IsNotFound(err) && !bound {
			return nil
		}
		return errors.Wrapf(err, "cannot get bucket %s", n)
	}
	i.Status.SetBound(bound)
	return errors.Wrapf(c.Update(ctx, i), "cannot update bucket %s", n)
}

func resolveGCSBucketClaimValues(bucket *v1alpha1.Bucket, claim corev1alpha1.ResourceClaim) error {
	bucketClaim, ok := claim.(*storagev1alpha1.Bucket)
	if !ok {
		return errors.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	// Set Name bucket name if Name value is provided by Bucket Claim spec
	if bucketClaim.Spec.Name != "" {
		bucket.Spec.NameFormat = bucketClaim.Spec.Name
	}

	spec := bucket.Spec

	// Set PredefinedACL from bucketClaim claim only iff: claim has this value and
	// it is not defined in the resource class (i.e. not already in the spec)
	if bucketClaim.Spec.PredefinedACL != nil && spec.PredefinedACL == "" {
		spec.PredefinedACL = string(*bucketClaim.Spec.PredefinedACL)
	}

	return nil
}
