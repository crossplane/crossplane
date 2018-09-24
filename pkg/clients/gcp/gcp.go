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

package gcp

import (
	"context"
	"fmt"
	"log"
	"net/http"

	k8sclients "github.com/upbound/conductor/pkg/clients/kubernetes"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleapi "google.golang.org/api/googleapi"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GetGoogleClient returns a client object that can be used to interact with the Google API
func GetGoogleClient(clientset kubernetes.Interface, namespace string, secretKey v1.SecretKeySelector,
	scopes ...string) (*http.Client, error) {

	var hc *http.Client

	// 1) look for a secret that has the credentials JSON
	gcpSecretData, err := k8sclients.GetSecret(clientset, namespace, secretKey.Name, secretKey.Key)
	if err == nil {
		var creds *google.Credentials
		creds, err = google.CredentialsFromJSON(context.Background(), []byte(gcpSecretData), scopes...)
		if err == nil {
			hc = oauth2.NewClient(context.Background(), creds.TokenSource)
			log.Printf("google client created from secret %s", secretKey.Name)
		}
	}

	// 2) try the default Google client
	if hc == nil {
		log.Printf("failed to get google client from secret %s, will try default client: %+v", secretKey.Name, err)
		hc, err = google.DefaultClient(context.Background(), scopes...)
		if err != nil {
			log.Printf("failed to get default google client: %+v", err)
		} else {
			log.Printf("default google client created")
		}
	}

	if hc != nil {
		return hc, nil
	}

	return nil, fmt.Errorf("failed to get google client: %+v", err)
}

// IsNotFound gets a value indicating whether the given error represents a "not found" response from the Google API
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	googleapiErr, ok := err.(*googleapi.Error)
	return ok && googleapiErr.Code == http.StatusNotFound
}
