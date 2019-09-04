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

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql/mysqlapi"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql/postgresqlapi"
	azurerest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	azuredbv1alpha1 "github.com/crossplaneio/crossplane/azure/apis/database/v1alpha1"
	"github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
)

const (
	backupRetentionDaysDefault = int32(7)
)

var (
	skuShortTiers = map[mysql.SkuTier]string{
		mysql.Basic:           "B",
		mysql.GeneralPurpose:  "GP",
		mysql.MemoryOptimized: "MO",
	}
)

// SQLServer represents an SQL Server (MySQL, PostgreSQL) instance used in the Azure API
type SQLServer struct {
	State string
	ID    string
	FQDN  string
}

// SQLServerAPI represents the API interface for a SQL Server client
type SQLServerAPI interface {
	GetServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (*SQLServer, error)
	CreateServerBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, adminPassword string) ([]byte, error)
	CreateServerEnd(createOp []byte) (bool, error)
	DeleteServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (azurerest.Future, error)
	GetFirewallRule(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) (err error)
	CreateFirewallRulesBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) ([]byte, error)
	CreateFirewallRulesEnd(createOp []byte) (bool, error)
}

//---------------------------------------------------------------------------------------------------------------------
// MySQLServerClient

// MySQLServerClient is the concreate implementation of the SQLServerAPI interface for MySQL that calls Azure API.
type MySQLServerClient struct {
	mysql.ServersClient
	mysql.FirewallRulesClient
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

	firewallRulesClient := mysql.NewFirewallRulesClient(client.SubscriptionID)
	firewallRulesClient.Authorizer = client.Authorizer
	firewallRulesClient.AddToUserAgent(UserAgent)

	return &MySQLServerClient{
		ServersClient:       mysqlServersClient,
		FirewallRulesClient: firewallRulesClient,
	}, nil
}

// GetServer retrieves the requested MySQL Server
func (c *MySQLServerClient) GetServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (*SQLServer, error) {
	server, err := c.ServersClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetName())
	if err != nil {
		return nil, err
	}

	var id string
	if server.ID != nil {
		id = *server.ID
	}

	var fqdn string
	if server.FullyQualifiedDomainName != nil {
		fqdn = *server.FullyQualifiedDomainName
	}

	return &SQLServer{State: string(server.UserVisibleState), ID: id, FQDN: fqdn}, nil
}

// CreateServerBegin begins the create operation for a MySQL Server with the
// given properties.
func (c *MySQLServerClient) CreateServerBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, adminPassword string) ([]byte, error) {
	spec := instance.GetSpec()

	// initialize all the parameters that specify how to configure the server during creation
	skuName, err := SQLServerSkuName(spec.PricingTier)
	if err != nil {
		return nil, fmt.Errorf("failed to create server SKU name: %+v", err)
	}
	capacity := int32(spec.PricingTier.VCores)
	storageMB := int32(spec.StorageProfile.StorageGB * 1024)
	backupRetentionDays := backupRetentionDaysDefault
	if spec.StorageProfile.BackupRetentionDays > 0 {
		backupRetentionDays = int32(spec.StorageProfile.BackupRetentionDays)
	}
	createParams := mysql.ServerForCreate{
		Sku: &mysql.Sku{
			Name:     &skuName,
			Tier:     mysql.SkuTier(spec.PricingTier.Tier),
			Capacity: &capacity,
			Family:   &spec.PricingTier.Family,
		},
		Properties: &mysql.ServerPropertiesForDefaultCreate{
			AdministratorLogin:         &spec.AdminLoginName,
			AdministratorLoginPassword: &adminPassword,
			Version:                    mysql.ServerVersion(spec.Version),
			SslEnforcement:             ToSslEnforcement(spec.SSLEnforced),
			StorageProfile: &mysql.StorageProfile{
				BackupRetentionDays: &backupRetentionDays,
				GeoRedundantBackup:  ToGeoRedundantBackup(spec.StorageProfile.GeoRedundantBackup),
				StorageMB:           &storageMB,
			},
			CreateMode: mysql.CreateModeDefault,
		},
		Location: &spec.Location,
	}

	// make the call to the MySQL Server Create API
	createFuture, err := c.Create(ctx, instance.GetSpec().ResourceGroupName, instance.GetName(), createParams)
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

// CreateServerEnd checks to see if the given create operation is completed and
// if any error has occurred.
func (c *MySQLServerClient) CreateServerEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &mysql.ServersCreateFuture{}
	if err = createFuture.UnmarshalJSON(createOp); err != nil {
		return false, err
	}

	// check if the operation is done yet
	done, err = createFuture.DoneWithContext(context.Background(), c.ServersClient.Client)
	if !done {
		return false, err
	}

	// check the result of the completed operation
	if _, err = createFuture.Result(c.ServersClient); err != nil {
		return true, err
	}

	return true, nil
}

