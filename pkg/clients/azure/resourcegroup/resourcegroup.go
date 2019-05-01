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

package resourcegroup

import (
	"encoding/json"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources/resourcesapi"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"

	"github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/azure"
)

// Resource group naming errors
const (
	NameTooShort  = "name of resource group must be at least one character"
	NameTooLong   = "name of resource group may not be longer than 90 characters"
	NameEndPeriod = "name of resource group may not end in a period"
	NameRegex     = "name of resource group is not well-formed per https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/createorupdate"
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

// CheckResourceGroupName checks to make sure Resource Group name adheres to
func CheckResourceGroupName(name string) error {
	if len(name) == 0 {
		return errors.New(NameTooShort)
	}
	if len(name) > 90 {
		return errors.New(NameTooLong)
	}
	if name[len(name)-1:] == "." {
		return errors.New(NameEndPeriod)
	}
	if matched, _ := regexp.MatchString(`^[-\w\._\(\)]+$`, name); !matched {
		return errors.New(NameRegex)
	}
	return nil
}
