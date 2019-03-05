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

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s3Bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

var (
	predefinedACLMap = map[bucketv1alpha1.PredefinedACL]s3.BucketCannedACL{
		bucketv1alpha1.ACLPrivate:           s3.BucketCannedACLPrivate,
		bucketv1alpha1.ACLPublicRead:        s3.BucketCannedACLPublicRead,
		bucketv1alpha1.ACLPublicReadWrite:   s3.BucketCannedACLPublicReadWrite,
		bucketv1alpha1.ACLAuthenticatedRead: s3.BucketCannedACLAuthenticatedRead,
	}
)

// S3BucketHandler handles S3 Instance functionality
type S3BucketHandler struct{}

// find S3BUCKET
func (h *S3BucketHandler) Find(name types.NamespacedName, c client.Client) (corev1alpha1.Resource, error) {
	s3Bucket := &s3Bucketv1alpha1.S3Bucket{}
	err := c.Get(ctx, name, s3Bucket)
	return s3Bucket, err
}

// newS3Bucket initialized bucket with resources and object references applied
func (h *S3BucketHandler) newS3Bucket(class *corev1alpha1.ResourceClass, instance *bucketv1alpha1.Bucket, bucketSpec *s3Bucketv1alpha1.S3BucketSpec) *s3Bucketv1alpha1.S3Bucket {
	// create and save S3Bucket
	bucket := &s3Bucketv1alpha1.S3Bucket{
		TypeMeta: metav1.TypeMeta{
			APIVersion: s3Bucketv1alpha1.APIVersion,
			Kind:       s3Bucketv1alpha1.S3BucketKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       class.Namespace,
			Name:            fmt.Sprintf("%s-%s", "bucket", instance.UID),
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Spec: *bucketSpec,
	}

	bucket.Spec.ProviderRef = class.ProviderRef
	bucket.Spec.ReclaimPolicy = class.ReclaimPolicy

	// set class and claim references
	bucket.Spec.ClassRef = class.ObjectReference()
	bucket.Spec.ClaimRef = instance.ObjectReference()

	return bucket
}

// provision creates a new S3Bucket
func (h *S3BucketHandler) Provision(class *corev1alpha1.ResourceClass, claim corev1alpha1.ResourceClaim, c client.Client) (corev1alpha1.Resource, error) {
	// construct S3Bucket Spec from class definition
	bucketSpec := s3Bucketv1alpha1.NewS3BucketSpec(class.Parameters)

	bucket, ok := claim.(*bucketv1alpha1.Bucket)
	if !ok {
		return nil, fmt.Errorf("unexpected claim type: %+v", reflect.TypeOf(claim))
	}

	// Making connection secret override configurable from parameters doesn't make sense, so we take the value from the instance.
	bucketSpec.ConnectionSecretNameOverride = bucket.Spec.ConnectionSecretNameOverride

	val, err := resolveClassInstanceValues(bucketSpec.Name, bucket.Spec.Name)
	if err != nil {
		return nil, err
	}
	bucketSpec.Name = val

	// translate and set predefinedACL
	instanceACL, err := translateACL(bucket.Spec.PredefinedACL)
	if err != nil {
		return nil, err
	}

	cannedACL, err := resolveClassInstanceACL(bucketSpec.CannedACL, instanceACL)
	if err != nil {
		return nil, err
	}

	if cannedACL != nil {
		bucketSpec.CannedACL = cannedACL
	}

	bucketSpec.LocalPermission, err = resolveClassInstanceLocalPermissions(bucketSpec.LocalPermission, bucket.Spec.LocalPermission)
	if err != nil {
		return nil, err
	}

	s3bucket := h.newS3Bucket(class, bucket, bucketSpec)

	err = c.Create(ctx, s3bucket)
	return s3bucket, err
}

// SetBindStatus updates resource state binding phase
// TODO: this SetBindStatus function could be refactored to 1 common implementation for all providers
func (h S3BucketHandler) SetBindStatus(name types.NamespacedName, c client.Client, bound bool) error {
	s3Bucket := &s3Bucketv1alpha1.S3Bucket{}
	err := c.Get(ctx, name, s3Bucket)
	if err != nil {
		// TODO: the CRD is not found and the binding state is supposed to be unbound. is this OK?
		if errors.IsNotFound(err) && !bound {
			return nil
		}
		return err
	}
	s3Bucket.Status.SetBound(bound)
	return c.Update(ctx, s3Bucket)
}

// resolveClassInstanceACL validates instance value against resource class properties.
// if both values are defined, then the instance value is validated against the resource class value and expected to match
// TODO: the "matching" process will be further refined once we implement constraint policies at the resource class level
func resolveClassInstanceACL(classValue *s3.BucketCannedACL, instanceValue *s3.BucketCannedACL) (*s3.BucketCannedACL, error) {
	if classValue == nil {
		return instanceValue, nil
	}
	if instanceValue == nil {
		return classValue, nil
	}
	if *classValue != *instanceValue {
		return nil, fmt.Errorf("bucket instance value [%s] does not match the one defined in the resource class [%s]", *instanceValue, *classValue)
	}
	return instanceValue, nil
}

// resolveClassInstanceValues validates instance value against resource class properties.
// if both values are defined, then the instance value is validated against the resource class value and expected to match
// TODO: the "matching" process will be further refined once we implement constraint policies at the resource class level
func resolveClassInstanceLocalPermissions(classValue, instanceValue *bucketv1alpha1.LocalPermissionType) (*bucketv1alpha1.LocalPermissionType, error) {
	if classValue == nil {
		return instanceValue, nil
	}
	if instanceValue == nil {
		return classValue, nil
	}
	if *classValue != *instanceValue {
		return nil, fmt.Errorf("bucket instance value [%s] does not match the one defined in the resource class [%s]", *instanceValue, *classValue)
	}
	return instanceValue, nil
}

func translateACL(acl *bucketv1alpha1.PredefinedACL) (*s3.BucketCannedACL, error) {
	if acl == nil {
		return nil, nil
	}

	if val, found := predefinedACLMap[*acl]; found {
		return &val, nil
	}

	return nil, fmt.Errorf("PredefinedACL %s, not available in s3", *acl)
}

// resolveClassInstanceValues validates instance value against resource class properties.
// if both values are defined, then the instance value is validated against the resource class value and expected to match
// TODO: the "matching" process will be further refined once we implement constraint policies at the resource class level
func resolveClassInstanceValues(classValue, instanceValue string) (string, error) {
	if classValue == "" {
		return instanceValue, nil
	}
	if instanceValue == "" {
		return classValue, nil
	}
	if classValue != instanceValue {
		return "", fmt.Errorf("bucket instance value [%s] does not match the one defined in the resource class [%s]", instanceValue, classValue)
	}
	return instanceValue, nil
}