// DeleteServer deletes the given MySQLServer resource
func (c *MySQLServerClient) DeleteServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (azurerest.Future, error) {
	result, err := c.ServersClient.Delete(ctx, instance.GetSpec().ResourceGroupName, instance.GetName())
	return result.Future, err
}

// GetFirewallRule gets the given firewall rule
func (c *MySQLServerClient) GetFirewallRule(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) error {
	_, err := c.FirewallRulesClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetName(), firewallRuleName)
	return err
}

// CreateFirewallRulesBegin begins the create operation for a firewall rule
func (c *MySQLServerClient) CreateFirewallRulesBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) ([]byte, error) {

	createParams := mysql.FirewallRule{
		Name: to.StringPtr(firewallRuleName),
		FirewallRuleProperties: &mysql.FirewallRuleProperties{
			// TODO: this firewall rules allows inbound access to the Azure MySQL Server from anywhere.
			// we need to better model/abstract tighter inbound access rules.
			StartIPAddress: to.StringPtr("0.0.0.0"),
			EndIPAddress:   to.StringPtr("255.255.255.255"),
		},
	}

	createFuture, err := c.FirewallRulesClient.CreateOrUpdate(ctx, instance.GetSpec().ResourceGroupName,
		instance.GetName(), firewallRuleName, createParams)
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

// CreateFirewallRulesEnd checks to see if the given create operation is completed and if any error has occurred.
func (c *MySQLServerClient) CreateFirewallRulesEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &mysql.FirewallRulesCreateOrUpdateFuture{}
	if err = createFuture.UnmarshalJSON(createOp); err != nil {
		return false, err
	}

	// check if the operation is done yet
	done, err = createFuture.DoneWithContext(context.Background(), c.FirewallRulesClient.Client)
	if !done {
		return false, err
	}

	// check the result of the completed operation
	if _, err = createFuture.Result(c.FirewallRulesClient); err != nil {
		return true, err
	}

	return true, nil
}

//---------------------------------------------------------------------------------------------------------------------
// MySQLVirtualNetworkRulesClient

// A MySQLVirtualNetworkRulesClient handles CRUD operations for Azure Virtual Network Rules.
type MySQLVirtualNetworkRulesClient mysqlapi.VirtualNetworkRulesClientAPI

