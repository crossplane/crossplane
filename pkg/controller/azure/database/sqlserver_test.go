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

package database

import (
	"context"
	"log"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/go-autorest/autorest"
	azurerest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/test"
)

type mockSQLServerClient struct {
	MockGetServer                func(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*azureclients.SQLServer, error)
	MockCreateServerBegin        func(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error)
	MockCreateServerEnd          func(createOp []byte) (bool, error)
	MockDeleteServer             func(ctx context.Context, instance azuredbv1alpha1.SqlServer) (azurerest.Future, error)
	MockGetFirewallRule          func(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) error
	MockCreateFirewallRulesBegin func(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) ([]byte, error)
	MockCreateFirewallRulesEnd   func(createOp []byte) (bool, error)
}

func (m *mockSQLServerClient) GetServer(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*azureclients.SQLServer, error) {
	if m.MockGetServer != nil {
		return m.MockGetServer(ctx, instance)
	}
	return &azureclients.SQLServer{}, nil
}

func (m *mockSQLServerClient) CreateServerBegin(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error) {
	if m.MockCreateServerBegin != nil {
		return m.MockCreateServerBegin(ctx, instance, adminPassword)
	}
	return nil, nil
}

func (m *mockSQLServerClient) CreateServerEnd(createOp []byte) (bool, error) {
	if m.MockCreateServerEnd != nil {
		return m.MockCreateServerEnd(createOp)
	}
	return true, nil
}

func (m *mockSQLServerClient) DeleteServer(ctx context.Context, instance azuredbv1alpha1.SqlServer) (azurerest.Future, error) {
	if m.MockDeleteServer != nil {
		return m.MockDeleteServer(ctx, instance)
	}
	return azurerest.Future{}, nil
}

func (m *mockSQLServerClient) GetFirewallRule(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) error {
	if m.MockGetFirewallRule != nil {
		return m.MockGetFirewallRule(ctx, instance, firewallRuleName)
	}
	return nil
}

func (m *mockSQLServerClient) CreateFirewallRulesBegin(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) ([]byte, error) {
	if m.MockCreateFirewallRulesBegin != nil {
		return m.MockCreateFirewallRulesBegin(ctx, instance, firewallRuleName)
	}
	return nil, nil
}

func (m *mockSQLServerClient) CreateFirewallRulesEnd(createOp []byte) (bool, error) {
	if m.MockCreateFirewallRulesEnd != nil {
		return m.MockCreateFirewallRulesEnd(createOp)
	}
	return true, nil
}

type mockSQLServerClientFactory struct {
	mockClient *mockSQLServerClient
}

func (m *mockSQLServerClientFactory) CreateAPIInstance(*v1alpha1.Provider, kubernetes.Interface) (azureclients.SQLServerAPI, error) {
	return m.mockClient, nil
}

