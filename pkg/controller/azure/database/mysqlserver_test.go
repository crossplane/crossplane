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
	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/test"
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
)

type mockMySQLServerClient struct {
	MockGetServer                func(ctx context.Context, resourceGroupName string, serverName string) (mysql.Server, error)
	MockCreateServerBegin        func(ctx context.Context, resourceGroupName string, serverName string, parameters mysql.ServerForCreate) ([]byte, error)
	MockCreateServerEnd          func(createOp []byte) (bool, error)
	MockDeleteServer             func(ctx context.Context, resourceGroupName string, serverName string) (mysql.ServersDeleteFuture, error)
	MockGetFirewallRule          func(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string) (result mysql.FirewallRule, err error)
	MockCreateFirewallRulesBegin func(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string, parameters mysql.FirewallRule) ([]byte, error)
	MockCreateFirewallRulesEnd   func(createOp []byte) (bool, error)
}

func (m *mockMySQLServerClient) GetServer(ctx context.Context, resourceGroupName string, serverName string) (mysql.Server, error) {
	if m.MockGetServer != nil {
		return m.MockGetServer(ctx, resourceGroupName, serverName)
	}
	return mysql.Server{}, nil
}

func (m *mockMySQLServerClient) CreateServerBegin(ctx context.Context, resourceGroupName string, serverName string, parameters mysql.ServerForCreate) ([]byte, error) {
	if m.MockCreateServerBegin != nil {
		return m.MockCreateServerBegin(ctx, resourceGroupName, serverName, parameters)
	}
	return nil, nil
}

func (m *mockMySQLServerClient) CreateServerEnd(createOp []byte) (bool, error) {
	if m.MockCreateServerEnd != nil {
		return m.MockCreateServerEnd(createOp)
	}
	return true, nil
}

func (m *mockMySQLServerClient) DeleteServer(ctx context.Context, resourceGroupName string, serverName string) (mysql.ServersDeleteFuture, error) {
	if m.MockDeleteServer != nil {
		return m.MockDeleteServer(ctx, resourceGroupName, serverName)
	}
	return mysql.ServersDeleteFuture{}, nil
}

func (m *mockMySQLServerClient) GetFirewallRule(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string) (result mysql.FirewallRule, err error) {
	if m.MockGetFirewallRule != nil {
		return m.MockGetFirewallRule(ctx, resourceGroupName, serverName, firewallRuleName)
	}
	return mysql.FirewallRule{}, nil
}

func (m *mockMySQLServerClient) CreateFirewallRulesBegin(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string, parameters mysql.FirewallRule) ([]byte, error) {
	if m.MockCreateFirewallRulesBegin != nil {
		return m.MockCreateFirewallRulesBegin(ctx, resourceGroupName, serverName, firewallRuleName, parameters)
	}
	return nil, nil
}

func (m *mockMySQLServerClient) CreateFirewallRulesEnd(createOp []byte) (bool, error) {
	if m.MockCreateFirewallRulesEnd != nil {
		return m.MockCreateFirewallRulesEnd(createOp)
	}
	return true, nil
}

type mockMySQLServerClientFactory struct {
	mockClient *mockMySQLServerClient
}

