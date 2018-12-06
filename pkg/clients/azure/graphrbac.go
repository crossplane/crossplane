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
	"time"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/crossplaneio/crossplane/pkg/util"
	"github.com/google/uuid"
)

//---------------------------------------------------------------------------------------------------------------------
// Azure Application API interfaces and clients

// ApplicationAPI represents the API interface for an Azure Application client
type ApplicationAPI interface {
	CreateApplication(ctx context.Context, name, url string, password graphrbac.PasswordCredential) (*graphrbac.Application, error)
	GetApplication(ctx context.Context, objectID string) (*graphrbac.Application, error)
	DeleteApplication(ctx context.Context, objectID string) error
}

// ApplicationClient is the concrete implementation of the ApplicationAPI interface that calls Azure API.
type ApplicationClient struct {
	*graphrbac.ApplicationsClient
}

// NewApplicationClient creates and initializes a ApplicationClient instance.
func NewApplicationClient(config *ClientCredentialsConfig) (ApplicationAPI, error) {

	authorizer, err := config.NewGraphAuthorizer()
	if err != nil {
		return nil, err
	}

	client := graphrbac.NewApplicationsClient(config.TenantID)
	client.Authorizer = authorizer
	_ = client.AddToUserAgent(UserAgent)

	return &ApplicationClient{&client}, nil
}

// CreateApplication creates a new AD application with the given name and URL
func (a *ApplicationClient) CreateApplication(ctx context.Context, name, url string, password graphrbac.PasswordCredential) (*graphrbac.Application, error) {

	createParams := graphrbac.ApplicationCreateParameters{
		AvailableToOtherTenants: to.BoolPtr(false),
		DisplayName:             to.StringPtr(name),
		Homepage:                to.StringPtr(url),
		IdentifierUris:          &[]string{url},
		PasswordCredentials:     &[]graphrbac.PasswordCredential{password},
	}

	app, err := a.ApplicationsClient.Create(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return &app, nil
}

// GetApplication with provided object ID
func (a *ApplicationClient) GetApplication(ctx context.Context, id string) (*graphrbac.Application, error) {
	app, err := a.Get(ctx, id)
	return &app, err
}

// DeleteApplication with provided object ID
func (a *ApplicationClient) DeleteApplication(ctx context.Context, id string) error {
	_, err := a.Delete(ctx, id)
	return err
}

//---------------------------------------------------------------------------------------------------------------------
// Azure Service Principal API interfaces and clients

// ServicePrincipalAPI represents the API interface for an Azure service principal client
type ServicePrincipalAPI interface {
	CreateServicePrincipal(ctx context.Context, appID string) (*graphrbac.ServicePrincipal, error)
	GetServicePrincipal(ctx context.Context, objectID string) (*graphrbac.ServicePrincipal, error)
	DeleteServicePrincipal(ctx context.Context, objectID string) error
}

// ServicePrincipalClient is the concrete implementation of the ServicePrincipalAPI interface that calls Azure API.
type ServicePrincipalClient struct {
	*graphrbac.ServicePrincipalsClient
}

// NewServicePrincipalClient creates and initializes a ServicePrincipalClient instance.
func NewServicePrincipalClient(config *ClientCredentialsConfig) (ServicePrincipalAPI, error) {
	authorizer, err := config.NewGraphAuthorizer()
	if err != nil {
		return nil, err
	}

	client := graphrbac.NewServicePrincipalsClient(config.TenantID)
	client.Authorizer = authorizer
	_ = client.AddToUserAgent(UserAgent)

	return &ServicePrincipalClient{&client}, nil
}

// CreateApplication creates a new service principal linked to the given AD application
func (c *ServicePrincipalClient) CreateServicePrincipal(ctx context.Context, appID string) (*graphrbac.ServicePrincipal, error) {
	// reduced parameter set
	params := graphrbac.ServicePrincipalCreateParameters{
		AppID:          to.StringPtr(appID),
		AccountEnabled: to.BoolPtr(true),
	}

	sp, err := c.ServicePrincipalsClient.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return &sp, nil
}

// GetServicePrincipal with given object id value
func (c *ServicePrincipalClient) GetServicePrincipal(ctx context.Context, id string) (*graphrbac.ServicePrincipal, error) {
	sp, err := c.Get(ctx, id)
	return &sp, err
}

// DeleteServicePrincipal with given object id value
func (c *ServicePrincipalClient) DeleteServicePrincipal(ctx context.Context, id string) error {
	_, err := c.Delete(ctx, id)
	return err
}

//---------------------------------------------------------------------------------------------------------------------

// duration for how long the credentials should be valid
const appCredsValidYears = 5

// PasswordCredential a new Azure password credentials with a given name
func NewPasswordCredential(name string) graphrbac.PasswordCredential {
	keyId, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}

	value, err := util.GeneratePassword(16)
	if err != nil {
		panic(err)
	}

	identifier := []byte(name)

	return graphrbac.PasswordCredential{
		CustomKeyIdentifier: &identifier,
		KeyID:               to.StringPtr(keyId.String()),
		Value:               to.StringPtr(value),
		StartDate:           &date.Time{Time: time.Now()},
		EndDate:             &date.Time{Time: time.Now().AddDate(appCredsValidYears, 0, 0)},
	}
}
