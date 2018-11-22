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

	s3Bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RDSInstanceHandler handles RDS Instance functionality
type S3BucketHandler struct{}

// find RDSInstance
func (h *S3BucketHandler) find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	s3Bucket := &s3Bucketv1alpha1.S3Bucket{}
	err := c.Get(ctx, name, s3Bucket)
	return s3Bucket, err
}

// provision creates a new S3Bucket
func (h *S3BucketHandler) provision(class *corev1alpha1.ResourceClass, instance *bucketv1alpha1.Bucket, c client.Client) (corev1alpha1.Resource, error) {
	// construct RDSInstance Spec from class definition
	bucketSpec := s3Bucketv1alpha1.NewS3BucketSpec(class.Parameters)
	bucketSpec.Name = instance.Spec.Name

	if instance.Spec.LocalPermissions != nil {
		bucketSpec.LocalPermissions = instance.Spec.LocalPermissions
	}

	if instance.Spec.PredefinedACL != nil {
		bucketSpec.CannedACL = s3Bucketv1alpha1.GetALCMap()[*instance.Spec.PredefinedACL]
	}

	bucketObjectName := fmt.Sprintf("%s-%s", "bucket", instance.UID)

	// assign provider reference and reclaim policy from the resource class
	bucketSpec.ProviderRef = class.ProviderRef
	bucketSpec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	bucketSpec.ClassRef = class.ObjectReference()
	bucketSpec.ClaimRef = instance.ObjectReference()

	// create and save RDSInstance
	bucket := &s3Bucketv1alpha1.S3Bucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: s3Bucketv1alpha1.APIVersion,
			Kind:       s3Bucketv1alpha1.S3BucketKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            bucketObjectName,
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *bucketSpec,
	}

	err := c.Create(ctx, bucket)
	return bucket, err
}

// bind updates resource state binding phase
// - state = true: bound
// - state = false: unbound
// TODO: this setBindStatus function could be refactored to 1 common implementation for all providers
func (h S3BucketHandler) setBindStatus(name types.NamespacedName, c client.Client, state bool) error {
	s3Bucket := &s3Bucketv1alpha1.S3Bucket{}
	err := c.Get(ctx, name, s3Bucket)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !state {
			return nil
		}
		return err
	}
	if state {
		s3Bucket.Status.SetBound()
	} else {
		s3Bucket.Status.SetUnbound()
	}
	return c.Update(ctx, s3Bucket)
}
