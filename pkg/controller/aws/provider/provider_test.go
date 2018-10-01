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
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-ini/ini"

	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/controller/core/provider"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client
var k kubernetes.Interface
var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

const timeout = 5 * time.Second

// MockValidator - validates credentials
type MockValidator struct{}

// Validate - never fails
func (mv *MockValidator) Validate(config *aws.Config) error {
	return nil
}

// Secret helper function to create AWS provider secret
func Secret(key, profile, id, secret string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-bar",
			Namespace: "default",
		},
		Data: map[string][]byte{
			key: []byte(fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s", strings.ToLower(profile), id, secret)),
		},
	}
}

// Provider helper function to create AWS provider instance
func Provider(key, region string) *v1alpha1.Provider {
	return &v1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: v1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "foo-bar"},
				Key:                  key,
			},
			Region: region,
		},
	}
}

// TestReconcileNoSecret - AWS Provider instance refers to non-existent secret
func TestReconcileNoSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &ConfigurationValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create instance
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(reconciledInstance.Status.Conditions).To(BeNil())
}

// TestReconcileInvalidSecretDataKey - AWS Provider is configured with secret key that does not exist in the
// actual secret
func TestReconcileInvalidSecretDataKey(t *testing.T) {
	g := NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &ConfigurationValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	secret := Secret("creds", ini.DEFAULT_SECTION, "test-id", "test-secret")
	secret, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())
	defer k.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

	// Create instance
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(reconciledInstance.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(reconciledInstance.Status, corev1alpha1.Invalid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(Equal("invalid AWS Provider secret, data key [credentials] is not found"))
}

// TestReconcileInvalidSecretCredentialsProfile - AWS Provider is configured with non-existent AwS credentials profile
func TestReconcileInvalidSecretCredentialsProfile(t *testing.T) {
	g := NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &ConfigurationValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	secret := Secret("credentials", "foo-bar", "test-id", "test-secret")
	secret, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())
	defer k.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

	// Create instance
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(reconciledInstance.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(reconciledInstance.Status, corev1alpha1.Invalid)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(ContainSubstring("error when getting key of section 'default'"))
}

// TestReconcileInvalidCredentials - AWS Provider secret contains invalid AWS credentials
func TestReconcileInvalidCredentials(t *testing.T) {
	g := NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &ConfigurationValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	secret := Secret("credentials", ini.DEFAULT_SECTION, "test-id", "test-secret")
	secret, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())
	defer k.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

	// Create instance - secret doesn't exit yet
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(reconciledInstance.Status, corev1alpha1.Valid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(reconciledInstance.Status, corev1alpha1.Invalid)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
	g.Expect(condition.Reason).To(And(
		ContainSubstring("InvalidAccessKeyId: The AWS Access Key Id you provided does not exist in our records."),
		ContainSubstring("status code: 403")))
}

// TestReconcileValidMock - valid reconciliation loop with Validation mock
func TestReconcileValidMock(t *testing.T) {
	g := NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &MockValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	secret := Secret("credentials", ini.DEFAULT_SECTION, "test-id", "test-secret")
	secret, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())
	defer k.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

	// Create instance - secret doesn't exit yet
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(reconciledInstance.Status, corev1alpha1.Invalid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(reconciledInstance.Status, corev1alpha1.Valid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
}

// TestReconcileValid - reads AWS configuration from the local file.
// The file path is provided via TEST_AWS_CREDENTIALS_FILE environment variable, otherwise the test is skipped.
func TestReconcileValid(t *testing.T) {
	g := NewGomegaWithT(t)

	awsCredsFile := os.Getenv("TEST_AWS_CREDENTIALS_FILE")
	if awsCredsFile == "" {
		t.Log("not found: TEST_AWS_CREDENTIALS_FILE")
		t.Skip()
	}

	data, err := ioutil.ReadFile(awsCredsFile)
	g.Expect(err).NotTo(HaveOccurred())

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())
	r := newReconciler(mgr, &ConfigurationValidator{})
	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-bar",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials": data,
		},
	}
	secret, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())
	defer k.CoreV1().Secrets(secret.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

	// Create instance - secret doesn't exit yet
	instance := Provider("credentials", "us-west-2")
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Assert
	reconciledInstance := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, reconciledInstance)
	g.Expect(err).NotTo(HaveOccurred())
	condition := provider.GetCondition(reconciledInstance.Status, corev1alpha1.Invalid)
	g.Expect(condition).To(BeNil())
	condition = provider.GetCondition(reconciledInstance.Status, corev1alpha1.Valid)
	g.Expect(condition.Status).To(Equal(corev1.ConditionTrue))
}
