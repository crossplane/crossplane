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
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s3Bucketv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/storage"
	. "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
)

func init() {
	flag.Parse()
}

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, storage.AddToSchemes, test.CRDs())
	t.Start()
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

func getBucketTestObjects() (claim *Bucket, class *corev1alpha1.ResourceClass, bucketSpec *s3Bucketv1alpha1.S3BucketSpec) {
	bucketSpec = &s3Bucketv1alpha1.S3BucketSpec{
		ReclaimPolicy: corev1alpha1.ReclaimDelete,
	}
	claim = &Bucket{}
	claim.UID = "uuid-test"
	class = &corev1alpha1.ResourceClass{
		ReclaimPolicy: corev1alpha1.ReclaimDelete,
	}
	return
}

func TestProvision(t *testing.T) {
	g := NewGomegaWithT(t)
	mc := &MockClient{}
	handler := S3BucketHandler{}

	valid := func(class *corev1alpha1.ResourceClass, claim *Bucket, expected *s3Bucketv1alpha1.S3Bucket) {
		var rtObj runtime.Object
		mc.MockCreate = func(ctx context.Context, obj runtime.Object) error {
			rtObj = obj
			return nil
		}

		_, err := handler.Provision(class, claim, mc)
		g.Expect(err).To(BeNil())
		g.Expect(expected).To(Equal(rtObj))
	}

	// Setup defaults objects
	claim, class, bucketSpec := getBucketTestObjects()
	expected := handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)

	// Test canned acl from class param
	claim, class, bucketSpec = getBucketTestObjects()
	class.Parameters = map[string]string{"cannedACL": string(s3.ObjectCannedACLPublicReadWrite)}
	perm := s3.BucketCannedACLPublicReadWrite
	bucketSpec.CannedACL = &perm
	expected = handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)

	// claim public read write -> bucketspec publicreadwrite
	claim, class, bucketSpec = getBucketTestObjects()
	perm = s3.BucketCannedACLPublicReadWrite
	claimPerm := ACLPublicReadWrite
	claim.Spec.PredefinedACL = &claimPerm
	bucketSpec.CannedACL = &perm
	expected = handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)

	// Test name from claim
	claim, class, bucketSpec = getBucketTestObjects()
	name := "test-name"
	claim.Spec.Name = name
	bucketSpec.NameFormat = name
	expected = handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)

	// Test localPermission from param
	claim, class, bucketSpec = getBucketTestObjects()
	localPerm := ReadWritePermission
	class.Parameters = map[string]string{"localPermission": string(localPerm)}
	bucketSpec.LocalPermission = &localPerm
	expected = handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)

	// Test localPermission from claim
	claim, class, bucketSpec = getBucketTestObjects()
	claim.Spec.LocalPermission = &localPerm
	bucketSpec.LocalPermission = &localPerm
	expected = handler.newS3Bucket(class, claim, bucketSpec)
	valid(class, claim, expected)
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
