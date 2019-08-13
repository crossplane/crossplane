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

package gcp

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/util"
)

var log = logging.Logger.WithName("clients.gcp")

// DefaultScope is the default scope to use for a GCP client
const DefaultScope = cloudresourcemanager.CloudPlatformScope

// GetGoogleClient returns a client object that can be used to interact with the Google API
func GetGoogleClient(clientset kubernetes.Interface, namespace string, secretKey v1.SecretKeySelector,
	scopes ...string) (*http.Client, error) {

	var hc *http.Client

	// 1) look for a secret that has the credentials JSON
	gcpSecretData, err := util.SecretData(clientset, namespace, secretKey)
	if err == nil {
		var creds *google.Credentials
		creds, err = google.CredentialsFromJSON(context.Background(), gcpSecretData, scopes...)
		if err == nil {
			hc = oauth2.NewClient(context.Background(), creds.TokenSource)
		}
	}

	// 2) try the default Google client
	if hc == nil {
		log.Error(err, "failed to get google client from secret, will try default client", "secret", secretKey.Name)
		hc, err = google.DefaultClient(context.Background(), scopes...)
		if err != nil {
			log.Error(err, "failed to get default google client")
		} else {
			log.V(logging.Debug).Info("default google client created")
		}
	}

	if hc != nil {
		return hc, nil
	}

	return nil, fmt.Errorf("failed to get google client: %+v", err)
}

// IsErrorNotFound gets a value indicating whether the given error represents a "not found" response from the Google API
func IsErrorNotFound(err error) bool {
	if err == nil {
		return false
	}
	googleapiErr, ok := err.(*googleapi.Error)
	return ok && googleapiErr.Code == http.StatusNotFound
}

// IsErrorAlreadyExists gets a value indicating whether the given error represents a "conflict" response from the Google API
func IsErrorAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	googleapiErr, ok := err.(*googleapi.Error)
	return ok && googleapiErr.Code == http.StatusConflict
}

// IsErrorBadRequest gets a value indicating whether the given error represents a "bad request" response from the Google API
func IsErrorBadRequest(err error) bool {
	if err == nil {
		return false
	}
	googleapiErr, ok := err.(*googleapi.Error)
	return ok && googleapiErr.Code == http.StatusBadRequest
}

// ProviderCredentials return google credentials based on the provider's credentials secret data
func ProviderCredentials(client kubernetes.Interface, p *gcpv1alpha1.Provider, scopes ...string) (*google.Credentials, error) {
	// retrieve provider secret data
	data, err := util.SecretData(client, p.Namespace, p.Spec.Secret)
	if err != nil {
		return nil, err
	}

	return google.CredentialsFromJSON(context.Background(), data, scopes...)
}

// ProjectInfo represent GCP Project information
type ProjectInfo struct {
	// Name: The user-assigned display name of the Project.
	Name string
	// ID: The unique, user-assigned ID of the Project.
	ID string
	// Number: The number uniquely identifying the project.
	Number int64
	// CreateTime: Project Creation time.
	CreateTime string
	// Labels: The labels associated with this Project.
	Labels map[string]string
}

// Project returns project information
func Project(creds *google.Credentials) (*ProjectInfo, error) {
	ctx := context.Background()

	// Create an authenticated client.
	client := oauth2.NewClient(ctx, creds.TokenSource)

	// Create a cloud resource manager client from which we can make API calls.
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	// Retrieve project information
	p, err := crmService.Projects.Get(creds.ProjectID).Context(context.Background()).Do()
	if err != nil {
		return nil, err
	}
	return &ProjectInfo{
		Name:       p.Name,
		ID:         p.ProjectId,
		Number:     p.ProjectNumber,
		CreateTime: p.CreateTime,
		Labels:     p.Labels,
	}, nil
}

// TestPermissions tests service account permission using provided credentials and assert that it has
// all the provided permissions.
// - return nil - if all permissions are found
// - return an error - if one or more expected permissions are not found
func TestPermissions(creds *google.Credentials, permissions []string) error {
	ctx := context.Background()

	// Create an authenticated client.
	client := oauth2.NewClient(ctx, creds.TokenSource)

	// Create a cloud resource manager client from which we can make API calls.
	crmService, err := cloudresourcemanager.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	if len(permissions) > 0 {
		req := &cloudresourcemanager.TestIamPermissionsRequest{
			Permissions: permissions,
		}

		// Get the permissions for the provider user over the project ID.
		rs, err := crmService.Projects.TestIamPermissions(creds.ProjectID, req).Context(ctx).Do()
		if err != nil {
			return err
		}

		missing := getMissingPermissions(permissions, rs.Permissions)
		if len(missing) > 0 {
			return fmt.Errorf("missing permissions: %v", missing)
		}
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
