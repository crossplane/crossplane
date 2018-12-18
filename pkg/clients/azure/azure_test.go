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
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/onsi/gomega"
)

const (
	authData = `{
		"clientId": "0f32e96b-b9a4-49ce-a857-243a33b20e5c",
		"clientSecret": "49d8cab5-d47a-4d1a-9133-5c5db29c345d",
		"subscriptionId": "bf1b0e59-93da-42e0-82c6-5a1d94227911",
		"tenantId": "302de427-dba9-4452-8583-a4268e46de6b",
		"activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
		"resourceManagerEndpointUrl": "https://management.azure.com/",
		"activeDirectoryGraphResourceId": "https://graph.windows.net/",
		"sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
		"galleryEndpointUrl": "https://gallery.azure.com/",
		"managementEndpointUrl": "https://management.core.windows.net/"
}`
)

func TestNewClientCredentialsConfig(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	conf, err := NewClientCredentialsConfig([]byte(authData))
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(conf).NotTo(gomega.BeNil())
	g.Expect(conf.SubscriptionID).To(gomega.Equal("bf1b0e59-93da-42e0-82c6-5a1d94227911"))
}

func TestIsNotFound(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{autorest.DetailedError{}, false},
		{autorest.DetailedError{StatusCode: http.StatusNotFound}, true},
	}

	for _, tt := range cases {
		actual := IsErrorNotFound(tt.err)
		g.Expect(actual).To(gomega.Equal(tt.expected))
	}
}
