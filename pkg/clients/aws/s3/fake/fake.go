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

package fake

import (
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/crossplaneio/crossplane/pkg/apis/aws/storage/v1alpha1"
	client "github.com/crossplaneio/crossplane/pkg/clients/aws/s3"
)

type MockS3Client struct {
	MockCreateOrUpdateBucket func(spec *v1alpha1.S3BucketSpec) error
	MockGetBucketInfo        func(username string, spec *v1alpha1.S3BucketSpec) (*client.Bucket, error)
	MockCreateUser           func(username string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, string, error)
	MockUpdateBucketACL      func(spec *v1alpha1.S3BucketSpec) error
	MockUpdateVersioning     func(spec *v1alpha1.S3BucketSpec) error
	MockUpdatePolicyDocument func(username string, spec *v1alpha1.S3BucketSpec) (string, error)
	MockDelete               func(bucket *v1alpha1.S3Bucket) error
}

// CreateBucket mock
func (m *MockS3Client) CreateOrUpdateBucket(spec *v1alpha1.S3BucketSpec) error {
	return m.MockCreateOrUpdateBucket(spec)
}

func (m *MockS3Client) GetBucketInfo(username string, spec *v1alpha1.S3BucketSpec) (*client.Bucket, error) {
	return m.MockGetBucketInfo(username, spec)
}

func (m *MockS3Client) CreateUser(username string, spec *v1alpha1.S3BucketSpec) (*iam.AccessKey, string, error) {
	return m.MockCreateUser(username, spec)
}

func (m *MockS3Client) UpdateBucketACL(spec *v1alpha1.S3BucketSpec) error {
	return m.MockUpdateBucketACL(spec)
}

func (m *MockS3Client) UpdateVersioning(spec *v1alpha1.S3BucketSpec) error {
	return m.MockUpdateVersioning(spec)
}

func (m *MockS3Client) UpdatePolicyDocument(username string, spec *v1alpha1.S3BucketSpec) (string, error) {
	return m.MockUpdatePolicyDocument(username, spec)
}

func (m *MockS3Client) DeleteBucket(bucket *v1alpha1.S3Bucket) error {
	return m.MockDelete(bucket)
}
