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
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace = "default"
	name      = "test-instance"
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

func TestStorageMysqlServer(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &MysqlServer{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
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

func TestNewMySQLServerSpec(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	m := make(map[string]string)
	exp := &MysqlServerSpec{ReclaimPolicy: corev1alpha1.ReclaimRetain}

	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val := "admin"
	m["adminLoginName"] = val
	exp.AdminLoginName = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "rg1"
	m["resourceGroupName"] = val
	exp.ResourceGroupName = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "loc1"
	m["location"] = val
	exp.Location = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "5.6"
	m["version"] = val
	exp.Version = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "true"
	m["sslEnforced"] = val
	exp.SSLEnforced = true
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
	// invalid sslEnforced value
	val = "not a bool"
	m["sslEnforced"] = val
	exp.SSLEnforced = false // value is not set
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "Basic"
	m["tier"] = val
	exp.PricingTier.Tier = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "4"
	m["vcores"] = val
	exp.PricingTier.VCores = 4
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
	// invalid vcores value
	val = "not a number"
	m["vcores"] = val
	exp.PricingTier.VCores = 0 // value is not set
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "Gen4"
	m["family"] = val
	exp.PricingTier.Family = val
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "100"
	m["storageGB"] = val
	exp.StorageProfile.StorageGB = 100
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
	// invalid storageGB value
	val = "not a number"
	m["storageGB"] = val
	exp.StorageProfile.StorageGB = 0 // value is not set
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "14"
	m["backupRetentionDays"] = val
	exp.StorageProfile.BackupRetentionDays = 14
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
	// invalid backupRetentionDays value
	val = "not a number"
	m["backupRetentionDays"] = val
	exp.StorageProfile.BackupRetentionDays = 0 // value is not set
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))

	val = "true"
	m["geoRedundantBackup"] = val
	exp.StorageProfile.GeoRedundantBackup = true
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
	// invalid geoRedundantBackup value
	val = "not a bool"
	m["geoRedundantBackup"] = val
	exp.StorageProfile.GeoRedundantBackup = false // value is not set
	g.Expect(NewMySQLServerSpec(m)).To(gomega.Equal(exp))
}
