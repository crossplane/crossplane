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

package cloudmemorystore

import (
	"context"
	"fmt"
	"reflect"

	redisv1 "cloud.google.com/go/redis/apiv1"
	"github.com/googleapis/gax-go"
	"google.golang.org/api/option"
	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"
	"google.golang.org/genproto/protobuf/field_mask"

	"github.com/crossplaneio/crossplane/gcp/apis/cache/v1alpha1"
)

// NamePrefix is the prefix for all created CloudMemorystore instances.
const NamePrefix = "cms"

// A Client handles CRUD operations for Cloud Memorystore instances. This
// interface is compatible with the upstream CloudRedisClient.
type Client interface {
	CreateInstance(ctx context.Context, req *redisv1pb.CreateInstanceRequest, opts ...gax.CallOption) (*redisv1.CreateInstanceOperation, error)
	UpdateInstance(ctx context.Context, req *redisv1pb.UpdateInstanceRequest, opts ...gax.CallOption) (*redisv1.UpdateInstanceOperation, error)
	DeleteInstance(ctx context.Context, req *redisv1pb.DeleteInstanceRequest, opts ...gax.CallOption) (*redisv1.DeleteInstanceOperation, error)
	GetInstance(ctx context.Context, req *redisv1pb.GetInstanceRequest, opts ...gax.CallOption) (*redisv1pb.Instance, error)
}

// NewClient returns a new CloudMemorystore Client. Credentials must be passed
// as JSON encoded data.
func NewClient(ctx context.Context, credentials []byte) (Client, error) {
	return redisv1.NewCloudRedisClient(ctx, option.WithCredentialsJSON(credentials))
}

// An InstanceID represents a CloudMemorystore instance in the GCP API.
type InstanceID struct {
	// Project in which this instance exists.
	Project string

	// Region in which this instance exists. The API calls this a 'location',
	// which is an overloaded term considering instances also have a 'location
	// id' (and 'alternative location id'), which represent zones.
	Region string

	// Instance name, or ID. The GCP API appears to call the unqualified name
	// (e.g. 'coolinstance') an ID, and the qualified name (e.g.
	// 'projects/coolproject/locations/us-west2/instances/coolinstance') a name.
	Instance string
}

// NewInstanceID returns an identifier used to represent CloudMemorystore
// instances in the GCP API. Instances may have names of up to 40 characters. We
// use a four character prefix and a 36 character UUID.
// https://godoc.org/google.golang.org/genproto/googleapis/cloud/redis/v1#CreateInstanceRequest
func NewInstanceID(project string, i *v1alpha1.CloudMemorystoreInstance) InstanceID {
	id := InstanceID{Project: project, Region: i.Spec.Region, Instance: i.Status.InstanceName}
	if id.Instance == "" {
		id.Instance = fmt.Sprintf("%s-%s", NamePrefix, i.GetUID())
	}
	return id
}

// Parent returns the instance's parent, suitable for the create API call.
func (id InstanceID) Parent() string {
	return fmt.Sprintf("projects/%s/locations/%s", id.Project, id.Region)
}

// Name returns the instance's name, suitable for get and delete API calls.
func (id InstanceID) Name() string {
	return fmt.Sprintf("projects/%s/locations/%s/instances/%s", id.Project, id.Region, id.Instance)
}

// NewInstanceTier converts the supplied string representation of a tier into an
// Instance_Tier suitable for use with requests to the GCP API.
func NewInstanceTier(t string) redisv1pb.Instance_Tier {
	return redisv1pb.Instance_Tier(redisv1pb.Instance_Tier_value[t])
}

// NewCreateInstanceRequest creates a request to create an instance suitable for
// use with the GCP API.
func NewCreateInstanceRequest(id InstanceID, i *v1alpha1.CloudMemorystoreInstance) *redisv1pb.CreateInstanceRequest {
	return &redisv1pb.CreateInstanceRequest{
		Parent:     id.Parent(),
		InstanceId: id.Instance,
		Instance: &redisv1pb.Instance{
			Tier:                  NewInstanceTier(i.Spec.Tier),
			LocationId:            i.Spec.LocationID,
			AlternativeLocationId: i.Spec.AlternativeLocationID,
			ReservedIpRange:       i.Spec.ReservedIPRange,
			AuthorizedNetwork:     i.Spec.AuthorizedNetwork,
			RedisVersion:          i.Spec.RedisVersion,
			RedisConfigs:          i.Spec.RedisConfigs,
			MemorySizeGb:          int32(i.Spec.MemorySizeGB),
		},
	}
}

// NewUpdateInstanceRequest creates a request to update an instance suitable for
// use with the GCP API.
func NewUpdateInstanceRequest(id InstanceID, i *v1alpha1.CloudMemorystoreInstance) *redisv1pb.UpdateInstanceRequest {
	return &redisv1pb.UpdateInstanceRequest{
		// These are the only fields we're concerned with that can be updated.
		// The documentation is incorrect regarding field masks - they must be
		// specified as snake case rather than camel case.
		// https://godoc.org/google.golang.org/genproto/googleapis/cloud/redis/v1#UpdateInstanceRequest
		UpdateMask: &field_mask.FieldMask{Paths: []string{"memory_size_gb", "redis_configs"}},
		Instance: &redisv1pb.Instance{
			Name:         id.Name(),
			RedisConfigs: i.Spec.RedisConfigs,
			MemorySizeGb: int32(i.Spec.MemorySizeGB),
		},
	}
}

// NeedsUpdate returns true if the supplied Kubernetes resource differs from the
// supplied GCP resource. It considers only fields that can be modified in
// place without deleting and recreating the instance.
func NeedsUpdate(kube *v1alpha1.CloudMemorystoreInstance, gcp *redisv1pb.Instance) bool {
	if kube.Spec.MemorySizeGB != int(gcp.GetMemorySizeGb()) {
		return true
	}
	if !reflect.DeepEqual(kube.Spec.RedisConfigs, gcp.GetRedisConfigs()) {
		return true
	}
	return false
}

// NewDeleteInstanceRequest creates a request to delete an instance suitable for
// use with the GCP API.
func NewDeleteInstanceRequest(id InstanceID) *redisv1pb.DeleteInstanceRequest {
	return &redisv1pb.DeleteInstanceRequest{Name: id.Name()}
}

// NewGetInstanceRequest creates a request to get an instance from the GCP API.
func NewGetInstanceRequest(id InstanceID) *redisv1pb.GetInstanceRequest {
	return &redisv1pb.GetInstanceRequest{Name: id.Name()}
}
