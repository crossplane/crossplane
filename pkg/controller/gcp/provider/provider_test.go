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
	"time"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// MockClient - kubernetes client
type MockClient struct {
	client.Client
}

// Update - mock update that never fails
func (mr *MockClient) Update(ctx context.Context, obj runtime.Object) error {
	return nil
}

// MockManager - controller manager
type MockManager struct {
	manager.Manager
	realManager manager.Manager
}

// GetClient - return mocked client
func (mm *MockManager) GetClient() client.Client {
	return &MockClient{mm.realManager.GetClient()}
}

// MockValidator - validates credentials
type MockValidator struct{}

// Validate - never fails
func (mv *MockValidator) Validate(secret []byte, permissions []string, projectID string) error {
	return nil
}

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-bar",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"credentials.json": []byte("Zm9vLWJhcgo="),
		},
	}

	instance := &v1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: secret.Namespace,
		},
		Spec: v1alpha1.ProviderSpec{
			SecretKey: v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{Name: "foo-bar"},
				Key:                  "credentials.json",
			},
			ProjectID: "projectfoo",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	c = mgr.GetClient()
	k = kubernetes.NewForConfigOrDie(mgr.GetConfig())

	mm := &MockManager{mgr, mgr}
	r := newReconciler(mm, &MockValidator{})

	recFn, requests := SetupTestReconcile(r)
	g.Expect(add(mgr, recFn)).NotTo(HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create secret
	_, err = k.CoreV1().Secrets(secret.Namespace).Create(secret)
	g.Expect(err).NotTo(HaveOccurred())

	// Create the Provider object and expect the Reconcile to run
	err = c.Create(context.TODO(), instance)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

	// Fetch created instance
	created := &v1alpha1.Provider{}
	err = c.Get(context.TODO(), expectedRequest.NamespacedName, created)
	g.Expect(err).NotTo(HaveOccurred())

	// Manually delete Deployment since GC isn't enabled in the test control plane
	g.Expect(c.Delete(context.TODO(), instance)).To(Succeed())

}

func TestMissingPermissions(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(getMissingPermissions([]string{}, []string{})).To(BeNil())
	g.Expect(getMissingPermissions([]string{"a"}, []string{})).To(Equal([]string{"a"}))
	g.Expect(getMissingPermissions([]string{"a", "a"}, []string{})).To(Equal([]string{"a", "a"}))
	g.Expect(getMissingPermissions([]string{"a", "a"}, []string{"a"})).To(BeNil())
	g.Expect(getMissingPermissions([]string{"a", "b"}, []string{"a"})).To(Equal([]string{"b"}))
}
