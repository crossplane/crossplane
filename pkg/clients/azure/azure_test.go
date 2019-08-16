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

package azure

import (
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
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

func TestNewClient(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	clientset := fake.NewSimpleClientset()

	namespace := "foo-ns"
	provider := &v1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azure-provider",
			Namespace: namespace,
		},
		Spec: v1alpha1.ProviderSpec{
			Secret: v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{Name: "azure-provider-creds"},
				Key:                  "creds",
			},
		},
	}

	// get client when secret doesn't exist, this should fail
	client, err := NewClient(provider, clientset)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(client).To(gomega.BeNil())

	// create the auth secret now
	authSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      provider.Spec.Secret.Name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			provider.Spec.Secret.Key: []byte(authData),
		},
	}
	clientset.CoreV1().Secrets(namespace).Create(authSecret)

	// now that the secret exists, getting a client should succeed
	client, err = NewClient(provider, clientset)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(client).NotTo(gomega.BeNil())
	g.Expect(client.SubscriptionID).To(gomega.Equal("bf1b0e59-93da-42e0-82c6-5a1d94227911"))
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
		actual := IsNotFound(tt.err)
		g.Expect(actual).To(gomega.Equal(tt.expected))
	}
}