// NewMySQLVirtualNetworkRulesClient returns a new Azure Virtual Network Rules client. Credentials must be
// passed as JSON encoded data.
func NewMySQLVirtualNetworkRulesClient(ctx context.Context, credentials []byte) (MySQLVirtualNetworkRulesClient, error) {
	c := Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	client := mysql.NewVirtualNetworkRulesClient(c.SubscriptionID)

	cfg := auth.ClientCredentialsConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TenantID:     c.TenantID,
		AADEndpoint:  c.ActiveDirectoryEndpointURL,
		Resource:     c.ResourceManagerEndpointURL,
	}
	a, err := cfg.Authorizer()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create Azure authorizer from credentials config")
	}
	client.Authorizer = a
	if err := client.AddToUserAgent(UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewMySQLVirtualNetworkRuleParameters returns an Azure VirtualNetworkRule object from a virtual network spec
func NewMySQLVirtualNetworkRuleParameters(v *azuredbv1alpha1.MysqlServerVirtualNetworkRule) mysql.VirtualNetworkRule {
	return mysql.VirtualNetworkRule{
		Name: ToStringPtr(v.Spec.Name),
		VirtualNetworkRuleProperties: &mysql.VirtualNetworkRuleProperties{
			VirtualNetworkSubnetID:           ToStringPtr(v.Spec.VirtualNetworkRuleProperties.VirtualNetworkSubnetID),
			IgnoreMissingVnetServiceEndpoint: ToBoolPtr(v.Spec.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint, FieldRequired),
		},
	}
}

// MySQLServerVirtualNetworkRuleNeedsUpdate determines if a virtual network rule needs to be updated
func MySQLServerVirtualNetworkRuleNeedsUpdate(kube *azuredbv1alpha1.MysqlServerVirtualNetworkRule, az mysql.VirtualNetworkRule) bool {
	up := NewMySQLVirtualNetworkRuleParameters(kube)

	switch {
	case !reflect.DeepEqual(up.VirtualNetworkRuleProperties.VirtualNetworkSubnetID, az.VirtualNetworkRuleProperties.VirtualNetworkSubnetID):
		return true
	case !reflect.DeepEqual(up.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint, az.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint):
		return true
	}

	return false
}

// MySQLVirtualNetworkRuleStatusFromAzure converts an Azure subnet to a SubnetStatus
func MySQLVirtualNetworkRuleStatusFromAzure(az mysql.VirtualNetworkRule) azuredbv1alpha1.VirtualNetworkRuleStatus {
	return azuredbv1alpha1.VirtualNetworkRuleStatus{
		State: string(az.VirtualNetworkRuleProperties.State),
		ID:    ToString(az.ID),
		Type:  ToString(az.Type),
	}
}

//---------------------------------------------------------------------------------------------------------------------
// PostgreSQLServerClient

// PostgreSQLServerClient is the concreate implementation of the SQLServerAPI interface for PostgreSQL that calls Azure API.
type PostgreSQLServerClient struct {
	postgresql.ServersClient
	postgresql.FirewallRulesClient
}

// NewPostgreSQLServerClient creates and initializes a PostgreSQLServerClient instance.
func NewPostgreSQLServerClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*PostgreSQLServerClient, error) {
	client, err := NewClient(provider, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %+v", err)
	}

	postgreSQLServerClient := postgresql.NewServersClient(client.SubscriptionID)
	postgreSQLServerClient.Authorizer = client.Authorizer
	postgreSQLServerClient.AddToUserAgent(UserAgent)

	firewallRulesClient := postgresql.NewFirewallRulesClient(client.SubscriptionID)
	firewallRulesClient.Authorizer = client.Authorizer
	firewallRulesClient.AddToUserAgent(UserAgent)

	return &PostgreSQLServerClient{
		ServersClient:       postgreSQLServerClient,
		FirewallRulesClient: firewallRulesClient,
	}, nil
}

// GetServer retrieves the requested PostgreSQL Server
func (c *PostgreSQLServerClient) GetServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (*SQLServer, error) {
	server, err := c.ServersClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetName())
	if err != nil {
		return nil, err
	}

	var id string
	if server.ID != nil {
		id = *server.ID
	}

	var fqdn string
	if server.FullyQualifiedDomainName != nil {
		fqdn = *server.FullyQualifiedDomainName
	}

	return &SQLServer{State: string(server.UserVisibleState), ID: id, FQDN: fqdn}, nil
}

// CreateServerBegin begins the create operation for a PostgreSQL Server with the given properties
func (c *PostgreSQLServerClient) CreateServerBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, adminPassword string) ([]byte, error) {
	spec := instance.GetSpec()

	// initialize all the parameters that specify how to configure the server during creation
	skuName, err := SQLServerSkuName(spec.PricingTier)
	if err != nil {
		return nil, fmt.Errorf("failed to create server SKU name: %+v", err)
	}
	capacity := int32(spec.PricingTier.VCores)
	storageMB := int32(spec.StorageProfile.StorageGB * 1024)
	backupRetentionDays := backupRetentionDaysDefault
	if spec.StorageProfile.BackupRetentionDays > 0 {
		backupRetentionDays = int32(spec.StorageProfile.BackupRetentionDays)
	}
	createParams := postgresql.ServerForCreate{
		Sku: &postgresql.Sku{
			Name:     &skuName,
			Tier:     postgresql.SkuTier(spec.PricingTier.Tier),
			Capacity: &capacity,
			Family:   &spec.PricingTier.Family,
		},
		Properties: &postgresql.ServerPropertiesForDefaultCreate{
			AdministratorLogin:         &spec.AdminLoginName,
			AdministratorLoginPassword: &adminPassword,
			Version:                    postgresql.ServerVersion(spec.Version),
			SslEnforcement:             postgresql.SslEnforcementEnum(ToSslEnforcement(spec.SSLEnforced)),
			StorageProfile: &postgresql.StorageProfile{
				BackupRetentionDays: &backupRetentionDays,
				GeoRedundantBackup:  postgresql.GeoRedundantBackup(ToGeoRedundantBackup(spec.StorageProfile.GeoRedundantBackup)),
				StorageMB:           &storageMB,
			},
			CreateMode: postgresql.CreateModeDefault,
		},
		Location: &spec.Location,
	}

	// make the call to the PostgreSQL Server Create API
	createFuture, err := c.Create(ctx, instance.GetSpec().ResourceGroupName, instance.GetName(), createParams)
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

// CreateServerEnd checks to see if the given create operation is completed and if any error has occurred.
func (c *PostgreSQLServerClient) CreateServerEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &postgresql.ServersCreateFuture{}
	if err = createFuture.UnmarshalJSON(createOp); err != nil {
		return false, err
	}

	// check if the operation is done yet
	done, err = createFuture.DoneWithContext(context.Background(), c.ServersClient.Client)
	if !done {
		return false, err
	}

	// check the result of the completed operation
	if _, err = createFuture.Result(c.ServersClient); err != nil {
		return true, err
	}

	return true, nil
}

