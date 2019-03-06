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
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	// UserAgent is the user agent extension that identifies the Crossplane Azure client
	UserAgent = "crossplane-azure-client"
)

// A FieldOption determines how common Go types are translated to the types
// required by the Azure Go SDK.
type FieldOption int

// Field options.
const (
	// FieldRequired causes zero values to be converted to a pointer to the zero
	// value, rather than a nil pointer. Azure Go SDK types use pointer fields,
	// with a nil pointer indicating an unset field. Our ToPtr functions return
	// a nil pointer for a zero values, unless FieldRequired is set.
	FieldRequired FieldOption = iota
)

// Client struct that represents the information needed to connect to the Azure services as a client
type Client struct {
	autorest.Authorizer
	SubscriptionID                 string
	clientID                       string
	clientSecret                   string
	tenantID                       string
	activeDirectoryEndpointURL     string
	activeDirectoryGraphResourceID string
}

// Credentials represents the contents of a JSON encoded Azure credentials file.
// It is a subset of the internal type used by the Azure auth library.
// https://github.com/Azure/go-autorest/blob/be17756/autorest/azure/auth/auth.go#L226
type Credentials struct {
	ClientID                       string `json:"clientId"`
	ClientSecret                   string `json:"clientSecret"`
	TenantID                       string `json:"tenantId"`
	SubscriptionID                 string `json:"subscriptionId"`
	ActiveDirectoryEndpointURL     string `json:"activeDirectoryEndpointUrl"`
	ResourceManagerEndpointURL     string `json:"resourceManagerEndpointUrl"`
	ActiveDirectoryGraphResourceID string `json:"activeDirectoryGraphResourceId"`
}

// NewClient will look up the Azure credential information from the given provider and return a client
// that can be used to connect to Azure services.
func NewClient(provider *v1alpha1.Provider, clientset kubernetes.Interface) (*Client, error) {
	// first get the secret data that should contain all the auth/creds information
	azureSecretData, err := util.SecretData(clientset, provider.Namespace, provider.Spec.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get azure client secret: %+v", err)
	}

	// load Credentials from json data
	creds := Credentials{}
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

	return &Client{
		Authorizer:                     authorizer,
		SubscriptionID:                 creds.SubscriptionID,
		clientID:                       creds.ClientID,
		clientSecret:                   creds.ClientSecret,
		tenantID:                       creds.TenantID,
		activeDirectoryEndpointURL:     creds.ActiveDirectoryEndpointURL,
		activeDirectoryGraphResourceID: creds.ActiveDirectoryGraphResourceID,
	}, nil
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

// ToStringPtr converts the supplied string for use with the Azure Go SDK.
func ToStringPtr(s string, o ...FieldOption) *string {
	for _, fo := range o {
		if fo == FieldRequired && s == "" {
			return to.StringPtr(s)
		}
	}

	if s == "" {
		return nil
	}

	return to.StringPtr(s)
}

// ToInt32Ptr converts the supplied int for use with the Azure Go SDK.
func ToInt32Ptr(i int, o ...FieldOption) *int32 {
	for _, fo := range o {
		if fo == FieldRequired && i == 0 {
			return to.Int32Ptr(int32(i))
		}
	}

	if i == 0 {
		return nil
	}
	return to.Int32Ptr(int32(i))
}

// ToBoolPtr converts the supplied bool for use with the Azure Go SDK.
func ToBoolPtr(b bool, o ...FieldOption) *bool {
	for _, fo := range o {
		if fo == FieldRequired && !b {
			return to.BoolPtr(b)
		}
	}

	if !b {
		return nil
	}
	return to.BoolPtr(b)
}

// ToStringPtrMap converts the supplied map for use with the Azure Go SDK.
func ToStringPtrMap(m map[string]string) map[string]*string {
	return *(to.StringMapPtr(m))
}

// ToString converts the supplied pointer to string to a string, returning the
// empty string if the pointer is nil.
func ToString(s *string) string {
	return to.String(s)
}

// ToInt converts the supplied pointer to int32 to an int, returning zero if the
// pointer is nil,
func ToInt(i *int32) int {
	return int(to.Int32(i))
}
