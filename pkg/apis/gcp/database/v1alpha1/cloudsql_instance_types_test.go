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

	"github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestStorageCloudsqlInstance(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &CloudsqlInstance{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &CloudsqlInstance{}
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

func TestNewCloudSQLInstanceSpec(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	m := make(map[string]string)
	exp := &CloudsqlInstanceSpec{ReclaimPolicy: corev1alpha1.ReclaimRetain}

	g.Expect(NewCloudSQLInstanceSpec(m)).To(gomega.Equal(exp))

	val := "db-n1-standard-1"
	m["tier"] = val
	exp.Tier = val
	g.Expect(NewCloudSQLInstanceSpec(m)).To(gomega.Equal(exp))

	val = "us-west2"
	m["region"] = val
	exp.Region = val
	g.Expect(NewCloudSQLInstanceSpec(m)).To(gomega.Equal(exp))

	val = "MYSQL_5_7"
	m["databaseVersion"] = val
	exp.DatabaseVersion = val
	g.Expect(NewCloudSQLInstanceSpec(m)).To(gomega.Equal(exp))

	val = "PD_SSD"
	m["storageType"] = val
	exp.StorageType = val
	g.Expect(NewCloudSQLInstanceSpec(m)).To(gomega.Equal(exp))
}
