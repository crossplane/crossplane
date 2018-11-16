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

	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
	. "github.com/onsi/gomega"
)

func TestResolveAzureClassInstanceValues(t *testing.T) {
	g := NewGomegaWithT(t)

	// no class or instance values set: no error and no resolved value
	mysqlServerSpec := azurev1alpha1.NewSQLServerSpec(map[string]string{})
	mysqlInstance := &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: ""}}
	err := resolveAzureClassInstanceValues(mysqlServerSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mysqlServerSpec.Version).To(Equal(""))

	// class parameter set, instance value not set.  class parameter should be honored
	mysqlServerSpec = azurev1alpha1.NewSQLServerSpec(map[string]string{"version": "5.6"})
	postgresInstance := &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: ""}}
	err = resolveAzureClassInstanceValues(mysqlServerSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mysqlServerSpec.Version).To(Equal("5.6"))

	// class parameter not set, instance value set.  instance value should be honored
	mysqlServerSpec = azurev1alpha1.NewSQLServerSpec(map[string]string{})
	postgresInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveAzureClassInstanceValues(mysqlServerSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mysqlServerSpec.Version).To(Equal("5.6"))

	// class parameter and instance value both set and in agreement. should be honored.
	mysqlServerSpec = azurev1alpha1.NewSQLServerSpec(map[string]string{"version": "5.6"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveAzureClassInstanceValues(mysqlServerSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(mysqlServerSpec.Version).To(Equal("5.6"))

	// class parameter and instance value both set to conflicting values, should be an error.
	mysqlServerSpec = azurev1alpha1.NewSQLServerSpec(map[string]string{"version": "5.7"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveAzureClassInstanceValues(mysqlServerSpec, mysqlInstance)
	g.Expect(err).To(HaveOccurred())
}