func (m *mockMySQLServerClientFactory) CreateAPIInstance(*v1alpha1.Provider, kubernetes.Interface) (azureclients.MySQLServerAPI, error) {
	return m.mockClient, nil
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	clientset := fake.NewSimpleClientset()
	mysqlServerClient := &mockMySQLServerClient{}
	mysqlServerClientFactory := &mockMySQLServerClientFactory{mockClient: mysqlServerClient}

	getCallCount := 0
	mysqlServerClient.MockGetServer = func(ctx context.Context, resourceGroupName string, serverName string) (mysql.Server, error) {
		getCallCount++
		if getCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the instance
			return mysql.Server{}, autorest.DetailedError{StatusCode: http.StatusNotFound}
		}
		// subsequent GET calls should return the created instance
		fqdn := instanceName + ".mydomain.azure.msft.com"
		id := instanceName + "-azure-id"
		return mysql.Server{
			ID: &id,
			ServerProperties: &mysql.ServerProperties{
				UserVisibleState:         mysql.ServerStateReady,
				FullyQualifiedDomainName: &fqdn,
			},
		}, nil
	}
	mysqlServerClient.MockCreateServerBegin = func(ctx context.Context, resourceGroupName string, serverName string, parameters mysql.ServerForCreate) ([]byte, error) {
		return []byte("mocked marshalled create future"), nil
	}

	getFirewallCallCount := 0
	mysqlServerClient.MockGetFirewallRule = func(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string) (result mysql.FirewallRule, err error) {
		getFirewallCallCount++
		if getFirewallCallCount <= 1 {
			// first GET should return not found, which will cause the reconcile loop to try to create the firewall rule
			return mysql.FirewallRule{}, autorest.DetailedError{StatusCode: http.StatusNotFound}
		}
		return mysql.FirewallRule{}, nil
	}
	mysqlServerClient.MockCreateFirewallRulesBegin = func(ctx context.Context, resourceGroupName string, serverName string, firewallRuleName string, parameters mysql.FirewallRule) ([]byte, error) {
		return []byte("mocked marshalled firewall create future"), nil
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c := mgr.GetClient()

	r := newMysqlServerReconciler(mgr, mysqlServerClientFactory, clientset)
	recFn, requests := SetupTestReconcile(r)
	g.Expect(addMysqlServerReconciler(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// create the provider object and defer its cleanup
	provider := testProvider(testSecret([]byte("testdata")))
	err = c.Create(ctx, provider)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(ctx, provider)

	// Create the MySQL Server object and defer its clean up
	instance := testInstance(provider)
	err = c.Create(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer cleanupMySQLServer(g, c, requests, instance)

	// 1st reconcile loop should start the create operation
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// after the first reconcile, the create operation should be saved on the running operation field
	expectedStatus := databasev1alpha1.MysqlServerStatus{
		RunningOperation:     "mocked marshalled create future",
		RunningOperationType: databasev1alpha1.OperationCreateServer,
	}
	assertMySQLServerStatus(g, c, expectedStatus)

	// 2nd reconcile should finish the create server operation and clear out the running operation field
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = databasev1alpha1.MysqlServerStatus{
		RunningOperation: "",
	}
	assertMySQLServerStatus(g, c, expectedStatus)

	// 3rd reconcile should see that there is no firewall rule yet and try to create it
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = databasev1alpha1.MysqlServerStatus{
		RunningOperation:     "mocked marshalled firewall create future",
		RunningOperationType: databasev1alpha1.OperationCreateFirewallRules,
	}
	assertMySQLServerStatus(g, c, expectedStatus)

	// 4th reconcile should finish the create firewall operation and clear out the running operation field
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	expectedStatus = databasev1alpha1.MysqlServerStatus{
		RunningOperation: "",
	}
	assertMySQLServerStatus(g, c, expectedStatus)

	// 5th reconcile should find the MySQL Server instance from Azure and update the full status of the CRD
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// verify that the CRD status was updated with details about the external MySQL Server and that the
	// CRD conditions show the transition from creating to running
	expectedStatus = databasev1alpha1.MysqlServerStatus{
		Message:    "MySQL Server instance test-db-instance is ready",
		State:      "Ready",
		ProviderID: instanceName + "-azure-id",
		Endpoint:   instanceName + ".mydomain.azure.msft.com",
		ConditionedStatus: corev1alpha1.ConditionedStatus{
			Conditions: []corev1alpha1.Condition{
				{
					Type:    corev1alpha1.Ready,
					Status:  v1.ConditionTrue,
					Reason:  conditionStateChanged,
					Message: "MySQL Server instance test-db-instance is in the Ready state",
				},
			},
		},
	}
	assertMySQLServerStatus(g, c, expectedStatus)

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
	g.Expect(instance.Finalizers[0]).To(gomega.Equal(finalizer))

	// test deletion of the instance
	cleanupMySQLServer(g, c, requests, instance)
}

func cleanupMySQLServer(g *gomega.GomegaWithT, c client.Client, requests chan reconcile.Request, instance *databasev1alpha1.MysqlServer) {
	deletedInstance := &databasev1alpha1.MysqlServer{}
	if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
		// instance has already been deleted, bail out
		return
	}

	log.Printf("cleaning up MySQL Server instance %s by deleting the CRD", instance.Name)
	err := c.Delete(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// wait for the deletion timestamp to be set
	err = wait.ExponentialBackoff(test.DefaultRetry, func() (done bool, err error) {
		deletedInstance := &databasev1alpha1.MysqlServer{}
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
		deletedInstance := &databasev1alpha1.MysqlServer{}
		if err := c.Get(ctx, expectedRequest.NamespacedName, deletedInstance); errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())
}

func assertMySQLServerStatus(g *gomega.GomegaWithT, c client.Client, expectedStatus databasev1alpha1.MysqlServerStatus) {
	instance := &databasev1alpha1.MysqlServer{}
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
	instance := &databasev1alpha1.MysqlServer{}
	err := c.Get(ctx, expectedRequest.NamespacedName, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey])).To(gomega.Equal(instance.Status.Endpoint))
	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretUserKey])).To(gomega.Equal(instance.Spec.AdminLoginName + "@" + instanceName))
	g.Expect(string(connectionSecret.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey])).NotTo(gomega.BeEmpty())
}
