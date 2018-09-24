/*
Copyright 2018 The Conductor Authors.

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

package provider

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/container/v1"
)

// Credentials - defines provider validation functions
type Validator interface {
	Validate(secret []byte, permissions []string, projectID string) error
}

// CredentialsValidator - provides functionality for validating provider credentials
type CredentialsValidator struct{}

// Validate GCP credentials secret
func (cv *CredentialsValidator) Validate(secret []byte, permissions []string, projectID string) error {
	ctx := context.Background()

	// Parse the credentials from the JSON to create a client.
	creds, err := google.CredentialsFromJSON(ctx, secret, container.CloudPlatformScope)
	if err != nil {
		return err
	}

	// Create an authenticated client.
	hc := oauth2.NewClient(ctx, creds.TokenSource)
	if err != nil {
		return err
	}

	// Create a cloud resource manager client from which we can make API calls.
	crmService, err := cloudresourcemanager.New(hc)
	if err != nil {
		return err
	}

	rb := &cloudresourcemanager.TestIamPermissionsRequest{
		Permissions: permissions,
	}

	// Get the permissions for the provider user over the project ID.
	response, err := crmService.Projects.TestIamPermissions(projectID, rb).Context(ctx).Do()
	if err != nil {
		return err
	}

	// Verify that each permission in 'expectedPermissions' is included in the set returned in the google response.
	if missing := getMissingPermissions(permissions, response.Permissions); len(missing) > 0 {
		return fmt.Errorf("invalid credentials, missing permissions: %v", permissions)
	}

	return nil
}

// getMissingPermissions - returns a slice of expected permissions that are not found in the actual permissions set
func getMissingPermissions(expected, actual []string) (missing []string) {
	for _, p := range expected {
		found := false
		for _, pp := range actual {
			if found = p == pp; found {
				break
			}
		}
		if !found {
			missing = append(missing, p)
		}
	}
	return
}
