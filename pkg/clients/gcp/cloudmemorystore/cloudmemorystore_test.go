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
	"testing"

	redisv1pb "google.golang.org/genproto/googleapis/cloud/redis/v1"
	"google.golang.org/genproto/protobuf/field_mask"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp/cache/v1alpha1"
)

const (
	uid                   = types.UID("definitely-a-uuid")
	region                = "us-cool1"
	project               = "coolProject"
	instanceName          = NamePrefix + "-" + string(uid)
	qualifiedName         = "projects/" + project + "/locations/" + region + "/instances/" + instanceName
	parent                = "projects/" + project + "/locations/" + region
	locationID            = region + "-a"
	alternativeLocationID = region + "-b"
	reservedIPRange       = "172.16.0.0/16"
	authorizedNetwork     = "default"
	redisVersion          = "REDIS_3_2"
	memorySizeGB          = 1
)

var (
	redisConfigs = map[string]string{"cool": "socool"}
	updateMask   = &field_mask.FieldMask{Paths: []string{"memory_size_gb", "redis_configs"}}
)

func TestInstanceID(t *testing.T) {
	cases := []struct {
		name       string
		project    string
		i          *v1alpha1.CloudMemorystoreInstance
		want       InstanceID
		wantName   string
		wantParent string
	}{
		{
			name:    "InstanceNameUnset",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec:       v1alpha1.CloudMemorystoreInstanceSpec{Region: region},
			},
			want: InstanceID{
				Project:  project,
				Region:   region,
				Instance: instanceName,
			},
			wantName:   qualifiedName,
			wantParent: parent,
		},
		{
			name:    "InstanceNameSet",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				ObjectMeta: metav1.ObjectMeta{UID: types.UID("i-am-different")},
				Spec:       v1alpha1.CloudMemorystoreInstanceSpec{Region: region},
				Status:     v1alpha1.CloudMemorystoreInstanceStatus{InstanceName: instanceName},
			},
			want: InstanceID{
				Project:  project,
				Region:   region,
				Instance: instanceName,
			},
			wantName:   qualifiedName,
			wantParent: parent,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewInstanceID(tc.project, tc.i)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewInstanceID(...): -want, +got:\n%s", diff)
			}

			gotName := got.Name()
			if gotName != tc.wantName {
				t.Errorf("got.Name(): want: %s got: %s", tc.wantName, gotName)
			}

			gotParent := got.Parent()
			if gotParent != tc.wantParent {
				t.Errorf("got.Parent(): want: %s got: %s", tc.wantParent, gotParent)
			}
		})
	}
}

func TestNewCreateInstanceRequest(t *testing.T) {
	cases := []struct {
		name    string
		project string
		i       *v1alpha1.CloudMemorystoreInstance
		want    *redisv1pb.CreateInstanceRequest
	}{
		{
			name:    "BasicInstance",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					Region:                region,
					Tier:                  redisv1pb.Instance_BASIC.String(),
					LocationID:            locationID,
					AlternativeLocationID: alternativeLocationID,
					ReservedIPRange:       reservedIPRange,
					AuthorizedNetwork:     authorizedNetwork,
					RedisVersion:          redisVersion,
					RedisConfigs:          redisConfigs,
					MemorySizeGB:          memorySizeGB,
				},
				Status: v1alpha1.CloudMemorystoreInstanceStatus{InstanceName: instanceName},
			},
			want: &redisv1pb.CreateInstanceRequest{
				Parent:     parent,
				InstanceId: instanceName,
				Instance: &redisv1pb.Instance{
					Tier:                  redisv1pb.Instance_BASIC,
					LocationId:            locationID,
					AlternativeLocationId: alternativeLocationID,
					ReservedIpRange:       reservedIPRange,
					AuthorizedNetwork:     authorizedNetwork,
					RedisVersion:          redisVersion,
					RedisConfigs:          redisConfigs,
					MemorySizeGb:          memorySizeGB,
				},
			},
		},
		{
			name:    "StandardHAInstance",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					Region:       region,
					Tier:         redisv1pb.Instance_STANDARD_HA.String(),
					MemorySizeGB: memorySizeGB,
				},
			},
			want: &redisv1pb.CreateInstanceRequest{
				Parent:     parent,
				InstanceId: instanceName,
				Instance: &redisv1pb.Instance{
					Tier:         redisv1pb.Instance_STANDARD_HA,
					MemorySizeGb: memorySizeGB,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := NewInstanceID(tc.project, tc.i)
			got := NewCreateInstanceRequest(id, tc.i)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewCreateInstanceRequest(...): -want, +got:\n%v", diff)
			}
		})
	}
}
func TestNewUpdateInstanceRequest(t *testing.T) {
	cases := []struct {
		name    string
		project string
		i       *v1alpha1.CloudMemorystoreInstance
		want    *redisv1pb.UpdateInstanceRequest
	}{
		{
			name:    "UpdatableFieldsOnly",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					Region:       region,
					RedisConfigs: redisConfigs,
					MemorySizeGB: memorySizeGB,
				},
				Status: v1alpha1.CloudMemorystoreInstanceStatus{InstanceName: instanceName},
			},
			want: &redisv1pb.UpdateInstanceRequest{
				UpdateMask: updateMask,
				Instance: &redisv1pb.Instance{
					Name:         qualifiedName,
					RedisConfigs: redisConfigs,
					MemorySizeGb: memorySizeGB,
				},
			},
		},
		{
			name:    "SuperfluousFields",
			project: project,
			i: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					Region:       region,
					MemorySizeGB: memorySizeGB,

					// This field cannot be updated and should be omitted.
					AuthorizedNetwork: authorizedNetwork,
				},
				Status: v1alpha1.CloudMemorystoreInstanceStatus{InstanceName: instanceName},
			},
			want: &redisv1pb.UpdateInstanceRequest{
				UpdateMask: updateMask,
				Instance: &redisv1pb.Instance{
					Name:         qualifiedName,
					MemorySizeGb: memorySizeGB,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := NewInstanceID(tc.project, tc.i)
			got := NewUpdateInstanceRequest(id, tc.i)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewUpdateInstanceRequest(...): -want, +got:\n%v", diff)
			}
		})
	}
}

func TestNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.CloudMemorystoreInstance
		gcp  *redisv1pb.Instance
		want bool
	}{
		{
			name: "NeedsLessMemory",
			kube: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					RedisConfigs: redisConfigs,
					MemorySizeGB: memorySizeGB,
				},
			},
			gcp:  &redisv1pb.Instance{MemorySizeGb: memorySizeGB + 1},
			want: true,
		},
		{
			name: "NeedsNewRedisConfigs",
			kube: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					RedisConfigs: redisConfigs,
				},
			},
			gcp:  &redisv1pb.Instance{RedisConfigs: map[string]string{"super": "cool"}},
			want: true,
		},
		{
			name: "NeedsNoUpdate",
			kube: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					RedisConfigs: redisConfigs,
					MemorySizeGB: memorySizeGB,
				},
			},
			gcp: &redisv1pb.Instance{
				RedisConfigs: redisConfigs,
				MemorySizeGb: memorySizeGB,
			},
			want: false,
		},
		{
			name: "CannotUpdateField",
			kube: &v1alpha1.CloudMemorystoreInstance{
				Spec: v1alpha1.CloudMemorystoreInstanceSpec{
					MemorySizeGB: memorySizeGB,

					// We can't change this field without destroying and recreating
					// the instance so it does not register as needing an update.
					AuthorizedNetwork: "wat",
				},
			},
			gcp: &redisv1pb.Instance{
				MemorySizeGb:      memorySizeGB,
				AuthorizedNetwork: authorizedNetwork,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsUpdate(tc.kube, tc.gcp)
			if got != tc.want {
				t.Errorf("NeedsUpdate(...): want: %t got: %t", tc.want, got)
			}
		})
	}
}

func TestNewDeleteInstanceRequest(t *testing.T) {
	cases := []struct {
		name    string
		project string
		id      InstanceID
		want    *redisv1pb.DeleteInstanceRequest
	}{
		{
			name:    "DeleteInstance",
			project: project,
			id:      InstanceID{Project: project, Region: region, Instance: instanceName},
			want:    &redisv1pb.DeleteInstanceRequest{Name: qualifiedName},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewDeleteInstanceRequest(tc.id)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewDeleteInstanceRequest(...): -want, +got:\n%v", diff)
			}
		})
	}
}

func TestNewGetInstanceRequest(t *testing.T) {
	cases := []struct {
		name    string
		project string
		id      InstanceID
		want    *redisv1pb.GetInstanceRequest
	}{
		{
			name:    "GetInstance",
			project: project,
			id:      InstanceID{Project: project, Region: region, Instance: instanceName},
			want:    &redisv1pb.GetInstanceRequest{Name: qualifiedName},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewGetInstanceRequest(tc.id)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewGetInstanceRequest(...): -want, +got:\n%v", diff)
			}
		})
	}
}
