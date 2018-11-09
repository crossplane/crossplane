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

package mysql

import (
	"testing"

	. "github.com/onsi/gomega"
	azuredbv1alpha1 "github.com/upbound/conductor/pkg/apis/azure/database/v1alpha1"
	mysqlv1alpha1 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
)

func TestTranslateValuesToAzureMySQLServer(t *testing.T) {
	g := NewGomegaWithT(t)

	// no value set on the abstract spec, no error returned and existing value on concrete spec should be maintained
	instanceSpec := mysqlv1alpha1.MySQLInstanceSpec{}
	mysqlServerSpec := &azuredbv1alpha1.MysqlServerSpec{Version: "5.6"}
	err := translateValuesToAzureMySQLServer(instanceSpec, mysqlServerSpec)
	g.Expect(err).NotTo(HaveOccurred())
	expectedMysqlServerSpec := &azuredbv1alpha1.MysqlServerSpec{Version: "5.6"}
	g.Expect(expectedMysqlServerSpec).To(Equal(mysqlServerSpec))

	// valid value on the abstract spec, no error returned and new value should be set on concrete spec
	instanceSpec = mysqlv1alpha1.MySQLInstanceSpec{EngineVersion: "5.7"}
	mysqlServerSpec = &azuredbv1alpha1.MysqlServerSpec{Version: "5.6"}
	err = translateValuesToAzureMySQLServer(instanceSpec, mysqlServerSpec)
	g.Expect(err).NotTo(HaveOccurred())
	expectedMysqlServerSpec = &azuredbv1alpha1.MysqlServerSpec{Version: "5.7"}
	g.Expect(expectedMysqlServerSpec).To(Equal(mysqlServerSpec))

	// invalid value on the abstract spec, should return error
	instanceSpec = mysqlv1alpha1.MySQLInstanceSpec{EngineVersion: "badVersion"}
	mysqlServerSpec = &azuredbv1alpha1.MysqlServerSpec{}
	err = translateValuesToAzureMySQLServer(instanceSpec, mysqlServerSpec)
	g.Expect(err).To(HaveOccurred())
}
