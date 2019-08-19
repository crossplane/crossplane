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

package redis

import (
	"testing"

	redismgmt "github.com/Azure/azure-sdk-for-go/services/redis/mgmt/2018-03-01/redis"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/azure/apis/cache/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

const (
	uid              = types.UID("definitely-a-uuid")
	resourceName     = NamePrefix + "-" + string(uid)
	enableNonSSLPort = true
	subnetID         = "coolsubnet"
	staticIP         = "172.16.0.1"
	shardCount       = 3
	skuName          = v1alpha1.SKUNameBasic
	skuFamily        = v1alpha1.SKUFamilyC
	skuCapacity      = 1
)

var redisConfiguration = map[string]string{"cool": "socool"}

func TestNewResourceName(t *testing.T) {
	cases := []struct {
		name string
		o    metav1.Object
		want string
	}{
		{
			name: "Successful",
			o:    &v1alpha1.Redis{ObjectMeta: metav1.ObjectMeta{UID: uid}},
			want: resourceName,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewResourceName(tc.o)
			if got != tc.want {
				t.Errorf("NewResourceName(...): want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestNewCreateParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *v1alpha1.Redis
		want redismgmt.CreateParameters
	}{
		{
			name: "Successful",
			r: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						SubnetID:           subnetID,
						StaticIP:           staticIP,
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			want: redismgmt.CreateParameters{
				CreateProperties: &redismgmt.CreateProperties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					SubnetID:           azure.ToStringPtr(subnetID),
					StaticIP:           azure.ToStringPtr(staticIP),
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewCreateParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewCreateParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestNewUpdateParameters(t *testing.T) {
	cases := []struct {
		name string
		r    *v1alpha1.Redis
		want redismgmt.UpdateParameters
	}{
		{
			name: "UpdatableFieldsOnly",
			r: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			want: redismgmt.UpdateParameters{
				UpdateProperties: &redismgmt.UpdateProperties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
		},
		{
			name: "SuperfluousFields",
			r: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						SubnetID:           subnetID,
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,

						// These fields cannot be updated and should be omitted.
						StaticIP:   staticIP,
						ShardCount: shardCount,
					},
				},
			},
			want: redismgmt.UpdateParameters{
				UpdateProperties: &redismgmt.UpdateProperties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewUpdateParameters(tc.r)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("NewUpdateParameters(...): -want, +got\n%s", diff)
			}
		})
	}
}

func TestNeedsUpdate(t *testing.T) {
	cases := []struct {
		name string
		kube *v1alpha1.Redis
		az   redismgmt.ResourceType
		want bool
	}{
		{
			name: "NeedsLessCapacity",
			kube: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			az: redismgmt.ResourceType{
				Properties: &redismgmt.Properties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity + 1),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
			want: true,
		},
		{
			name: "NeedsNewRedisConfiguration",
			kube: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			az: redismgmt.ResourceType{
				Properties: &redismgmt.Properties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(map[string]string{"super": "cool"}),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
			want: true,
		},
		{
			name: "NeedsSSLPortDisabled",
			kube: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			az: redismgmt.ResourceType{
				Properties: &redismgmt.Properties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(!enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
			want: true,
		},
		{
			name: "NeedsFewerShards",
			kube: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			az: redismgmt.ResourceType{
				Properties: &redismgmt.Properties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount + 1),
				},
			},
			want: true,
		},
		{
			name: "NeedsNoUpdate",
			kube: &v1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{UID: uid},
				Spec: v1alpha1.RedisSpec{
					RedisParameters: v1alpha1.RedisParameters{
						SKU: v1alpha1.SKUSpec{
							Name:     skuName,
							Family:   skuFamily,
							Capacity: skuCapacity,
						},
						EnableNonSSLPort:   enableNonSSLPort,
						RedisConfiguration: redisConfiguration,
						ShardCount:         shardCount,
					},
				},
			},
			az: redismgmt.ResourceType{
				Properties: &redismgmt.Properties{
					Sku: &redismgmt.Sku{
						Name:     redismgmt.SkuName(skuName),
						Family:   redismgmt.SkuFamily(skuFamily),
						Capacity: azure.ToInt32Ptr(skuCapacity),
					},
					EnableNonSslPort:   azure.ToBoolPtr(enableNonSSLPort),
					RedisConfiguration: azure.ToStringPtrMap(redisConfiguration),
					ShardCount:         azure.ToInt32Ptr(shardCount),
				},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsUpdate(tc.kube, tc.az)
			if got != tc.want {
				t.Errorf("NeedsUpdate(...): want %t, got %t", tc.want, got)
			}
		})
	}
}
