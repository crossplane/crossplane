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
	. "github.com/upbound/conductor/pkg/apis/azure/v1alpha1"
	. "k8s.io/client-go/kubernetes/fake"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/upbound/conductor/pkg/apis/azure"
	azureclient "github.com/upbound/conductor/pkg/clients/azure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace     = "azure-provider-test"
	secretName    = "test-auth-secret"
	secretDataKey = "credentials"
	providerName  = "test-provider"
	authData      = `{
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

var (
	key = types.NamespacedName{
		Namespace: namespace,
		Name:      providerName,
	}
	request = reconcile.Request{
		NamespacedName: key,
	}
)

func init() {
	azure.AddToScheme(scheme.Scheme)
}

func testSecret(data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretDataKey: data,
		},
	}
}

func testProvider() *Provider {
	return &Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
		Spec: ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  secretDataKey,
			},
		},
	}
}

func TestReconcileObjectNotFound(t *testing.T) {
	g := NewGomegaWithT(t)

	r := &Reconciler{
		Client: NewFakeClient(),
	}
	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())
}

func TestReconcileSecretNotFound(t *testing.T) {
	g := NewGomegaWithT(t)

	// test objects
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(),
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	rp := &Provider{}
	err = r.Get(ctx, key, rp)
	g.Expect(err).To(BeNil())
	g.Expect(rp.Status.IsReady()).To(BeFalse())
	g.Expect(rp.Status.IsFailed()).To(BeTrue())
}

func TestReconcileSecretKeyNotFound(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte("foo"))
	// change key
	ts.Data["testkey"] = ts.Data[secretDataKey]
	delete(ts.Data, secretDataKey)

	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	rp := &Provider{}
	err = r.Get(ctx, key, rp)
	g.Expect(err).To(BeNil())
	g.Expect(rp.Status.IsReady()).To(BeFalse())
	g.Expect(rp.Status.IsFailed()).To(BeTrue())
}

func TestReconcileValidationPassed(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte(authData))
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
		validate: func(*azureclient.Client) error {
			return nil
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(result))
	g.Expect(err).To(BeNil())

	// assert provider status
	rp := &Provider{}
	err = r.Get(ctx, key, rp)
	g.Expect(err).To(BeNil())
	g.Expect(rp.Status.IsReady()).To(BeTrue())
	g.Expect(rp.Status.IsFailed()).To(BeFalse())
}

func TestReconcileValidationFailed(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte(authData))
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
		validate: func(*azureclient.Client) error {
			return fmt.Errorf("test-error")
		},
	}

	rs, err := r.Reconcile(request)
	g.Expect(rs).To(Equal(resultRequeue))
	g.Expect(err).To(BeNil())

	// assert provider status
	rp := &Provider{}
	err = r.Get(ctx, key, rp)
	g.Expect(err).To(BeNil())
	g.Expect(rp.Status.IsReady()).To(BeFalse())
	g.Expect(rp.Status.IsFailed()).To(BeTrue())
}
