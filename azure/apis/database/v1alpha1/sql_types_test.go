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
	"context"
	"testing"

	"github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	localtest "github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	c   client.Client
	ctx = context.TODO()
)

var (
	_ SQLServer = &MysqlServer{}
	_ SQLServer = &PostgresqlServer{}

	_ resource.Managed = &MysqlServer{}
	_ resource.Managed = &PostgresqlServer{}
)

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, SchemeBuilder.SchemeBuilder, localtest.CRDs())
	c = t.StartClient()
	t.StopAndExit(m.Run())
}

func TestStorageMysqlServer(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &MysqlServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: SQLServerSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference: &core.ObjectReference{},
			},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &MysqlServer{}
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

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

func TestStoragePostgresqlServer(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &PostgresqlServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: SQLServerSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ProviderReference: &core.ObjectReference{},
			},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &PostgresqlServer{}
	g.Expect(c.Create(ctx, created)).NotTo(gomega.HaveOccurred())

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
