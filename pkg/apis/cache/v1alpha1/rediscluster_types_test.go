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

package v1alpha1

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

var _ resource.Claim = &RedisCluster{}

func TestRedisClusterStorage(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	created := &RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: RedisClusterSpec{
			ResourceClaimSpec: v1alpha1.ResourceClaimSpec{
				ClassReference: &corev1.ObjectReference{
					Name:      "test-class",
					Namespace: "test-system",
				},
			},
			EngineVersion: "3.2",
		},
	}

	// Test Create
	fetched := &RedisCluster{}
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updated.Spec.ResourceReference = &corev1.ObjectReference{
		Name:      "test-class",
		Namespace: "test-resource",
	}
	g.Expect(c.Update(ctx, updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(gomega.HaveOccurred())
}

func TestEngineVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		valid   bool
	}{
		{
			name:    "ValidVersion",
			version: "3.2",
			valid:   true,
		},
		{
			name:    "InvalidVersion",
			version: "0.0",
			valid:   false,
		},
		{
			name:    "PatchVersionIsInvalid",
			version: "3.2.1",
			valid:   false,
		},
		{
			name:    "EmptyVersionIsInvalid",
			version: "",
			valid:   false,
		},
	}

	g := gomega.NewGomegaWithT(t)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created := &RedisCluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: RedisClusterSpec{
					ResourceClaimSpec: v1alpha1.ResourceClaimSpec{
						ClassReference: &corev1.ObjectReference{
							Name:      "test-class",
							Namespace: "test-system",
						},
					},
					EngineVersion: tc.version,
				},
			}

			fetched := &RedisCluster{}

			if !tc.valid {
				g.Expect(c.Create(ctx, created)).To(gomega.HaveOccurred())
				return
			}
			g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())
			g.Expect(c.Get(ctx, key, fetched)).NotTo(gomega.HaveOccurred())
			g.Expect(fetched).To(gomega.Equal(created))
			g.Expect(c.Delete(ctx, fetched)).NotTo(gomega.HaveOccurred())
			g.Expect(c.Get(ctx, key, fetched)).To(gomega.HaveOccurred())
		})
	}
}
