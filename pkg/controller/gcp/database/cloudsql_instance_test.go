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
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/onsi/gomega"
	coredbv1alpha1 "github.com/upbound/conductor/pkg/apis/core/database/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/database/v1alpha1"
	"github.com/upbound/conductor/pkg/test"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	clientset := fake.NewSimpleClientset()
	cloudSQLClient := &mockCloudSQLClient{}
	cloudSQLClientFactory := &mockCloudSQLClientFactory{mockClient: cloudSQLClient}
	options := ReconcilerOptions{
		WaitSleepTime: 1 * time.Millisecond,
	}

	// Mock the GetInstance function with functionality that simulates creating a CloudSQL instance and
	// the creation operation taking a while to complete before the instance is runnable.
	getInstanceCallCount := 0
	getInstanceCallCountBeforeRunning := 5
	cloudSQLClient.MockGetInstance = func(project string, instance string) (*sqladmin.DatabaseInstance, error) {
		getInstanceCallCount++
		if getInstanceCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the instance
			return nil, &googleapi.Error{Code: http.StatusNotFound}
		} else if getInstanceCallCount >= 2 && getInstanceCallCount <= getInstanceCallCountBeforeRunning {
			// for a few GET calls, return PENDING_CREATE, simulating that the instance is in the process of
			// being created.  This will exercise the requeueing of the reconciliation loop.
			return createMockDatabaseInstance(project, instance, "PENDING_CREATE"), nil
		}
		// Finally we simulate that the create operation has completed and the instance is RUNNABLE
		return createMockDatabaseInstance(project, instance, "RUNNABLE"), nil
	}
	cloudSQLClient.MockCreateInstance = createInstanceDefault
	cloudSQLClient.MockDeleteInstance = deleteInstanceDefault
	cloudSQLClient.MockListUsers = listUsersDefault
	cloudSQLClient.MockUpdateUser = updateUserDefault
	cloudSQLClient.MockGetOperation = getOperationDefault

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	r := newCloudsqlInstanceReconciler(mgr, cloudSQLClientFactory, clientset, options)
	recFn, requests := SetupTestReconcile(r)
	g.Expect(addCloudsqlInstanceReconciler(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// create the provider object and defer its cleanup
	provider := testProvider(testSecret([]byte("testdata")))
	err = c.Create(ctx, provider)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(ctx, provider)

	// Create the CloudSQL object and defer its clean up
	instance := testInstance(provider)
	err = c.Create(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer cleanupCloudsqlInstance(g, c, requests, instance)

	// wait on the number of reconciliation requests that are caused by the instance status being PENDING_CREATE for a few GET calls
	expectedReconciliationCalls := getInstanceCallCountBeforeRunning + 1
	for i := 1; i <= expectedReconciliationCalls; i++ {
		g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	}

	// verify that the CRD status was updated with details about the external CloudSQL instance and that the
	// CRD conditions show the transition from creating to running
	expectedInstanceName := fmt.Sprintf("test-db-instance-%s", instance.UID)
	expectedStatus := databasev1alpha1.CloudsqlInstanceStatus{
		Message:      "Cloud SQL instance test-db-instance is running",
		State:        "RUNNABLE",
		ProviderID:   fmt.Sprintf("https://www.googleapis.com/sql/v1beta4/projects/%s/instances/test-db-instance-%s", providerProject, instance.UID),
		Endpoint:     fmt.Sprintf("%s:us-west2:%s", providerProject, expectedInstanceName),
		InstanceName: expectedInstanceName,
		ConditionedStatus: corev1alpha1.ConditionedStatus{
			Conditions: []corev1alpha1.Condition{
				{
					Type:    corev1alpha1.Creating,
					Status:  v1.ConditionFalse,
					Reason:  conditionStateChanged,
					Message: "cloud sql instance test-db-instance is in the Creating state",
				},
				{
					Type:    corev1alpha1.Ready,
					Status:  v1.ConditionTrue,
					Reason:  conditionStateChanged,
					Message: "cloud sql instance test-db-instance is in the Ready state",
				},
			},
		},
	}
	assertCloudsqlInstanceStatus(g, c, expectedStatus)

	// wait for the connection information to be stored in a secret, then verify it
	var connectionSecret *v1.Secret
	connectionSecretName := fmt.Sprintf(coredbv1alpha1.ConnectionSecretRefFmt, "test-db-instance")
	for {
		if connectionSecret, err = r.clientset.CoreV1().Secrets(namespace).Get(connectionSecretName, metav1.GetOptions{}); err == nil {
			break
		}
	}
	assertConnectionSecret(g, c, connectionSecret)

	// verify that a finalizer was added to the CRD
	c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(len(instance.Finalizers)).To(gomega.Equal(1))
	g.Expect(instance.Finalizers[0]).To(gomega.Equal(finalizer))

	// test deletion of the instance
	cleanupCloudsqlInstance(g, c, requests, instance)
}

func cleanupCloudsqlInstance(g *gomega.GomegaWithT, c client.Client, requests chan reconcile.Request, instance *databasev1alpha1.CloudsqlInstance) {
	deletedInstance := &databasev1alpha1.CloudsqlInstance{}
	if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
		// instance has already been deleted, bail out
		return
	}

	log.Printf("cleaning up cloud sql instance %s by deleting the CRD", instance.Name)
	err := c.Delete(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// wait for the deletion timestamp to be set
	err = wait.ExponentialBackoff(test.DefaultRetry, func() (done bool, err error) {
		deletedInstance := &databasev1alpha1.CloudsqlInstance{}
		c.Get(ctx, expectedRequest.NamespacedName, deletedInstance)
		if deletedInstance.DeletionTimestamp != nil {
			return true, nil
		}
		return false, nil
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// wait for the reconcile to happen that handles the CRD deletion
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// wait for the finalizer to run and the instance to be deleted for good
	err = wait.ExponentialBackoff(test.DefaultRetry, func() (done bool, err error) {
		deletedInstance := &databasev1alpha1.CloudsqlInstance{}
		if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
}

func assertCloudsqlInstanceStatus(g *gomega.GomegaWithT, c client.Client, expectedStatus databasev1alpha1.CloudsqlInstanceStatus) {
	instance := &databasev1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// assert the expected status properties
	g.Expect(instance.Status.Message).To(gomega.Equal(expectedStatus.Message))
	g.Expect(instance.Status.State).To(gomega.Equal(expectedStatus.State))
	g.Expect(instance.Status.ProviderID).To(gomega.Equal(expectedStatus.ProviderID))
	g.Expect(instance.Status.Endpoint).To(gomega.Equal(expectedStatus.Endpoint))
	g.Expect(instance.Status.InstanceName).To(gomega.Equal(expectedStatus.InstanceName))

	// assert the expected status conditions
	corev1alpha1.AssertConditions(g, expectedStatus.Conditions, instance.Status.ConditionedStatus)
}

func assertConnectionSecret(g *gomega.GomegaWithT, c client.Client, connectionSecret *v1.Secret) {
	instance := &databasev1alpha1.CloudsqlInstance{}
	err := c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(string(connectionSecret.Data[coredbv1alpha1.ConnectionSecretEndpointKey])).To(gomega.Equal(instance.Status.Endpoint))
	g.Expect(string(connectionSecret.Data[coredbv1alpha1.ConnectionSecretUserKey])).To(gomega.Equal("root"))
	g.Expect(string(connectionSecret.Data[coredbv1alpha1.ConnectionSecretPasswordKey])).NotTo(gomega.BeEmpty())
}
