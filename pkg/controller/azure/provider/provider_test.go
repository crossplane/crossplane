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
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	azureclient "github.com/upbound/conductor/pkg/clients/azure"
	corev1 "k8s.io/api/core/v1"
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

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	mgr.reconciler.Validator = &MockValidator{}
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// create auth secret, defer deletion
	authSecret, err := mgr.createSecret(testSecret(authSecretName, authSecretDataKey, []byte(authData)))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(authSecret)

	// create provider, defer its deletion
	p := testProvider()
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// run a single reconciliation
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// assert that the provider has the Valid status condition
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	validCondition := rp.Status.GetCondition(corev1alpha1.Valid)
	g.Expect(validCondition).NotTo(BeNil())
	g.Expect(validCondition.Status).To(Equal(corev1.ConditionTrue))
}

func TestReconcileNoAuthSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	mgr.reconciler.Validator = &MockValidator{}
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// do not create the auth secret

	// create provider, defer its deletion
	p := testProvider()
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// run a single reconciliation
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// assert that the provider is in an invalid condition due to the missing auth secret
	assertInvalid(g, mgr, errorCreatingClient)
}

func TestReconcileValidationFailure(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager with a mock validator that will return an error
	mgr, err := NewTestManager()
	mgr.reconciler.Validator = &MockValidator{
		MockValidate: func(*azureclient.Client) error { return fmt.Errorf("mock validate error") },
	}
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// create auth secret, defer deletion
	authSecret, err := mgr.createSecret(testSecret(authSecretName, authSecretDataKey, []byte(authData)))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(authSecret)

	// create provider, defer its deletion
	p := testProvider()
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// run a single reconciliation
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// assert that the provider is in an invalid condition due to authorizing failing its validation test
	assertInvalid(g, mgr, errorTestingClient)
}

func assertInvalid(g *GomegaWithT, mgr *TestManager, expectedReason string) {
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	validCondition := rp.Status.GetCondition(corev1alpha1.Valid)
	g.Expect(validCondition).To(BeNil())
	invalidCondition := rp.Status.GetCondition(corev1alpha1.Invalid)
	g.Expect(invalidCondition).ToNot(BeNil())
	g.Expect(invalidCondition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(invalidCondition.Reason).To(Equal(expectedReason))
}
