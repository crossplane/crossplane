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

package resourcegroup

import (
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources/resourcesapi"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// A GroupsClient handles CRUD operations for Azure Resource Group resources.
type GroupsClient resourcesapi.GroupsClientAPI

// NewClient returns a new Azure Resource Groups client. Credentials must be
// passed as JSON encoded data.
func NewClient(credentials []byte) (GroupsClient, error) {
	c := azure.Credentials{}
	if err := json.Unmarshal(credentials, &c); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}
	client := resources.NewGroupsClient(c.SubscriptionID)

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
	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// NewParameters returns Resource Group resource creation parameters suitable for
// use with the Azure API.
func NewParameters(r *v1alpha1.ResourceGroup) resources.Group {
	return resources.Group{
		Name:     azure.ToStringPtr(r.Spec.Name),
		Location: azure.ToStringPtr(r.Spec.Location),
	}
}
