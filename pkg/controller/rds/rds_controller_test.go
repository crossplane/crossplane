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

package rds

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/onsi/gomega"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	awsclients "github.com/upbound/conductor/pkg/clients/aws"
	"golang.org/x/net/context"
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

type mockEC2Client struct {
	awsclients.EC2API
}

type mockRDSClient struct {
	awsclients.RDSAPI
}

func (m *mockRDSClient) DescribeDBInstances(input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	id := *input.DBInstanceIdentifier
	arn := "fooARN"
	status := "created"
	return &rds.DescribeDBInstancesOutput{
		DBInstances: []rds.DBInstance{
			{
				DBInstanceIdentifier: &id,
				DBInstanceArn:        &arn,
				DBInstanceStatus:     &status,
			},
		},
	}, nil
}

func (m *mockRDSClient) WaitUntilDBInstanceAvailable(input *rds.DescribeDBInstancesInput) error {
	return nil
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &awsv1alpha1.RDS{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

	clientset := fake.NewSimpleClientset()
	ec2Client := &mockEC2Client{}
	rdsClient := &mockRDSClient{}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	reconciler := newReconciler(mgr, clientset, ec2Client, rdsClient)

	recFn, requests := SetupTestReconcile(reconciler)
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the RDS object, defer its clean up, and wait for the Reconcile to run
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// wait on a 2nd reconcile request that is caused by the first reconcile updating the CRD status
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// verify that the CRD status was updated with details about the external RDS instance
	updatedInstance := &awsv1alpha1.RDS{}
	c.Get(context.TODO(), expectedRequest.NamespacedName, updatedInstance)
	expectedStatus := awsv1alpha1.RDSStatus{
		Message:    "RDS instance foo exists: [foo, fooARN]",
		State:      "created",
		ProviderID: "fooARN",
	}
	g.Expect(updatedInstance.Status).To(gomega.Equal(expectedStatus))
}
