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
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/storage"
	. "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
)

var (
	cfg *rest.Config
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	storage.AddToScheme(scheme.Scheme)
	t := test.NewTestEnv(namespace, test.CRDs())
	cfg = t.Start()
	t.StopAndExit(m.Run())
}

func TestResolveClassInstanceLocalPermissions(t *testing.T) {
	g := NewGomegaWithT(t)

	valid := func(class, instance, expected *LocalPermissionType) {
		v, err := resolveClassInstanceLocalPermissions(class, instance)
		g.Expect(v).To(Equal(expected))
		g.Expect(err).NotTo(HaveOccurred())
	}
	read := ReadOnlyPermission
	write := WriteOnlyPermission

	valid(&read, &read, &read)
	valid(&write, &write, &write)
	valid(&write, nil, &write)
	valid(nil, &read, &read)
	valid(nil, nil, nil)

	v, err := resolveClassInstanceLocalPermissions(&read, &write)
	g.Expect(v).To(BeNil())
	g.Expect(err).To(And(HaveOccurred(), MatchError("bucket instance value [Write] does not match the one defined in the resource class [Read]")))
}

func TestTranslateACL(t *testing.T) {
	g := NewGomegaWithT(t)

	valid := func(input *PredefinedACL, expected *s3.BucketCannedACL) {
		v, err := translateACL(input)
		g.Expect(v).To(Equal(expected))
		g.Expect(err).NotTo(HaveOccurred())
	}

	read := ACLPublicRead
	readWrite := ACLPublicReadWrite
	private := ACLPrivate
	authRead := ACLAuthenticatedRead

	readS3 := s3.BucketCannedACLPublicRead
	readWriteS3 := s3.BucketCannedACLPublicReadWrite
	privateS3 := s3.BucketCannedACLPrivate
	authReadS3 := s3.BucketCannedACLAuthenticatedRead

	valid(&read, &readS3)
	valid(&readWrite, &readWriteS3)
	valid(&private, &privateS3)
	valid(&authRead, &authReadS3)

	aclName := "invalid-acl"
	invalid := PredefinedACL(aclName)
	v, err := translateACL(&invalid)
	g.Expect(v).To(BeNil())
	g.Expect(err).To(And(HaveOccurred(), MatchError(fmt.Sprintf("PredefinedACL %s, not available in s3", aclName))))
}

func getBucketTestObjects() (instance *Bucket, class *corev1alpha1.ResourceClass, bucketSpec *s3Bucketv1alpha1.S3BucketSpec) {
	bucketSpec = &s3Bucketv1alpha1.S3BucketSpec{
		ReclaimPolicy: corev1alpha1.ReclaimDelete,
	}
	instance = &Bucket{}
	instance.UID = "uuid-test"
	class = &corev1alpha1.ResourceClass{
		ReclaimPolicy: corev1alpha1.ReclaimDelete,
	}
	return
}

func TestProvision(t *testing.T) {
	g := NewGomegaWithT(t)
	mc := &MockClient{}
	handler := S3BucketHandler{}

	valid := func(class *corev1alpha1.ResourceClass, instance *Bucket, expected *s3Bucketv1alpha1.S3Bucket) {
		var rtObj runtime.Object
		mc.MockCreate = func(ctx context.Context, obj runtime.Object) error {
			rtObj = obj
			return nil
		}

		_, err := handler.provision(class, instance, mc)
		g.Expect(err).To(BeNil())
		g.Expect(rtObj).To(Equal(expected))
	}

	// Setup defaults objects
	instance, class, bucketSpec := getBucketTestObjects()
	expected := handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Test canned acl from class param
	instance, class, bucketSpec = getBucketTestObjects()
	class.Parameters = map[string]string{"cannedACL": string(s3.ObjectCannedACLPublicReadWrite)}
	perm := s3.BucketCannedACLPublicReadWrite
	bucketSpec.CannedACL = &perm
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Instance public read write -> bucketspec publicreadwrite
	instance, class, bucketSpec = getBucketTestObjects()
	perm = s3.BucketCannedACLPublicReadWrite
	instancePerm := ACLPublicReadWrite
	instance.Spec.PredefinedACL = &instancePerm
	bucketSpec.CannedACL = &perm
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Test name from instance
	instance, class, bucketSpec = getBucketTestObjects()
	name := "test-name"
	instance.Spec.Name = name
	bucketSpec.Name = name
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Test localPermission from param
	instance, class, bucketSpec = getBucketTestObjects()
	localPerm := ReadWritePermission
	class.Parameters = map[string]string{"localPermission": string(localPerm)}
	bucketSpec.LocalPermission = &localPerm
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Test localPermission from instance
	instance, class, bucketSpec = getBucketTestObjects()
	instance.Spec.LocalPermission = &localPerm
	bucketSpec.LocalPermission = &localPerm
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)

	// Test localPermission from instance
	instance, class, bucketSpec = getBucketTestObjects()
	instance.Spec.LocalPermission = &localPerm
	bucketSpec.LocalPermission = &localPerm
	expected = handler.newS3Bucket(class, instance, bucketSpec)
	valid(class, instance, expected)
}

// MockClient controller-runtime client
type MockClient struct {
	client.Client

	MockCreate func(ctx context.Context, obj runtime.Object) error
	MockUpdate func(ctx context.Context, obj runtime.Object) error
}

func (mc *MockClient) Create(ctx context.Context, obj runtime.Object) error {
	return mc.MockCreate(ctx, obj)
}

func (mc *MockClient) Update(ctx context.Context, obj runtime.Object) error {
	return mc.MockUpdate(ctx, obj)
}