// DeleteServer deletes the given PostgreSQL resource
func (c *PostgreSQLServerClient) DeleteServer(ctx context.Context, instance azuredbv1alpha1.SQLServer) (azurerest.Future, error) {
	result, err := c.ServersClient.Delete(ctx, instance.GetSpec().ResourceGroupName, instance.GetName())
	return result.Future, err
}

// GetFirewallRule gets the given firewall rule
func (c *PostgreSQLServerClient) GetFirewallRule(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) error {
	_, err := c.FirewallRulesClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetName(), firewallRuleName)
	return err
}

// CreateFirewallRulesBegin begins the create operation for a firewall rule
func (c *PostgreSQLServerClient) CreateFirewallRulesBegin(ctx context.Context, instance azuredbv1alpha1.SQLServer, firewallRuleName string) ([]byte, error) {

	createParams := postgresql.FirewallRule{
		Name: to.StringPtr(firewallRuleName),
		FirewallRuleProperties: &postgresql.FirewallRuleProperties{
			// TODO: this firewall rules allows inbound access to the Azure PostgreSQL Server from anywhere.
			// we need to better model/abstract tighter inbound access rules.
			StartIPAddress: to.StringPtr("0.0.0.0"),
			EndIPAddress:   to.StringPtr("255.255.255.255"),
		},
	}

	createFuture, err := c.FirewallRulesClient.CreateOrUpdate(ctx, instance.GetSpec().ResourceGroupName,
		instance.GetName(), firewallRuleName, createParams)
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

// CreateFirewallRulesEnd checks to see if the given create operation is completed and if any error has occurred.
func (c *PostgreSQLServerClient) CreateFirewallRulesEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &postgresql.FirewallRulesCreateOrUpdateFuture{}
	if err = createFuture.UnmarshalJSON(createOp); err != nil {
		return false, err
	}

	// check if the operation is done yet
	done, err = createFuture.DoneWithContext(context.Background(), c.FirewallRulesClient.Client)
	if !done {
		return false, err
	}

	// check the result of the completed operation
	if _, err = createFuture.Result(c.FirewallRulesClient); err != nil {
		return true, err
	}

	return true, nil
}

//---------------------------------------------------------------------------------------------------------------------
// PostgreSQLVirtualNetworkRulesClient

// A PostgreSQLVirtualNetworkRulesClient handles CRUD operations for Azure Virtual Network Rules.
type PostgreSQLVirtualNetworkRulesClient postgresqlapi.VirtualNetworkRulesClientAPI

