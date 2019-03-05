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
	"log"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	ctx = context.TODO()
	cfg *rest.Config
	c   client.Client

	key = types.NamespacedName{Name: name, Namespace: namespace}
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

func TestKubernetes(t *testing.T) {
	g := NewGomegaWithT(t)

	created := &KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: KubernetesClusterSpec{
			ClassRef: &corev1.ObjectReference{
				Name:      "test-class",
				Namespace: "test-system",
			},
		},
	}

	// Test Create
	fetched := &KubernetesCluster{}
	g.Expect(c.Create(ctx, created)).NotTo(HaveOccurred())

	g.Expect(c.Get(ctx, key, fetched)).NotTo(HaveOccurred())
	g.Expect(fetched).To(Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	updated.Spec.ResourceRef = &corev1.ObjectReference{
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

func TestWorkload(t *testing.T) {
	g := NewGomegaWithT(t)

	om := metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	}

	created := &Workload{
		ObjectMeta: om,
		Spec: WorkloadSpec{
			TargetNamespace: namespace,
			TargetDeployment: &appsv1.Deployment{
				ObjectMeta: om,
			},
			TargetService: &corev1.Service{
				ObjectMeta: om,
			},
			Resources: []ResourceReference{
				{
					ObjectReference: corev1.ObjectReference{Name: "mysql-database"},
					SecretName:      "mysql-database-creds",
				},
				{
					ObjectReference: corev1.ObjectReference{Name: "my-bucket"},
				},
			},
		},
	}

	// Test Create
	fetched := &Workload{}
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
