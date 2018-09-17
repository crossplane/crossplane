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
	"net/http"
	"testing"
	"time"

	"github.com/onsi/gomega"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/database/v1alpha1"
	"golang.org/x/net/context"
	googleapi "google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &databasev1alpha1.CloudsqlInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"},
		Spec: databasev1alpha1.CloudsqlInstanceSpec{
			ProjectID: "foo-project",
		},
	}

	cloudSQLClient := &mockCloudSQLClient{}
	options := ReconcileCloudsqlInstanceOptions{
		PostCreateSleepTime: 1 * time.Millisecond,
		WaitSleepTime:       1 * time.Millisecond,
	}

	// Mock the GetInstance function with functionality that simulates creating a CloudSQL instance and
	// the creation operation taking a while to complete before the instance is runnable.
	getInstanceCallCount := 0
	cloudSQLClient.MockGetInstance = func(project string, instance string) (*sqladmin.DatabaseInstance, error) {
		getInstanceCallCount++
		if getInstanceCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the instance
			return nil, &googleapi.Error{Code: http.StatusNotFound}
		} else if getInstanceCallCount >= 2 && getInstanceCallCount < 10 {
			// for GET calls 2-10, return PENDING_CREATE, simulating that the instance is in the process of
			// being created.  This will exercise the post create wait loop.
			return createMockDatabaseInstance(project, instance, "PENDING_CREATE"), nil
		}
		// Finally we simulate that the create operation has completed and the instance is RUNNABLE
		return createMockDatabaseInstance(project, instance, "RUNNABLE"), nil
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newCloudsqlInstanceReconciler(mgr, cloudSQLClient, options))
	g.Expect(addCloudsqlInstanceReconciler(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the CloudSQL object, defer its clean up, and wait for the Reconcile to run
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// wait on a 2nd reconcile request that is caused by the first reconcile updating the CRD status
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// verify that the CRD status was updated with details about the external CloudSQL instance
	updatedInstance := &databasev1alpha1.CloudsqlInstance{}
	c.Get(context.TODO(), expectedRequest.NamespacedName, updatedInstance)
	expectedStatus := databasev1alpha1.CloudsqlInstanceStatus{
		Message:    "Cloud SQL instance foo is running",
		State:      "RUNNABLE",
		ProviderID: "https://www.googleapis.com/sql/v1beta4/projects/foo-project/instances/foo",
	}
	g.Expect(updatedInstance.Status).To(gomega.Equal(expectedStatus))
}