// NewPostgreSQLVirtualNetworkRulesClient returns a new Azure Virtual Network Rules client. Credentials must be
// passed as JSON encoded data.
func NewPostgreSQLVirtualNetworkRulesClient(ctx context.Context, credentials []byte) (PostgreSQLVirtualNetworkRulesClient, error) {
	c := Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	client := postgresql.NewVirtualNetworkRulesClient(c.SubscriptionID)

	cfg := auth.ClientCredentialsConfig{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TenantID:     c.TenantID,
		AADEndpoint:  c.ActiveDirectoryEndpointURL,
		Resource:     c.ResourceManagerEndpointURL,
	}
	a, err := cfg.Authorizer()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create Azure authorizer from credentials config")
	}
	client.Authorizer = a
	if err := client.AddToUserAgent(UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewPostgreSQLVirtualNetworkRuleParameters returns an Azure VirtualNetworkRule object from a virtual network spec
func NewPostgreSQLVirtualNetworkRuleParameters(v *azuredbv1alpha1.PostgresqlServerVirtualNetworkRule) postgresql.VirtualNetworkRule {
	return postgresql.VirtualNetworkRule{
		Name: ToStringPtr(v.Spec.Name),
		VirtualNetworkRuleProperties: &postgresql.VirtualNetworkRuleProperties{
			VirtualNetworkSubnetID:           ToStringPtr(v.Spec.VirtualNetworkRuleProperties.VirtualNetworkSubnetID),
			IgnoreMissingVnetServiceEndpoint: ToBoolPtr(v.Spec.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint, FieldRequired),
		},
	}
}

// PostgreSQLServerVirtualNetworkRuleNeedsUpdate determines if a virtual network rule needs to be updated
func PostgreSQLServerVirtualNetworkRuleNeedsUpdate(kube *azuredbv1alpha1.PostgresqlServerVirtualNetworkRule, az postgresql.VirtualNetworkRule) bool {
	up := NewPostgreSQLVirtualNetworkRuleParameters(kube)

	switch {
	case !reflect.DeepEqual(up.VirtualNetworkRuleProperties.VirtualNetworkSubnetID, az.VirtualNetworkRuleProperties.VirtualNetworkSubnetID):
		return true
	case !reflect.DeepEqual(up.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint, az.VirtualNetworkRuleProperties.IgnoreMissingVnetServiceEndpoint):
		return true
	}

	return false
}

// PostgreSQLVirtualNetworkRuleStatusFromAzure converts an Azure subnet to a SubnetStatus
func PostgreSQLVirtualNetworkRuleStatusFromAzure(az postgresql.VirtualNetworkRule) azuredbv1alpha1.VirtualNetworkRuleStatus {
	return azuredbv1alpha1.VirtualNetworkRuleStatus{
		State: string(az.State),
		ID:    ToString(az.ID),
		Type:  ToString(az.Type),
	}
}

// SQLServerAPIFactory is an interface that can create instances of the SQLServerAPI interface
type SQLServerAPIFactory interface {
	CreateAPIInstance(*v1alpha1.Provider, kubernetes.Interface) (SQLServerAPI, error)
}

// MySQLServerClientFactory implements the SQLServerAPIFactory by returning the concrete MySQLServerClient implementation
type MySQLServerClientFactory struct {
}

// CreateAPIInstance returns a concrete MySQLServerClient implementation
func (f *MySQLServerClientFactory) CreateAPIInstance(provider *v1alpha1.Provider, clientset kubernetes.Interface) (SQLServerAPI, error) {
	return NewMySQLServerClient(provider, clientset)
}

// PostgreSQLServerClientFactory implements the SQLServerAPIFactory by returning the concrete PostgreSQLServerClient implementation
type PostgreSQLServerClientFactory struct {
}

// CreateAPIInstance returns a concrete PostgreSQLServerClient implementation
func (f *PostgreSQLServerClientFactory) CreateAPIInstance(provider *v1alpha1.Provider, clientset kubernetes.Interface) (SQLServerAPI, error) {
	return NewPostgreSQLServerClient(provider, clientset)
}

// Helper functions
// NOTE: These helper functions work for both MySQL and PostreSQL, but we cast everything to the MySQL types because
// the generated Azure clients for MySQL and PostgreSQL are exactly the same content, just a different package. See:
// https://github.com/Azure/azure-sdk-for-go/blob/master/services/mysql/mgmt/2017-12-01/mysql/models.go
// https://github.com/Azure/azure-sdk-for-go/blob/master/services/postgresql/mgmt/2017-12-01/postgresql/models.go

// SQLServerCondition converts the given MySQL Server state string into a corresponding condition.
func SQLServerCondition(state string) runtimev1alpha1.Condition {
	if mysql.ServerState(state) == mysql.ServerStateReady {
		return runtimev1alpha1.Available()
	}
	return runtimev1alpha1.Unavailable()
}

// SQLServerStatusMessage returns a status message based on the given server state
func SQLServerStatusMessage(instanceName string, state string) string {
	switch mysql.ServerState(state) {
	case mysql.ServerStateDisabled:
		return fmt.Sprintf("SQL Server instance %s is disabled", instanceName)
	case mysql.ServerStateDropping:
		return fmt.Sprintf("SQL Server instance %s is dropping", instanceName)
	case mysql.ServerStateReady:
		return fmt.Sprintf("SQL Server instance %s is ready", instanceName)
	default:
		return fmt.Sprintf("SQL Server instance %s is in an unknown state %s", instanceName, state)
	}
}

// SQLServerSkuName returns the name of the MySQL Server SKU, which is tier + family + cores, e.g. B_Gen4_1, GP_Gen5_8.
func SQLServerSkuName(pricingTier azuredbv1alpha1.PricingTierSpec) (string, error) {
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
