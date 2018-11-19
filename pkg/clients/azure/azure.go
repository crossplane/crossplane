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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/client-go/kubernetes"
)

const (
	// UserAgent is the user agent extension that identifies the Crossplane Azure client
	UserAgent = "crossplane-azure-client"
)

// Client struct that represents the information needed to connect to the Azure services as a client
type Client struct {
	autorest.Authorizer
	SubscriptionID string
}

type credentials struct {
	ClientID                   string `json:"clientId"`
	ClientSecret               string `json:"clientSecret"`
	TenantID                   string `json:"tenantId"`
	SubscriptionID             string `json:"subscriptionId"`
	ActiveDirectoryEndpointURL string `json:"activeDirectoryEndpointUrl"`
	ResourceManagerEndpointURL string `json:"resourceManagerEndpointUrl"`
}

// NewClient will look up the Azure credential information from the given provider and return a client
// that can be used to connect to Azure services.
func NewClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*Client, error) {
	// first get the secret data that should contain all the auth/creds information
	azureSecretData, err := util.SecretData(clientset, provider.Namespace, provider.Spec.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get azure client secret: %+v", err)
	}

	// load credentials from json data
	creds := credentials{}
	err = json.Unmarshal(azureSecretData, &creds)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal azure client secret data: %+v", err)
	}

	// create a config object from the loaded credentials data
	config := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	config.AADEndpoint = creds.ActiveDirectoryEndpointURL
	config.Resource = creds.ResourceManagerEndpointURL

	authorizer, err := config.Authorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to get authorizer from config: %+v", err)
	}

	return &Client{Authorizer: authorizer, SubscriptionID: creds.SubscriptionID}, nil
}

// ValidateClient verifies if the given client is valid by testing if it can make an Azure service API call
// TODO: is there a better way to validate the Azure client?
func ValidateClient(client *Client) error {
	groupsClient := resources.NewGroupsClient(client.SubscriptionID)
	groupsClient.Authorizer = client.Authorizer
	groupsClient.AddToUserAgent(UserAgent)

	_, err := groupsClient.ListComplete(context.TODO(), "", nil)
	return err
}

// IsNotFound returns a value indicating whether the given error represents that the resource was not found.
func IsNotFound(err error) bool {
	detailedError, ok := err.(autorest.DetailedError)
	if !ok {
		return false
	}

	statusCode, ok := detailedError.StatusCode.(int)
	if !ok {
		return false
	}

	return statusCode == http.StatusNotFound
}
