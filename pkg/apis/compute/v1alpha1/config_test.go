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

package v1alpha1

import (
	"testing"

	"github.com/onsi/gomega"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

const (
	// This data format is the general kubectl config (kubeconfig) format, but it specifically
	// came from the Azure AKS ListClusterAdminCredentials API:
	// https://docs.microsoft.com/en-us/rest/api/aks/managedclusters/listclusteradmincredentials
	// The values in it have all been replaced with fake data
	mockRawClusterCredentialsData = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: Y2VydGlmaWNhdGUtYXV0aG9yaXR5LWRhdGEtdmFsdWU=
    server: https://crossplane-aks-55e038af.hcp.westus2.azmk8s.io:443
  name: aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
contexts:
- context:
    cluster: aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
    user: clusterAdmin_rg-123_aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
    namespace: foo
  name: aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
current-context: aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
kind: Config
preferences: {}
users:
- name: clusterAdmin_rg-123_aks-c7f893e4-d903-402c-ba60-8be6bbeac6a3
  user:
    client-certificate-data: Y2xpZW50LWNlcnRpZmljYXRlLWRhdGEtdmFsdWU=
    client-key-data: Y2xpZW50LWtleS1kYXRhLXZhbHVl
    token: 799e6a56219da5ff09e3b41b1dd08f3f
`
)

func TestParseKubeconfig(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// the unmarshal of the raw data will decode the base64 encoded values, so we expect them in plain-text
	expectedKubeconfigData := map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey:   []byte("https://crossplane-aks-55e038af.hcp.westus2.azmk8s.io:443"),
		corev1alpha1.ResourceCredentialsSecretUserKey:       []byte(""),
		corev1alpha1.ResourceCredentialsSecretPasswordKey:   []byte(""),
		corev1alpha1.ResourceCredentialsSecretCAKey:         []byte("certificate-authority-data-value"),
		corev1alpha1.ResourceCredentialsSecretClientCertKey: []byte("client-certificate-data-value"),
		corev1alpha1.ResourceCredentialsSecretClientKeyKey:  []byte("client-key-data-value"),
		corev1alpha1.ResourceCredentialsTokenKey:            []byte("799e6a56219da5ff09e3b41b1dd08f3f"),
	}

	kubeconfigData, err := ParseKubeconfig([]byte(mockRawClusterCredentialsData))
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(kubeconfigData).To(gomega.Equal(expectedKubeconfigData))
}
