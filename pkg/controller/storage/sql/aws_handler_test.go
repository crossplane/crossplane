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

	. "github.com/onsi/gomega"

	awsdbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/database/v1alpha1"
	storagev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/storage/v1alpha1"
)

func TestResolveAWSClassInstanceValues(t *testing.T) {
	g := NewGomegaWithT(t)

	// no class or instance values set: no error and no resolved value (except for engine because
	// that can be inferred from the abstract type)
	rdsInstanceSpec := awsdbv1alpha1.NewRDSInstanceSpec(map[string]string{})
	mysqlInstance := &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: ""}}
	err := resolveAWSClassInstanceValues(rdsInstanceSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rdsInstanceSpec.EngineVersion).To(Equal(""))
	g.Expect(rdsInstanceSpec.Engine).To(Equal(awsdbv1alpha1.MysqlEngine))

	// class parameter set, instance value not set.  class parameter should be honored
	rdsInstanceSpec = awsdbv1alpha1.NewRDSInstanceSpec(map[string]string{"engineVersion": "9.6.9"})
	postgresInstance := &storagev1alpha1.PostgreSQLInstance{Spec: storagev1alpha1.PostgreSQLInstanceSpec{EngineVersion: ""}}
	err = resolveAWSClassInstanceValues(rdsInstanceSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rdsInstanceSpec.EngineVersion).To(Equal("9.6.9"))
	g.Expect(rdsInstanceSpec.Engine).To(Equal(awsdbv1alpha1.PostgresqlEngine))

	// class parameter not set, instance value set.  instance value should be honored
	rdsInstanceSpec = awsdbv1alpha1.NewRDSInstanceSpec(map[string]string{})
	postgresInstance = &storagev1alpha1.PostgreSQLInstance{Spec: storagev1alpha1.PostgreSQLInstanceSpec{EngineVersion: "9.6.9"}}
	err = resolveAWSClassInstanceValues(rdsInstanceSpec, postgresInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rdsInstanceSpec.EngineVersion).To(Equal("9.6.9"))
	g.Expect(rdsInstanceSpec.Engine).To(Equal(awsdbv1alpha1.PostgresqlEngine))

	// class parameter and instance value both set and in agreement. should be honored.
	rdsInstanceSpec = awsdbv1alpha1.NewRDSInstanceSpec(map[string]string{"engineVersion": "5.6.45"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveAWSClassInstanceValues(rdsInstanceSpec, mysqlInstance)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rdsInstanceSpec.EngineVersion).To(Equal("5.6.45"))
	g.Expect(rdsInstanceSpec.Engine).To(Equal(awsdbv1alpha1.MysqlEngine))

	// class parameter and instance value both set to conflicting values, should be an error.
	rdsInstanceSpec = awsdbv1alpha1.NewRDSInstanceSpec(map[string]string{"engineVersion": "5.7.23"})
	mysqlInstance = &storagev1alpha1.MySQLInstance{Spec: storagev1alpha1.MySQLInstanceSpec{EngineVersion: "5.6"}}
	err = resolveAWSClassInstanceValues(rdsInstanceSpec, mysqlInstance)
	g.Expect(err).To(HaveOccurred())

}
