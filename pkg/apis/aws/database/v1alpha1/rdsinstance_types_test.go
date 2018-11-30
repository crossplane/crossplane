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

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorage(t *testing.T) {
	g := NewGomegaWithT(t)

	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &RDSInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}

	// Test Create
	fetched := &RDSInstance{}
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

func TestNewRDSInstanceSpec(t *testing.T) {
	g := NewGomegaWithT(t)

	m := make(map[string]string)
	exp := &RDSInstanceSpec{ReclaimPolicy: corev1alpha1.ReclaimRetain}

	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val := "master"
	m["masterUsername"] = val
	exp.MasterUsername = val
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val = "password"
	m["password"] = val
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val = "5.7"
	m["engineVersion"] = val
	exp.EngineVersion = val
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val = "100"
	m["size"] = val
	exp.Size = int64(100)
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))
	// invalid size value
	val = "100ab"
	m["size"] = val
	exp.Size = int64(0) // value is not set
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val = "one,two,tree"
	m["securityGroups"] = val
	exp.SecurityGroups = []string{"one", "two", "tree"}
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))

	val = "test-subnetgroup"
	m["subnetGroupName"] = val
	exp.SubnetGroupName = val
	g.Expect(NewRDSInstanceSpec(m)).To(Equal(exp))
}

func TestIsAvailable(t *testing.T) {
	g := NewGomegaWithT(t)
	r := &RDSInstance{}
	g.Expect(r.IsAvailable()).To(BeFalse())

	r.Status.State = "foo"
	g.Expect(r.IsAvailable()).To(BeFalse())

	r.Status.State = string(RDSInstanceStateAvailable)
	g.Expect(r.IsAvailable()).To(BeTrue())
}
