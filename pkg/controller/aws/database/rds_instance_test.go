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

package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/onsi/gomega"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	k8sclients "github.com/upbound/conductor/pkg/clients/kubernetes"
	awstests "github.com/upbound/conductor/tests/aws"
	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// setup the RDS instance CRD that will be used for this test
	instance := &databasev1alpha1.RDSInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: databasev1alpha1.RDSInstanceSpec{
			MasterPassword: v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{Name: "passwordSecretName"},
				Key:                  "passwordSecretKey",
			},
			PubliclyAccessible: true,
		},
	}

	// mock a Kubernetes clientset that knows about the k8s resources for this test
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node1",
			Labels: map[string]string{k8sclients.NodeRegionLabel: "region1"},
		},
		Spec: v1.NodeSpec{ProviderID: "aws-providerid-node1"},
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: instance.Spec.MasterPassword.Name, Namespace: "default"},
		Data:       map[string][]byte{instance.Spec.MasterPassword.Key: []byte("secretdata")},
	}
	clientset := fake.NewSimpleClientset(node, secret)

	// mock an EC2 client
	ec2Client := &awstests.EC2Client{}
	ec2Client.MockDescribeInstances = func(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
		vpcID := "vpc1"
		sgID := "sg1"
		return &ec2.DescribeInstancesOutput{Reservations: []ec2.RunInstancesOutput{
			{
				Instances: []ec2.Instance{
					{
						VpcId:          &vpcID,
						SecurityGroups: []ec2.GroupIdentifier{{GroupId: &sgID}},
					},
				}}},
		}, nil
	}
	ec2Client.MockDescribeSubnets = func(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
		subnetID := "subnet1"
		return &ec2.DescribeSubnetsOutput{
			Subnets: []ec2.Subnet{{SubnetId: &subnetID, MapPublicIpOnLaunch: &instance.Spec.PubliclyAccessible}},
		}, nil
	}

	// mock a RDS client
	rdsClient := &mockRDSClient{}
	createInstanceCalled := false
	createDBSubnetGroupCalled := false
	rdsClient.MockDescribeDBInstances = func(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
		if !createInstanceCalled {
			// until CreateDBInstance has been called, DescribeDBInstances should return "not found",
			// causing the Reconcile loop to try to create it
			return nil, fmt.Errorf(rds.ErrCodeDBInstanceNotFoundFault)
		}

		// CreateDBInstance has been called, DescribeDBInstances should indicate the instance has been created
		id := input.DBInstanceIdentifier
		arn := fmt.Sprintf("%s-ARN", *id)
		status := "created"
		return &rds.DescribeDBInstancesOutput{
			DBInstances: []rds.DBInstance{
				{DBInstanceIdentifier: id, DBInstanceArn: &arn, DBInstanceStatus: &status},
			},
		}, nil
	}
	rdsClient.MockCreateDBInstance = func(*rds.CreateDBInstanceInput) (*rds.CreateDBInstanceOutput, error) {
		createInstanceCalled = true
		return &rds.CreateDBInstanceOutput{}, nil
	}
	rdsClient.MockDescribeDBSecurityGroups = func(input *rds.DescribeDBSecurityGroupsInput) (*rds.DescribeDBSecurityGroupsOutput, error) {
		if !createDBSubnetGroupCalled {
			return nil, fmt.Errorf(rds.ErrCodeDBSecurityGroupNotFoundFault)
		}
		return &rds.DescribeDBSecurityGroupsOutput{}, nil
	}
	rdsClient.MockCreateDBSubnetGroup = func(*rds.CreateDBSubnetGroupInput) (*rds.CreateDBSubnetGroupOutput, error) {
		createDBSubnetGroupCalled = true
		return &rds.CreateDBSubnetGroupOutput{}, nil
	}

	// setup test options for the reconciler
	options := ReconcileRDSInstanceOptions{PostCreateSleepTime: 1 * time.Millisecond}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	reconciler := newRDSInstanceReconciler(mgr, clientset, ec2Client, rdsClient, options)

	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(addRDSInstanceReconciler(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the RDS object, defer its clean up, and wait for the Reconcile to run
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// wait on a 2nd reconcile request that is caused by the first reconcile updating the CRD status
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// verify that the CRD status was updated with details about the external RDS instance
	updatedInstance := &databasev1alpha1.RDSInstance{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, updatedInstance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	expectedStatus := databasev1alpha1.RDSInstanceStatus{
		Message:    "RDS instance foo exists: [foo, foo-ARN]",
		State:      "created",
		ProviderID: "foo-ARN",
	}
	g.Expect(updatedInstance.Status).To(gomega.Equal(expectedStatus))
}
