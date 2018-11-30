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
	"log"
	"testing"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	name      = "test-cluster"
)

var (
	cfg *rest.Config
	c   client.Client
	ctx = context.TODO()
)

func TestMain(m *testing.M) {
	err := SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	t := test.NewTestEnv(namespace, test.CRDs())
	cfg = t.Start()

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	t.StopAndExit(m.Run())
}

func TestAKSCluster(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &AKSCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: AKSClusterSpec{
			Version:           "1.1.1",
			NodeCount:         1,
			NodeVMSize:        "Standard_B2s",
			ResourceGroupName: "rg1",
			Location:          "West US",
			DNSNamePrefix:     "conductor-aks",
			DisableRBAC:       true,
		},
	}
	g := NewGomegaWithT(t)

	// Test Create
	fetched := &AKSCluster{}
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

func TestNewAKSClusterSpec(t *testing.T) {
	g := NewGomegaWithT(t)

	m := make(map[string]string)
	exp := &AKSClusterSpec{ReclaimPolicy: corev1alpha1.ReclaimRetain, NodeCount: 1} // default values

	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val := "rg1"
	m["resourceGroupName"] = val
	exp.ResourceGroupName = val
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "loc1"
	m["location"] = val
	exp.Location = val
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "1.11.1"
	m["version"] = val
	exp.Version = val
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "4"
	m["nodeCount"] = val
	exp.NodeCount = 4
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))
	// invalid nodeCount value
	val = "not a number"
	m["nodeCount"] = val
	exp.NodeCount = 1 // value is not changed from default
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "Standard_B2s"
	m["nodeVMSize"] = val
	exp.NodeVMSize = val
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "foo"
	m["dnsNamePrefix"] = val
	exp.DNSNamePrefix = val
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))

	val = "true"
	m["disableRBAC"] = val
	exp.DisableRBAC = true
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))
	// invalid disableRBAC value
	val = "not a bool"
	m["disableRBAC"] = val
	exp.DisableRBAC = false // value is not set
	g.Expect(NewAKSClusterSpec(m)).To(Equal(exp))
}
