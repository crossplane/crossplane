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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/resource"
)

var (
	_ resource.Claim = &MySQLInstance{}
	_ resource.Claim = &PostgreSQLInstance{}
	_ resource.Claim = &Bucket{}
)

func TestMySQLInstanceStorage(t *testing.T) {
	g := NewGomegaWithT(t)

	created := &MySQLInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: MySQLInstanceSpec{
			ResourceClaimSpec: v1alpha1.ResourceClaimSpec{
				ClassReference: &corev1.ObjectReference{
					Name:      "test-class",
					Namespace: "test-system",
				},
			},
			EngineVersion: "5.6",
		},
	}

	// Test Create
	fetched := &MySQLInstance{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updated.Spec.ResourceReference = &corev1.ObjectReference{
		Name:      "test-class",
		Namespace: "test-resource",
	}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestPostgreSQLInstanceStorage(t *testing.T) {
	g := NewGomegaWithT(t)

	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &PostgreSQLInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: PostgreSQLInstanceSpec{
			ResourceClaimSpec: v1alpha1.ResourceClaimSpec{
				ClassReference: &corev1.ObjectReference{
					Name:      "test-class",
					Namespace: "test-system",
				},
			},
		},
	}

	// Test Create
	fetched := &PostgreSQLInstance{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updated.Spec.ResourceReference = &corev1.ObjectReference{
		Name:      "test-class",
		Namespace: "test-resource",
	}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestEngineVersion(t *testing.T) {
	g := NewGomegaWithT(t)

	validate := func(version string, expectedValid bool) {
		created := &MySQLInstance{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: MySQLInstanceSpec{
				ResourceClaimSpec: v1alpha1.ResourceClaimSpec{
					ClassReference: &corev1.ObjectReference{
						Name:      "test-class",
						Namespace: "test-system",
					},
				},
				EngineVersion: version,
			},
		}

		fetched := &MySQLInstance{}

		if expectedValid {
			g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())
			g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
			g.Expect(fetched).To(Equal(created))
			g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
			g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
		} else {
			g.Expect(c.Create(ctx, created)).To(HaveOccurred())
		}
	}

	// Test Create: valid versions
	validate("5.6", true)
	validate("5.7", true)

	// Test Create: invalid versions
	validate("", false)
	validate("5.8", false)
	validate("5.6.40", false)
}
