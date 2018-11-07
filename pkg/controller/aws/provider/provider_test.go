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
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	. "k8s.io/client-go/kubernetes/fake"
	. "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/aws/aws-sdk-go-v2/aws"
	apisaws "github.com/upbound/conductor/pkg/apis/aws"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	namespace      = "default"
	secretName     = "test-secret"
	secretDataKey  = "credentials"
	providerName   = "test-provider"
	providerRegion = "us-east-1"
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
	apisaws.AddToScheme(scheme.Scheme)
}

func testSecretData(profile, id, secret string) []byte {
	return []byte(fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s", strings.ToLower(profile), id, secret))
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

func testProvider() *awsv1alpha1.Provider {
	return &awsv1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: namespace,
		},
		Spec: awsv1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  secretDataKey,
			},
			Region: providerRegion,
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

	ts := testSecret([]byte("data-is-not-used"))
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

func TestReconcileInvalidationPassed(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte(testSecretData("default", "test-id", "test-secret")))
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
		validate: func(config *aws.Config) error {
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

func TestReconcileInvalidCredentialsFormat(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte("test"))
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
		validate: func(config *aws.Config) error {
			return nil
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

func TestReconcileValidationFailed(t *testing.T) {
	g := NewGomegaWithT(t)

	ts := testSecret([]byte(testSecretData("default", "test-id", "test-secret")))
	tp := testProvider()

	r := &Reconciler{
		Client:     NewFakeClient(tp),
		kubeclient: NewSimpleClientset(ts),
		validate: func(config *aws.Config) error {
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
