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

package azure

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/mysql/mgmt/2017-12-01/mysql"
	"github.com/Azure/azure-sdk-for-go/services/postgresql/mgmt/2017-12-01/postgresql"
	azurerest "github.com/Azure/go-autorest/autorest/azure"
	azuredbv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"k8s.io/client-go/kubernetes"
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
	Get(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*SQLServer, error)
	CreateBegin(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error)
	CreateEnd(createOp []byte) (bool, error)
	Delete(ctx context.Context, instance azuredbv1alpha1.SqlServer) (azurerest.Future, error)
}

//---------------------------------------------------------------------------------------------------------------------
// MySQLServerClient

// MySQLServerClient is the concreate implementation of the SQLServerAPI interface for MySQL that calls Azure API.
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

// Get retrieves the requested MySQL Server
func (c *MySQLServerClient) Get(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*SQLServer, error) {
	server, err := c.ServersClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name)
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

// CreateBegin begins the create operation for a MySQL Server with the given properties
func (c *MySQLServerClient) CreateBegin(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error) {
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
	createFuture, err := c.Create(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name, createParams)
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

// Delete deletes the given MySQLServer resource
func (c *MySQLServerClient) Delete(ctx context.Context, instance azuredbv1alpha1.SqlServer) (azurerest.Future, error) {
	result, err := c.ServersClient.Delete(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name)
	return result.Future, err
}

//---------------------------------------------------------------------------------------------------------------------
// PostgreSQLServerClient

// PostgreSQLServerClient is the concreate implementation of the SQLServerAPI interface for PostgreSQL that calls Azure API.
type PostgreSQLServerClient struct {
	postgresql.ServersClient
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

	return &PostgreSQLServerClient{postgreSQLServerClient}, nil
}

// Get retrieves the requested PostgreSQL Server
func (c *PostgreSQLServerClient) Get(ctx context.Context, instance azuredbv1alpha1.SqlServer) (*SQLServer, error) {
	server, err := c.ServersClient.Get(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name)
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

// CreateBegin begins the create operation for a PostgreSQL Server with the given properties
func (c *PostgreSQLServerClient) CreateBegin(ctx context.Context, instance azuredbv1alpha1.SqlServer, adminPassword string) ([]byte, error) {
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
	createFuture, err := c.Create(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name, createParams)
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
func (c *PostgreSQLServerClient) CreateEnd(createOp []byte) (done bool, err error) {
	// unmarshal the given create complete data into a future object
	createFuture := &postgresql.ServersCreateFuture{}
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

// Delete deletes the given PostgreSQL resource
func (c *PostgreSQLServerClient) Delete(ctx context.Context, instance azuredbv1alpha1.SqlServer) (azurerest.Future, error) {
	result, err := c.ServersClient.Delete(ctx, instance.GetSpec().ResourceGroupName, instance.GetObjectMeta().Name)
	return result.Future, err
}

//---------------------------------------------------------------------------------------------------------------------
// SQLServerAPIFactory

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

//---------------------------------------------------------------------------------------------------------------------
// Helper functions
// NOTE: These helper functions work for both MySQL and PostreSQL, but we cast everything to the MySQL types because
// the generated Azure clients for MySQL and PostgreSQL are exactly the same content, just a different package. See:
// https://github.com/Azure/azure-sdk-for-go/blob/master/services/mysql/mgmt/2017-12-01/mysql/models.go
// https://github.com/Azure/azure-sdk-for-go/blob/master/services/postgresql/mgmt/2017-12-01/postgresql/models.go

// SQLServerConditionType converts the given MySQL Server state string into a corresponding condition type
func SQLServerConditionType(state string) corev1alpha1.ConditionType {
	switch mysql.ServerState(state) {
	case mysql.ServerStateReady:
		return corev1alpha1.Ready
	default:
		return corev1alpha1.Failed
	}
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
		return fmt.Sprintf("SQL Server instance %s is in an unknown state %s", instanceName, string(state))
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
