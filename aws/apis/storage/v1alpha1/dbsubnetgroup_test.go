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
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/onsi/gomega"

	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

func Test_DBSubnetGroup_BuildExternalStatusFromObservation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	r := DBSubnetGroup{}

	r.UpdateExternalStatus(rds.DBSubnetGroup{
		Subnets: []rds.Subnet{
			{
				SubnetIdentifier: aws.String("arbitrary identifier"),
			},
		},
	})

	g.Expect(r.Status.DBSubnetGroupExternalStatus).ToNot(gomega.BeNil())
}

func Test_DBSubnetGroup_BuildFromRDSTags(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	tags := BuildFromRDSTags([]rds.Tag{
		{
			Key:   aws.String("key1"),
			Value: aws.String("val1"),
		},
		{
			Key:   aws.String("key2"),
			Value: aws.String("val2"),
		},
	})

	g.Expect(len(tags)).To(gomega.Equal(2))
}
