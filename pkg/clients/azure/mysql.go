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

type MySQLServerAPI interface {
	Get(ctx context.Context, resourceGroupName string, serverName string) (mysql.Server, error)
	Create(ctx context.Context, resourceGroupName string, serverName string, parameters mysql.ServerForCreate) (mysql.ServersCreateFuture, error)
	CreateDone(createFuture *mysql.ServersCreateFuture) (bool, error)
	CreateResult(createFuture *mysql.ServersCreateFuture) (mysql.Server, error)
	MarshalCreateFuture(createFuture mysql.ServersCreateFuture) ([]byte, error)
	UnmarshalCreateFuture(createFuture *mysql.ServersCreateFuture, data []byte) error
	Delete(ctx context.Context, resourceGroupName string, serverName string) (mysql.ServersDeleteFuture, error)
}

type MySQLServerClient struct {
	mysql.ServersClient
}

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

func (c *MySQLServerClient) CreateDone(createFuture *mysql.ServersCreateFuture) (bool, error) {
	return createFuture.Done(c.Client)
}

func (c *MySQLServerClient) CreateResult(createFuture *mysql.ServersCreateFuture) (mysql.Server, error) {
	return createFuture.Result(c.ServersClient)
}

func (c *MySQLServerClient) MarshalCreateFuture(createFuture mysql.ServersCreateFuture) ([]byte, error) {
	return createFuture.MarshalJSON()
}

func (c *MySQLServerClient) UnmarshalCreateFuture(createFuture *mysql.ServersCreateFuture, data []byte) error {
	if createFuture == nil {
		return fmt.Errorf("cannot unmarshal a nil ServersCreateFuture")
	}
	return createFuture.UnmarshalJSON(data)
}

type MySQLServerAPIFactory interface {
	CreateAPIInstance(*v1alpha1.Provider, kubernetes.Interface) (MySQLServerAPI, error)
}

type MySQLServerClientFactory struct {
}

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
