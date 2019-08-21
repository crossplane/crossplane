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

package compute

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/crossplaneio/crossplane/azure/apis"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/test"
	"github.com/crossplaneio/crossplane/azure/apis/compute/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/compute/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
)

const (
	timeout       = 5 * time.Second
	namespace     = "test-compute-namespace"
	instanceName  = "test-compute-instance"
	secretName    = "test-secret"
	secretDataKey = "credentials"
	providerName  = "test-provider"

	clientEndpoint = "https://example.org"
	clientCAdata   = "DEFINITELYPEMENCODED"
	clientCert     = "SOMUCHPEM"
	clientKey      = "WOWVERYENCODED"
)

const kubeconfigTemplate = `
---
apiVersion: v1
kind: Config
contexts:
- context:
    cluster: aks
    user: aks
  name: %s
clusters:
- cluster:
    server: %s
    certificate-authority-data: %s
  name: aks
users:
- name: aks
  user:
    client-certificate-data: %s
    client-key-data: %s
current-context: aks
preferences: {}
`

var (
	cfg             *rest.Config
	expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: instanceName, Namespace: namespace}}

	kubecfg = []byte(fmt.Sprintf(kubeconfigTemplate,
		instanceName,
		clientEndpoint,
		base64.StdEncoding.EncodeToString([]byte(clientCAdata)),
		base64.StdEncoding.EncodeToString([]byte(clientCert)),
		base64.StdEncoding.EncodeToString([]byte(clientKey)),
	))
)

func TestMain(m *testing.M) {
	t := test.NewEnv(namespace, apis.AddToSchemes, test.CRDs())
	cfg = t.Start()
	t.StopAndExit(m.Run())
}

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager, g *gomega.GomegaWithT) chan struct{} {
	stop := make(chan struct{})
	go func() {
		g.Expect(mgr.Start(stop)).NotTo(gomega.HaveOccurred())
	}()
	return stop
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

func testProvider(s *corev1.Secret) *azurev1alpha1.Provider {
	return &azurev1alpha1.Provider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerName,
			Namespace: s.Namespace,
		},
		Spec: azurev1alpha1.ProviderSpec{
			Secret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  secretDataKey,
			},
		},
	}
}

func testInstance(p *azurev1alpha1.Provider) *computev1alpha1.AKSCluster {
	return &computev1alpha1.AKSCluster{
		ObjectMeta: metav1.ObjectMeta{Name: instanceName, Namespace: namespace},
		Spec: computev1alpha1.AKSClusterSpec{
			ResourceSpec: runtimev1alpha1.ResourceSpec{
				ReclaimPolicy:                    runtimev1alpha1.ReclaimDelete,
				ProviderReference:                meta.ReferenceTo(p, azurev1alpha1.ProviderGroupVersionKind),
				WriteConnectionSecretToReference: corev1.LocalObjectReference{Name: "coolSecret"},
			},
			AKSClusterParameters: v1alpha1.AKSClusterParameters{
				WriteServicePrincipalSecretTo: corev1.LocalObjectReference{Name: "coolPrincipal"},
				ResourceGroupName:             "rg1",
				Location:                      "loc1",
				Version:                       "1.12.5",
				NodeCount:                     to.IntPtr(3),
				NodeVMSize:                    "Standard_F2s_v2",
				DNSNamePrefix:                 "crossplane-aks",
				DisableRBAC:                   false,
			},
		},
	}
}
