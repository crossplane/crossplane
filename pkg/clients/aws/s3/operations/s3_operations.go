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

package operations

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3iface"
)

// S3Operations provides methods for common S3 operations
type S3Operations struct {
	s3 s3iface.S3API
}

// NewS3Operations creates a new instance of S3Operations
func NewS3Operations(s3 s3iface.S3API) *S3Operations {
	return &S3Operations{s3: s3}
}

// GetBucketVersioningRequest creates a get bucket versioning request
func (api *S3Operations) GetBucketVersioningRequest(i *s3.GetBucketVersioningInput) GetBucketVersioningRequest {
	return api.s3.GetBucketVersioningRequest(i)
}

// PutBucketACLRequest creates a put bucket ACL request
func (api *S3Operations) PutBucketACLRequest(i *s3.PutBucketAclInput) PutBucketACLRequest {
	return api.s3.PutBucketAclRequest(i)
}

// PutBucketVersioningRequest creates a put bucket versioning request
func (api *S3Operations) PutBucketVersioningRequest(i *s3.PutBucketVersioningInput) PutBucketVersioningRequest {
	return api.s3.PutBucketVersioningRequest(i)
}

// DeleteBucketRequest creates a delete bucket request
func (api *S3Operations) DeleteBucketRequest(i *s3.DeleteBucketInput) DeleteBucketRequest {
	return api.s3.DeleteBucketRequest(i)
}

// CreateBucketRequest creates a create bucket request
func (api *S3Operations) CreateBucketRequest(i *s3.CreateBucketInput) CreateBucketRequest {
	return api.s3.CreateBucketRequest(i)
}
