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
	"testing"

	"github.com/go-ini/ini"
	. "github.com/onsi/gomega"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/controller/core/provider"
	corev1 "k8s.io/api/core/v1"
)

// TestReconcileNoSecret - AWS Provider instance refers to non-existent secret
func TestReconcileNoSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create instance
	p := testProvider(testSecret([]byte("test-secret-data")))
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(rp.Status.Conditions).To(BeNil())
}

// TestReconcileInvalidSecretDataKey - AWS Provider is configured with secret key that does not exist in the
// actual secret
func TestReconcileInvalidSecretDataKey(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret with invalid data key
	s := testSecret(testSecretData(ini.DEFAULT_SECTION, "test-id", "test-secret"))
	s.Data["invalid-key"] = s.Data[secretDataKey]
	delete(s.Data, secretDataKey)
	s, err = mgr.createSecret(s)
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create provider
	p := testProvider(s)
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(rp.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(rp.Status, corev1alpha1.Invalid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(Equal("invalid AWS Provider secret, data key [credentials] is not found"))
}

// TestReconcileInvalidSecretCredentialsProfile - AWS Provider is configured with non-existent AwS credentials profile
func TestReconcileInvalidSecretCredentialsProfile(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	s, err := mgr.createSecret(testSecret(testSecretData("invalid-profile", "test-id", "test-secret")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create provider
	p := testProvider(s)
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(rp.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(rp.Status, corev1alpha1.Invalid)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(ContainSubstring("error when getting key of section 'default'"))
}

// TestReconcileInvalidCredentials - AWS Provider secret contains invalid AWS credentials
func TestReconcileInvalidCredentials(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	s, err := mgr.createSecret(testSecret(testSecretData(ini.DEFAULT_SECTION, "test-id", "test-secret")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create provider
	p := testProvider(s)
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(rp.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(rp.Status, corev1alpha1.Invalid)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(And(
		ContainSubstring("InvalidAccessKeyId: The AWS Access Key Id you provided does not exist in our records."),
		ContainSubstring("status code: 403")))
}

// TestReconcileValidMock - valid reconciliation loop with Validation mock
func TestReconcileValidMock(t *testing.T) {
	g := NewGomegaWithT(t)

	// create and start manager
	mgr, err := NewTestManager()
	mgr.reconciler.Validator = &MockValidator{}
	g.Expect(err).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	s, err := mgr.createSecret(testSecret(testSecretData(ini.DEFAULT_SECTION, "test-id", "test-secret")))
	g.Expect(err).NotTo(HaveOccurred())
	defer mgr.deleteSecret(s)

	// Create provider
	p := testProvider(s)
	g.Expect(mgr.createProvider(p)).NotTo(HaveOccurred())
	defer mgr.deleteProvider(p)

	// Reconcile loop
	g.Eventually(mgr.requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	rp, err := mgr.getProvider()
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(rp.Status, corev1alpha1.Invalid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(rp.Status, corev1alpha1.Valid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
}
