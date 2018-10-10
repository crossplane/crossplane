/*
Copyright 2018 The Conductor Authors.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageRDSInstance(t *testing.T) {
	g := NewGomegaWithT(t)

	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &RDSInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	fetched := &RDSInstance{}

	// Test Create
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestStorageRDSInstanceClass(t *testing.T) {
	g := NewGomegaWithT(t)

	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &RDSInstanceClass{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	fetched := &RDSInstanceClass{}

	// Test Create
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}

func TestStorageRDSInstanceClaim(t *testing.T) {
	g := NewGomegaWithT(t)

	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &RDSInstanceClass{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	fetched := &RDSInstanceClass{}

	// Test Create
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(ctx, updated)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(updated))

	// Test Delete
	g.Expect(c.Delete(ctx, fetched)).NotTo(HaveOccurred())
	g.Expect(c.Get(ctx, key, fetched)).To(HaveOccurred())
}
