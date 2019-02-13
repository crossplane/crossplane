// /*
// Copyright 2018 The Crossplane Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package azure

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
)

// CreateOrUpdateGroup either creates a resource group or updates an already existing one by the same name
func CreateOrUpdateGroup(client *Client, name string, location string) error {
	if err := CheckResourceGroupName(name); err != nil {
		return err
	}
	groupsClient := resources.NewGroupsClient(client.SubscriptionID)
	groupsClient.Authorizer = client.Authorizer
	groupsClient.AddToUserAgent(UserAgent)
	_, err := groupsClient.CreateOrUpdate(context.TODO(), name, resources.Group{Location: &location})
	return err
}

// CheckResourceGroupName checks to make sure Resource Group name adheres to
func CheckResourceGroupName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name of resource group must be at least one character")
	}
	if len(name) > 90 {
		return fmt.Errorf("name of resource group may not be longer than 90 characters")
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("name of resource group may not end in a period")
	}
	return nil
}
