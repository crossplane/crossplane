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
	gcpdbv1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/database/v1alpha1"
	mysqlv1alpha1 "github.com/upbound/conductor/pkg/apis/storage/v1alpha1"
)

func TestTranslateToCloudSQL(t *testing.T) {
	g := NewGomegaWithT(t)

	// no value set on the abstract spec, no error returned and existing value on concrete spec should be maintained
	instanceSpec := mysqlv1alpha1.MySQLInstanceSpec{}
	cloudsqlSpec := &gcpdbv1alpha1.CloudsqlInstanceSpec{DatabaseVersion: "MYSQL_5_6"}
	err := translateToCloudSQL(instanceSpec, cloudsqlSpec)
	g.Expect(err).NotTo(HaveOccurred())
	expectedCloudsqlInstanceSpec := &gcpdbv1alpha1.CloudsqlInstanceSpec{DatabaseVersion: "MYSQL_5_6"}
	g.Expect(expectedCloudsqlInstanceSpec).To(Equal(cloudsqlSpec))

	// valid value on the abstract spec, no error returned and new (translated) value should be set on concrete spec
	instanceSpec = mysqlv1alpha1.MySQLInstanceSpec{EngineVersion: "5.7"}
	cloudsqlSpec = &gcpdbv1alpha1.CloudsqlInstanceSpec{DatabaseVersion: "MYSQL_5_6"}
	err = translateToCloudSQL(instanceSpec, cloudsqlSpec)
	g.Expect(err).NotTo(HaveOccurred())
	expectedCloudsqlInstanceSpec = &gcpdbv1alpha1.CloudsqlInstanceSpec{DatabaseVersion: "MYSQL_5_7"}
	g.Expect(expectedCloudsqlInstanceSpec).To(Equal(cloudsqlSpec))

	// invalid value on the abstract spec, should return error
	instanceSpec = mysqlv1alpha1.MySQLInstanceSpec{EngineVersion: "badVersion"}
	cloudsqlSpec = &gcpdbv1alpha1.CloudsqlInstanceSpec{}
	err = translateToCloudSQL(instanceSpec, cloudsqlSpec)
	g.Expect(err).To(HaveOccurred())
}