// TestReconcile function tests the reconciliation for the full lifecycle of an Azure SQL Server instance.
// In this test, we specifically use the MySQLServer type, but the underlying reconciliation logic for the
// generic SQL Server is getting exercised (which also covers PostgreSQL)
func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	clientset := fake.NewSimpleClientset()
	sqlServerClient := &mockSQLServerClient{}
	sqlServerClientFactory := &mockSQLServerClientFactory{mockClient: sqlServerClient}

	getCallCount := 0
	sqlServerClient.MockGetServer = func(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*azureclients.SQLServer, error) {
		getCallCount++
		if getCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the instance
			return nil, autorest.DetailedError{StatusCode: http.StatusNotFound}
		}
		// subsequent GET calls should return the created instance
		instanceName := instance.GetName()
		return &azureclients.SQLServer{
			State: string(mysql.ServerStateReady),
			ID:    instanceName + "-azure-id",
			FQDN:  instanceName + ".mydomain.azure.msft.com",
		}, nil
	}
	sqlServerClient.MockCreateServerBegin = func(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error) {
		return []byte("mocked marshalled create future"), nil
	}

	getFirewallCallCount := 0
	sqlServerClient.MockGetFirewallRule = func(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) error {
		getFirewallCallCount++
		if getFirewallCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the firewall rule
			return autorest.DetailedError{StatusCode: http.StatusNotFound}
		}
		return nil
	}
	sqlServerClient.MockCreateFirewallRulesBegin = func(ctx context.Context, instance azuredbv1alpha1.SqlServer, firewallRuleName string) ([]byte, error) {
		return []byte("mocked marshalled firewall create future"), nil
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	r := newMysqlServerReconciler(mgr, sqlServerClientFactory, clientset)
	recFn, requests := SetupTestReconcile(r)
	g.Expect(addMysqlServerReconciler(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// create the provider object and defer its cleanup
	provider := testProvider(testSecret([]byte("testdata")))
	err = c.Create(ctx, provider)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(ctx, provider)

	// Create the SQL Server object and defer its clean up
	instance := testInstance(provider)
	err = c.Create(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer cleanupSQLServer(g, c, requests, instance)

	// 1st reconcile loop should start the create operation
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// after the first reconcile, the create operation should be saved on the running operation field
	expectedStatus := azuredbv1alpha1.SQLServerStatus{
		RunningOperation:     "mocked marshalled create future",
		RunningOperationType: azuredbv1alpha1.OperationCreateServer,
	}
	assertSQLServerStatus(g, c, expectedStatus)

	// 2nd reconcile should finish the create server operation and clear out the running operation field
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = azuredbv1alpha1.SQLServerStatus{
		RunningOperation: "",
	}
	assertSQLServerStatus(g, c, expectedStatus)

	// 3rd reconcile should see that there is no firewall rule yet and try to create it
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = azuredbv1alpha1.SQLServerStatus{
		RunningOperation:     "mocked marshalled firewall create future",
		RunningOperationType: azuredbv1alpha1.OperationCreateFirewallRules,
	}
	assertSQLServerStatus(g, c, expectedStatus)

	// 4th reconcile should finish the create firewall operation and clear out the running operation field
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = azuredbv1alpha1.SQLServerStatus{
		RunningOperation: "",
	}
	assertSQLServerStatus(g, c, expectedStatus)

	// 5th reconcile should find the SQL Server instance from Azure and update the full status of the CRD
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// verify that the CRD status was updated with details about the external SQL Server and that the
	// CRD conditions show the transition from creating to running
	expectedStatus = azuredbv1alpha1.SQLServerStatus{
		Message:    "SQL Server instance test-db-instance is ready",
		State:      "Ready",
		ProviderID: instanceName + "-azure-id",
		Endpoint:   instanceName + ".mydomain.azure.msft.com",
		ConditionedStatus: corev1alpha1.ConditionedStatus{
			Conditions: []corev1alpha1.Condition{
				{
					Type:    corev1alpha1.Ready,
					Status:  v1.ConditionTrue,
					Reason:  conditionStateChanged,
					Message: "SQL Server instance test-db-instance is in the Ready state",
				},
			},
		},
	}
	assertSQLServerStatus(g, c, expectedStatus)

	// wait for the connection information to be stored in a secret, then verify it
	var connectionSecret *v1.Secret
	for {
		if connectionSecret, err = r.clientset.CoreV1().Secrets(namespace).Get(instanceName, metav1.GetOptions{}); err == nil {
			if string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]) != "" {
				break
			}
		}
	}
	assertConnectionSecret(g, c, connectionSecret)

	// verify that a finalizer was added to the CRD
	c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(len(instance.Finalizers)).To(gomega.Equal(1))
	g.Expect(instance.Finalizers[0]).To(gomega.Equal(mysqlFinalizer))

	// test deletion of the instance
	cleanupSQLServer(g, c, requests, instance)
}

func cleanupSQLServer(g *gomega.GomegaWithT, c client.Client, requests chan reconcile.Request, instance *azuredbv1alpha1.MysqlServer) {
	deletedInstance := &azuredbv1alpha1.MysqlServer{}
	if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
		// instance has already been deleted, bail out
		return
	}

	log.Printf("cleaning up SQL Server instance %s by deleting the CRD", instance.Name)
	err := c.Delete(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// wait for the deletion timestamp to be set
	err = wait.ExponentialBackoff(test.DefaultRetry, func() (done bool, err error) {
		deletedInstance := &azuredbv1alpha1.MysqlServer{}
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
		deletedInstance := &azuredbv1alpha1.MysqlServer{}
		if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
}

func assertSQLServerStatus(g *gomega.GomegaWithT, c client.Client, expectedStatus azuredbv1alpha1.SQLServerStatus) {
	instance := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// assert the expected status properties
	g.Expect(instance.Status.Message).To(gomega.Equal(expectedStatus.Message))
	g.Expect(instance.Status.State).To(gomega.Equal(expectedStatus.State))
	g.Expect(instance.Status.ProviderID).To(gomega.Equal(expectedStatus.ProviderID))
	g.Expect(instance.Status.Endpoint).To(gomega.Equal(expectedStatus.Endpoint))
	g.Expect(instance.Status.RunningOperation).To(gomega.Equal(expectedStatus.RunningOperation))
	g.Expect(instance.Status.RunningOperationType).To(gomega.Equal(expectedStatus.RunningOperationType))

	// assert the expected status conditions
	corev1alpha1.AssertConditions(g, expectedStatus.Conditions, instance.Status.ConditionedStatus)
}

func assertConnectionSecret(g *gomega.GomegaWithT, c client.Client, connectionSecret *v1.Secret) {
	instance := &azuredbv1alpha1.MysqlServer{}
	err := c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey])).To(gomega.Equal(instance.Status.Endpoint))
	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretUserKey])).To(gomega.Equal(instance.Spec.AdminLoginName + "@" + instanceName))
	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey])).NotTo(gomega.BeEmpty())
}
