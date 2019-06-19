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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	c   client.Client
	ctx = context.TODO()
)

var _ resource.ManagedResource = &Redis{}

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageRedis(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &Redis{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       RedisSpec{SKU: SKUSpec{Name: SKUNameBasic, Family: SKUFamilyC, Capacity: 0}},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

	fetched := &Redis{}
	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(gomega.HaveOccurred())
}

func TestNewRedisSpec(t *testing.T) {
	cases := []struct {
		name       string
		properties map[string]string
		want       *RedisSpec
	}{
		{
			name: "AllProperties",
			properties: map[string]string{
				"resourceGroupName":  "coolResourceGroup",
				"location":           "Australia East",
				"staticIP":           "172.16.0.1",
				"subnetId":           "/subscriptions/subid/resourceGroups/coolResourceGroup/providers/Microsoft.Network/virtualNetworks/coolNetwork/subnets/coolSubnet",
				"enableNonSslPort":   "true",
				"shardCount":         "3",
				"redisConfiguration": "maxmemory-policy: lots, maxclients: 800",
				"skuName":            SKUNameBasic,
				"skuFamily":          SKUFamilyC,
				"skuCapacity":        "4",
			},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				ResourceGroupName: "coolResourceGroup",
				Location:          "Australia East",
				StaticIP:          "172.16.0.1",
				SubnetID:          "/subscriptions/subid/resourceGroups/coolResourceGroup/providers/Microsoft.Network/virtualNetworks/coolNetwork/subnets/coolSubnet",
				EnableNonSSLPort:  true,
				ShardCount:        3,
				RedisConfiguration: map[string]string{
					"maxmemory-policy": "lots",
					"maxclients":       "800",
				},
				SKU: SKUSpec{Name: SKUNameBasic, Family: SKUFamilyC, Capacity: 4},
			},
		},
		{
			name:       "NilProperties",
			properties: nil,
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "UnknownProperties",
			properties: map[string]string{"unknown": "wat"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "EnableNonSSLPortNotABool",
			properties: map[string]string{"enableNonSslPort": "maybe"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "ShardCountNotANumber",
			properties: map[string]string{"shardCount": "wat"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "SKUCapacityNotANumber",
			properties: map[string]string{"skuCapacity": "wat"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "RedisConfigurationUnparseable",
			properties: map[string]string{"redisConfiguration": "wat,wat"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{},
			},
		},
		{
			name:       "RedisConfigurationExtraneousWhitespace",
			properties: map[string]string{"redisConfiguration": "   verykey:suchvalue"},
			want: &RedisSpec{
				ResourceSpec: corev1alpha1.ResourceSpec{
					ReclaimPolicy: corev1alpha1.ReclaimRetain,
				},
				RedisConfiguration: map[string]string{"verykey": "suchvalue"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewRedisSpec(tc.properties)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("got != want:\n%v", diff)
			}
		})
	}
}
