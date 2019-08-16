/*
Copyright 2019 The Crossplane Authors.

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

package fake

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/crossplaneio/crossplane/aws/apis/storage/v1alpha1"
	client "github.com/crossplaneio/crossplane/pkg/clients/aws/s3"
)

// MockS3Client for testing.
type MockS3Client struct {
	MockCreateOrUpdateBucket func(bucket *v1alpha1.S3Bucket) error
	MockGetBucketInfo        func(username string, bucket *v1alpha1.S3Bucket) (*client.Bucket, error)
	MockCreateUser           func(username string, bucket *v1alpha1.S3Bucket) (*iam.AccessKey, string, error)
	MockUpdateBucketACL      func(bucket *v1alpha1.S3Bucket) error
	MockUpdateVersioning     func(bucket *v1alpha1.S3Bucket) error
	MockUpdatePolicyDocument func(username string, bucket *v1alpha1.S3Bucket) (string, error)
	MockDelete               func(bucket *v1alpha1.S3Bucket) error
}

// CreateOrUpdateBucket calls the underlying MockCreateOrUpdateBucket method.
func (m *MockS3Client) CreateOrUpdateBucket(bucket *v1alpha1.S3Bucket) error {
	return m.MockCreateOrUpdateBucket(bucket)
}

// GetBucketInfo calls the underlying MockGetBucketInfo method.
func (m *MockS3Client) GetBucketInfo(username string, bucket *v1alpha1.S3Bucket) (*client.Bucket, error) {
	return m.MockGetBucketInfo(username, bucket)
}

// CreateUser calls the underlying MockCreateUser method.
func (m *MockS3Client) CreateUser(username string, bucket *v1alpha1.S3Bucket) (*iam.AccessKey, string, error) {
	return m.MockCreateUser(username, bucket)
}

// UpdateBucketACL calls the underlying MockUpdateBucketACL method.
func (m *MockS3Client) UpdateBucketACL(bucket *v1alpha1.S3Bucket) error {
	return m.MockUpdateBucketACL(bucket)
}

// UpdateVersioning calls the underlying MockUpdateVersioning method.
func (m *MockS3Client) UpdateVersioning(bucket *v1alpha1.S3Bucket) error {
	return m.MockUpdateVersioning(bucket)
}

// UpdatePolicyDocument calls the underlying MockUpdatePolicyDocument method.
func (m *MockS3Client) UpdatePolicyDocument(username string, bucket *v1alpha1.S3Bucket) (string, error) {
	return m.MockUpdatePolicyDocument(username, bucket)
}

// DeleteBucket calls the underlying MockDeleteBucket method.
func (m *MockS3Client) DeleteBucket(bucket *v1alpha1.S3Bucket) error {
	return m.MockDelete(bucket)
}
