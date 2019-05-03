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

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, test.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageCloudMemorystoreInstance(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &CloudMemorystoreInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       CloudMemorystoreInstanceSpec{Tier: TierBasic},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

	fetched := &CloudMemorystoreInstance{}
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

func TestNewCloudMemorystoreInstanceSpec(t *testing.T) {
	cases := []struct {
		name       string
		properties map[string]string
		want       *CloudMemorystoreInstanceSpec
	}{
		{
			name: "AllProperties",
			properties: map[string]string{
				"tier":                  TierBasic,
				"region":                "au-east1",
				"locationId":            "au-east1-a",
				"alternativeLocationId": "au-east1-b",
				"reservedIpRange":       "172.16.0.0/29",
				"authorizedNetwork":     "default",
				"memorySizeGb":          "4",
				"redisVersion":          "REDIS_3_2",
				"redisConfigs":          "max-memory-policy: lots, notify-keyspace-events: surewhynot",
			},
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy:         corev1alpha1.ReclaimRetain,
				Tier:                  TierBasic,
				Region:                "au-east1",
				LocationID:            "au-east1-a",
				AlternativeLocationID: "au-east1-b",
				ReservedIPRange:       "172.16.0.0/29",
				AuthorizedNetwork:     "default",
				MemorySizeGB:          4,
				RedisVersion:          "REDIS_3_2",
				RedisConfigs: map[string]string{
					"max-memory-policy":      "lots",
					"notify-keyspace-events": "surewhynot",
				},
			},
		},
		{
			name:       "NilProperties",
			properties: nil,
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
				RedisConfigs:  map[string]string{},
			},
		},
		{
			name:       "UnknownProperties",
			properties: map[string]string{"unknown": "wat"},
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
				RedisConfigs:  map[string]string{},
			},
		},
		{
			name:       "MemorySizeGbNotANumber",
			properties: map[string]string{"memorySizeGb": "wat"},
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
				RedisConfigs:  map[string]string{},
				MemorySizeGB:  0,
			},
		},
		{
			name:       "RedisConfigsUnparseable",
			properties: map[string]string{"redisConfigs": "wat,wat"},
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
				RedisConfigs:  map[string]string{},
			},
		},
		{
			name:       "RedisConfigsExtraneousWhitespace",
			properties: map[string]string{"redisConfigs": "   verykey:suchvalue"},
			want: &CloudMemorystoreInstanceSpec{
				ReclaimPolicy: corev1alpha1.ReclaimRetain,
				RedisConfigs:  map[string]string{"verykey": "suchvalue"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NewCloudMemorystoreInstanceSpec(tc.properties)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("got != want:\n%v", diff)
			}
		})
	}
}
