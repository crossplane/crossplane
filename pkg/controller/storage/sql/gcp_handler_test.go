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

package sql

import (
	"testing"

	gcpdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	. "github.com/onsi/gomega"
)

func TestResolveGCPClassInstanceValues(t *testing.T) {
	g := NewGomegaWithT(t)

	// no class or instance values set: no error and no resolved value
	cloudsqlInstanceSpec := gcpdbv1alpha1.NewCloudSQLInstanceSpec(map[string]string{})
	mysqlInstance := &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: ""}}
	err := resolveGCPClassInstanceValues(cloudsqlInstanceSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cloudsqlInstanceSpec.DatabaseVersion).To(Equal(""))

	// class parameter set, instance value not set.  class parameter should be honored
	cloudsqlInstanceSpec = gcpdbv1alpha1.NewCloudSQLInstanceSpec(map[string]string{"databaseVersion": "POSTGRES_9_6"})
	postgresInstance := &storagev1alpha1.PostgreSQLInstance{Spec: storagev1alpha1.PostgreSQLInstanceSpec{EngineVersion: ""}}
	err = resolveGCPClassInstanceValues(cloudsqlInstanceSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cloudsqlInstanceSpec.DatabaseVersion).To(Equal("POSTGRES_9_6"))

	// class parameter not set, instance value set.  translated instance value should be honored
	cloudsqlInstanceSpec = gcpdbv1alpha1.NewCloudSQLInstanceSpec(map[string]string{})
	postgresInstance = &storagev1alpha1.PostgreSQLInstance{Spec: storagev1alpha1.PostgreSQLInstanceSpec{EngineVersion: "9.6"}}
	err = resolveGCPClassInstanceValues(cloudsqlInstanceSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cloudsqlInstanceSpec.DatabaseVersion).To(Equal("POSTGRES_9_6"))

	// class parameter and instance value both set and in agreement. should be honored.
	cloudsqlInstanceSpec = gcpdbv1alpha1.NewCloudSQLInstanceSpec(map[string]string{"databaseVersion": "MYSQL_5_6"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveGCPClassInstanceValues(cloudsqlInstanceSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cloudsqlInstanceSpec.DatabaseVersion).To(Equal("MYSQL_5_6"))

	// class parameter and instance value both set to conflicting values, should be an error.
	cloudsqlInstanceSpec = gcpdbv1alpha1.NewCloudSQLInstanceSpec(map[string]string{"databaseVersion": "MYSQL_5_7"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveGCPClassInstanceValues(cloudsqlInstanceSpec, mysqlInstance)
	g.Expect(err).To(HaveOccurred())
}
