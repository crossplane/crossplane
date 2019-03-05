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
	"time"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/date"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	appURLFmt          = "https://%s.%s.%s.cloudapp.crossplane.io"
	appCredsValidYears = 5
	urlSaltDataLen     = 3
)

//---------------------------------------------------------------------------------------------------------------------
// Azure Application API interfaces and clients

// ApplicationAPI represents the API interface for an Azure Application client
type ApplicationAPI interface {
	CreateApplication(ctx context.Context, appParams ApplicationParameters) (*graphrbac.Application, error)
	DeleteApplication(ctx context.Context, appObjectID string) error
}

// ApplicationClient is the concreate implementation of the ApplicationAPI interface that calls Azure API.
type ApplicationClient struct {
	graphrbac.ApplicationsClient
}

// ApplicationParameters are the parameters used to create an AD application
type ApplicationParameters struct {
	Name          string
	DNSNamePrefix string
	Location      string
	ObjectID      string
	ClientSecret  string
}

// NewApplicationClient creates and initializes a ApplicationClient instance.
func NewApplicationClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*ApplicationClient, error) {
	client, err := NewClient(provider, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %+v", err)
	}

	graphAuthorizer, err := getGraphAuthorizer(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph authorizer: %+v", err)
	}

	appClient := graphrbac.NewApplicationsClient(client.tenantID)
	appClient.Authorizer = graphAuthorizer
	appClient.AddToUserAgent(UserAgent)

	return &ApplicationClient{appClient}, nil
}

// CreateApplication creates a new AD application with the given parameters
func (c *ApplicationClient) CreateApplication(ctx context.Context, appParams ApplicationParameters) (*graphrbac.Application, error) {
	if appParams.ObjectID != "" {
		// the caller has already created the app, fetch and return it
		app, err := c.ApplicationsClient.Get(ctx, appParams.ObjectID)
		return &app, err
	}

	location := util.ToLowerRemoveSpaces(appParams.Location)
	salt, err := util.GenerateHex(urlSaltDataLen)
	if err != nil {
		return nil, fmt.Errorf("failed to generate url salt: %+v", err)
	}
	url := fmt.Sprintf(appURLFmt, salt, appParams.DNSNamePrefix, location)

	credsKeyID, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate creds key ID: %+v", err)
	}

	passwordCreds := &[]graphrbac.PasswordCredential{
		{
			StartDate: &date.Time{Time: time.Now()},
			EndDate:   &date.Time{Time: time.Now().AddDate(appCredsValidYears, 0, 0)},
			KeyID:     to.StringPtr(credsKeyID.String()),
			Value:     to.StringPtr(appParams.ClientSecret),
		},
	}

	createParams := graphrbac.ApplicationCreateParameters{
		AvailableToOtherTenants: to.BoolPtr(false),
		DisplayName:             to.StringPtr(appParams.Name),
		Homepage:                to.StringPtr(url),
		IdentifierUris:          &[]string{url},
		PasswordCredentials:     passwordCreds,
	}

	app, err := c.ApplicationsClient.Create(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return &app, nil
}

// DeleteApplication will delete the given AD application
func (c *ApplicationClient) DeleteApplication(ctx context.Context, appObjectID string) error {
	_, err := c.ApplicationsClient.Delete(ctx, appObjectID)
	return err
}

//---------------------------------------------------------------------------------------------------------------------
// Azure Service Principal API interfaces and clients

// ServicePrincipalAPI represents the API interface for an Azure service principal client
type ServicePrincipalAPI interface {
	CreateServicePrincipal(ctx context.Context, spID, appID string) (*graphrbac.ServicePrincipal, error)
	DeleteServicePrincipal(ctx context.Context, spID string) error
}

// ServicePrincipalClient is the concreate implementation of the ServicePrincipalAPI interface that calls Azure API.
type ServicePrincipalClient struct {
	graphrbac.ServicePrincipalsClient
}

// NewServicePrincipalClient creates and initializes a ServicePrincipalClient instance.
func NewServicePrincipalClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*ServicePrincipalClient, error) {
	client, err := NewClient(provider, clientset)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %+v", err)
	}

	graphAuthorizer, err := getGraphAuthorizer(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph authorizer: %+v", err)
	}

	spClient := graphrbac.NewServicePrincipalsClient(client.tenantID)
	spClient.Authorizer = graphAuthorizer
	spClient.AddToUserAgent(UserAgent)

	return &ServicePrincipalClient{spClient}, nil
}

// CreateServicePrincipal creates a new service principal linked to the given AD application
func (c *ServicePrincipalClient) CreateServicePrincipal(ctx context.Context, spID, appID string) (*graphrbac.ServicePrincipal, error) {
	if spID != "" {
		// the caller has already created the service principal, fetch and return it
		sp, err := c.ServicePrincipalsClient.Get(ctx, spID)
		return &sp, err
	}

	createParams := graphrbac.ServicePrincipalCreateParameters{
		AppID:          to.StringPtr(appID),
		AccountEnabled: to.BoolPtr(true),
	}

	sp, err := c.ServicePrincipalsClient.Create(ctx, createParams)
	if err != nil {
		return nil, err
	}

	return &sp, nil
}

// DeleteServicePrincipal will delete the given service principal
func (c *ServicePrincipalClient) DeleteServicePrincipal(ctx context.Context, spID string) error {
	_, err := c.ServicePrincipalsClient.Delete(ctx, spID)
	return err
}

//---------------------------------------------------------------------------------------------------------------------
// Graph helpers

func getGraphAuthorizer(client *Client) (autorest.Authorizer, error) {
	oauthConfig, err := adal.NewOAuthConfig(client.activeDirectoryEndpointURL, client.tenantID)
	if err != nil {
		return nil, err
	}

	token, err := adal.NewServicePrincipalToken(*oauthConfig, client.clientID, client.clientSecret, client.activeDirectoryGraphResourceID)
	if err != nil {
		return nil, err
	}
	token.Refresh()

	return autorest.NewBearerAuthorizer(token), nil
}
