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

package azure

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/azure/database/v1alpha1"
	"github.com/upbound/conductor/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"k8s.io/client-go/kubernetes"
)

var (
	skuShortTiers = map[mysql.SkuTier]string{
		mysql.Basic:           "B",
		mysql.GeneralPurpose:  "GP",
		mysql.MemoryOptimized: "MO",
	}
)

// MySQLServerAPI represents the API interface for a MySQL Server client
type MySQLServerAPI interface {
	Get(ctx context.Context, resourceGroupName string, serverName string) (mysql.Server, error)
	CreateBegin(ctx context.Context, resourceGroupName string, serverName string, parameters mysql.ServerForCreate) ([]byte, error)
	CreateEnd(createOp []byte) (bool, error)
	Delete(ctx context.Context, resourceGroupName string, serverName string) (mysql.ServersDeleteFuture, error)
}

// MySQLServerClient is the concreate implementation of the MySQLServerAPI interface that calls Azure API.
type MySQLServerClient struct {
	mysql.ServersClient
}

// NewMySQLServerClient creates and initializes a MySQLServerClient instance.
func NewMySQLServerClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*MySQLServerClient, error) {
	client, err := NewClient(provider, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %+v", err)
	}

	mysqlServersClient := mysql.NewServersClient(client.SubscriptionID)
	mysqlServersClient.Authorizer = client.Authorizer
	mysqlServersClient.AddToUserAgent(UserAgent)

	return &MySQLServerClient{mysqlServersClient}, nil
}

// CreateBegin begins the create operation for a MySQL Server with the given properties
func (c *MySQLServerClient) CreateBegin(ctx context.Context, resourceGroupName string, serverName string,
	parameters mysql.ServerForCreate) ([]byte, error) {

	// make the call to the MySQL Server Create API
	createFuture, err := c.Create(ctx, resourceGroupName, serverName, parameters)
	if err != nil {
		return nil, err
	}

	// serialize the create operation
	createFutureJSON, err := createFuture.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return createFutureJSON, nil
}

// CreateEnd checks to see if the given create operation is completed and if any error has occurred.
func (c *MySQLServerClient) CreateEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &mysql.ServersCreateFuture{}
	if err = createFuture.UnmarshalJSON(createOp); err != nil {
		return false, err
	}

	// check if the operation is done yet
	done, err = createFuture.Done(c.Client)
	if !done {
		return false, err
	}

	// check the result of the completed operation
	if _, err = createFuture.Result(c.ServersClient); err != nil {
		return true, err
	}

	return true, nil
}

// MySQLServerAPIFactory is an interface that can create instances of the MySQLServerAPI interface
type MySQLServerAPIFactory interface {
	CreateAPIInstance(*v1alpha1.Provider, kubernetes.Interface) (MySQLServerAPI, error)
}

// MySQLServerClientFactory implements the MySQLServerAPIFactory by returning the concrete MySQLServerClient implementation
type MySQLServerClientFactory struct {
}

// CreateAPIInstance returns a concrete MySQLServerClient implementation
func (f *MySQLServerClientFactory) CreateAPIInstance(provider *v1alpha1.Provider, clientset kubernetes.Interface) (MySQLServerAPI, error) {
	return NewMySQLServerClient(provider, clientset)
}

// MySQLServerConditionType converts the given MySQL Server state string into a corresponding condition type
func MySQLServerConditionType(state mysql.ServerState) corev1alpha1.ConditionType {
	switch state {
	case mysql.ServerStateReady:
		return corev1alpha1.Running
	default:
		return corev1alpha1.Failed
	}
}

// MySQLServerStatusMessage returns a status message based on the given server state
func MySQLServerStatusMessage(instanceName string, state mysql.ServerState) string {
	switch state {
	case mysql.ServerStateDisabled:
		return fmt.Sprintf("MySQL Server instance %s is disabled", instanceName)
	case mysql.ServerStateDropping:
		return fmt.Sprintf("MySQL Server instance %s is dropping", instanceName)
	case mysql.ServerStateReady:
		return fmt.Sprintf("MySQL Server instance %s is ready", instanceName)
	default:
		return fmt.Sprintf("MySQL Server instance %s is in an unknown state %s", instanceName, string(state))
	}
}

// MySQLServerSkuName returns the name of the MySQL Server SKU, which is tier + family + cores, e.g. B_Gen4_1, GP_Gen5_8.
func MySQLServerSkuName(pricingTier databasev1alpha1.PricingTierSpec) (string, error) {
	t, ok := skuShortTiers[mysql.SkuTier(pricingTier.Tier)]
	if !ok {
		return "", fmt.Errorf("tier '%s' is not one of the supported values: %+v", pricingTier.Tier, mysql.PossibleSkuTierValues())
	}

	return fmt.Sprintf("%s_%s_%s", t, pricingTier.Family, strconv.Itoa(pricingTier.VCores)), nil
}

// ToSslEnforcement converts the given bool its corresponding SslEnforcementEnum value
func ToSslEnforcement(sslEnforced bool) mysql.SslEnforcementEnum {
	if sslEnforced {
		return mysql.SslEnforcementEnumEnabled
	}
	return mysql.SslEnforcementEnumDisabled
}

// ToGeoRedundantBackup converts the given bool its corresponding GeoRedundantBackup value
func ToGeoRedundantBackup(geoRedundantBackup bool) mysql.GeoRedundantBackup {
	if geoRedundantBackup {
		return mysql.Enabled
	}
	return mysql.Disabled
}
