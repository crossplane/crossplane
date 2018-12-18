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
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

const (
	// UserAgent is the user agent extension that identifies the Crossplane Azure client
	UserAgent = "crossplane-azure-client"
)

type credentials struct {
	ClientID                       string `json:"clientId"`
	ClientSecret                   string `json:"clientSecret"`
	TenantID                       string `json:"tenantId"`
	SubscriptionID                 string `json:"subscriptionId"`
	ActiveDirectoryEndpointURL     string `json:"activeDirectoryEndpointUrl"`
	ResourceManagerEndpointURL     string `json:"resourceManagerEndpointUrl"`
	ActiveDirectoryGraphResourceID string `json:"activeDirectoryGraphResourceId"`
}

type ClientCredentialsConfig struct {
	*auth.ClientCredentialsConfig
	SubscriptionID string
}

func NewClientCredentialsConfig(data []byte) (*ClientCredentialsConfig, error) {
	// parse credentials
	creds := credentials{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	// create a config object from the loaded credentials data
	config := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	config.AADEndpoint = creds.ActiveDirectoryEndpointURL
	config.Resource = creds.ResourceManagerEndpointURL

	return &ClientCredentialsConfig{&config, creds.SubscriptionID}, nil
}

// ValidateClient verifies if the given client is valid by testing if it can make an Azure service API call
// TODO: is there a better way to validate the Azure client?
func ValidateClient(config *ClientCredentialsConfig) error {
	authorizer, err := config.Authorizer()
	if err != nil {
		return err
	}

	groupsClient := resources.NewGroupsClient(config.SubscriptionID)
	groupsClient.Authorizer = authorizer
	if err := groupsClient.AddToUserAgent(UserAgent); err != nil {
		return err
	}

	_, err = groupsClient.ListComplete(context.TODO(), "", nil)
	return err
}

// IsErrorNotFound returns a value indicating whether the given error represents that the resource was not found.
func IsErrorNotFound(err error) bool {
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

// IsBadRequest returns a value indicating whether the given error represents bad request.
func IsErrorBadRequest(err error) bool {
	detailedError, ok := err.(autorest.DetailedError)
	if !ok {
		return false
	}

	statusCode, ok := detailedError.StatusCode.(int)
	if !ok {
		return false
	}

	return statusCode == http.StatusBadRequest
}
