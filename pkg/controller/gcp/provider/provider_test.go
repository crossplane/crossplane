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

package provider

import (
	"fmt"
	"testing"

	. "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	. "github.com/onsi/gomega"
	. "k8s.io/client-go/kubernetes/fake"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplaneio/crossplane/pkg/apis/gcp"
	"golang.org/x/oauth2/google"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace       = "default"
	secretName      = "test-secret"
	secretDataKey   = "credentials"
	providerName    = "test-provider"
	providerProject = "test-project"
	authData        = `{
"type": "service_account",
"project_id": "test-project",
"private_key_id": "a1b2c3",
"private_key": "-----BEGIN PRIVATE KEY-----\nMIIsomeverylongstringVs\n-----END PRIVATE KEY-----\n",
"client_email": "user@test-project.iam.gserviceaccount.com",
"client_id": "123456789",
"auth_uri": "https://accounts.google.com/o/oauth2/auth",
"token_uri": "https://accounts.google.com/o/oauth2/token",
"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/gke-operator%40test-project.iam.gserviceaccount.com"
}
`
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
	gcp.AddToScheme(scheme.Scheme)
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
			ProjectID: providerProject,
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

func TestReconcileValidationPassed(t *testing.T) {
	g := NewGomegaWithT(t)

	r := &Reconciler{
		Client:     NewFakeClient(testProvider()),
		kubeclient: NewSimpleClientset(testSecret([]byte(authData))),
		validate: func(*google.Credentials, []string) error {
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

	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(),
		validate: func(*google.Credentials, []string) error {
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
