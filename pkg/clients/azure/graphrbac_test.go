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
	"io/ioutil"
	"os"
	"testing"

	"github.com/crossplaneio/crossplane/pkg/util"
	. "github.com/onsi/gomega"
)

const (
	TestAssetAzureCredsFile = "AZURE_CREDS_FILE"
)

// AzureCredsData loaded from the environment value if provide, otherwise the test is skipped
func AzureCredsDataOrSkip(t *testing.T) []byte {
	g := NewGomegaWithT(t)

	file := os.Getenv(TestAssetAzureCredsFile)
	if file == "" {
		t.Skipf("test asset %s environment variable is not provided", TestAssetAzureCredsFile)
	}

	data, err := ioutil.ReadFile(file)
	g.Expect(err).NotTo(HaveOccurred())
	return data
}

func randomName(g *GomegaWithT, prefix string) string {
	suffix, err := util.GenerateHex(3)
	g.Expect(err).NotTo(HaveOccurred())
	return prefix + suffix
}

func TestApplication(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()

	// load local creds or skip the test
	data := AzureCredsDataOrSkip(t)
	config, err := NewClientCredentialsConfig(data)
	g.Expect(err).NotTo(HaveOccurred())

	appClient, err := NewApplicationClient(config)
	g.Expect(err).NotTo(HaveOccurred())

	name := randomName(g, "test-one-")
	url := "https://" + name + ".crossplane.io"

	// Generate new password creds
	password := NewPasswordCredential("test")

	// Test creating the app
	app, err := appClient.CreateApplication(ctx, name, url, password)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*app.DisplayName).To(Equal(name))
	g.Expect(*app.IdentifierUris).To(ContainElement(url))
	g.Expect(*app.AppID).NotTo(BeEmpty())
	g.Expect(*app.PasswordCredentials).NotTo(BeEmpty())
	g.Expect(*(*app.PasswordCredentials)[0].KeyID).To(Equal(*password.KeyID))

	// Test retrieving the app by id
	getApp, err := appClient.GetApplication(ctx, *app.ObjectID)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*getApp.PasswordCredentials).NotTo(BeEmpty())
	g.Expect(*(*getApp.PasswordCredentials)[0].KeyID).To(Equal(*password.KeyID))
	g.Expect((*getApp.PasswordCredentials)[0].Value).To(BeNil())

	// Test deleting app
	g.Expect(appClient.DeleteApplication(ctx, *app.ObjectID)).To(Succeed())

	// Test retrieving non-existing app
	getApp, err = appClient.GetApplication(ctx, *app.ObjectID)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsErrorNotFound(err)).To(BeTrue())
}

func TestServicePrincipal(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()

	// load local creds or skip the test
	data := AzureCredsDataOrSkip(t)
	config, err := NewClientCredentialsConfig(data)
	g.Expect(err).NotTo(HaveOccurred())

	spClient, err := NewServicePrincipalClient(config)
	g.Expect(err).NotTo(HaveOccurred())

	// Test creating SP w/ non-existing application
	sp, err := spClient.CreateServicePrincipal(ctx, "foo")
	g.Expect(err).To(HaveOccurred())
	g.Expect(sp).To(BeNil())

	// Crate Application
	appClient, err := NewApplicationClient(config)
	g.Expect(err).NotTo(HaveOccurred())

	name := randomName(g, "test-one-")
	url := "https://" + name + ".crossplane.io"
	app, err := appClient.CreateApplication(ctx, name, url, NewPasswordCredential("test"))
	g.Expect(err).NotTo(HaveOccurred())
	defer appClient.DeleteApplication(ctx, *app.ObjectID)

	// Test creating SP with existing application
	sp, err = spClient.CreateServicePrincipal(ctx, *app.AppID)
	g.Expect(err).NotTo(HaveOccurred())

	// Test retrieving SP by object ID
	_, err = spClient.GetServicePrincipal(ctx, *sp.ObjectID)
	g.Expect(err).To(Succeed())

	// Test deleting SP by object ID
	g.Expect(spClient.DeleteServicePrincipal(ctx, *sp.ObjectID)).To(Succeed())

	// Test retrieving non-existing SP
	_, err = spClient.GetServicePrincipal(ctx, *sp.ObjectID)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsErrorNotFound(err)).To(BeTrue())

}
