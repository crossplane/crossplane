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
	"fmt"

	"github.com/ghodss/yaml"
	kubectlv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// ParseKubeconfig returns a map representing the configuration of either the
// configured current context, or the first context of a kubeconfig file.
func ParseKubeconfig(rawKubeconfig []byte) (map[string][]byte, error) {
	// unmarshal the raw kubeconfig into a strongly typed kubeconfig struct
	kubeconfig := &kubectlv1.Config{}
	if err := yaml.Unmarshal(rawKubeconfig, kubeconfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig: %+v", err)
	}

	if len(kubeconfig.Contexts) == 0 {
		// no contexts in the kubeconfig, we can't return anything meaningful
		return nil, fmt.Errorf("no contexts found in kubeconfig")
	}

	// find the current context for this kubeconfig
	var currentContext *kubectlv1.NamedContext
	if kubeconfig.CurrentContext == "" {
		// no current context set, just use the first context
		currentContext = &kubeconfig.Contexts[0]
	} else {
		// current context is set, find the matching named context
		for i, ctx := range kubeconfig.Contexts {
			if kubeconfig.CurrentContext == ctx.Name {
				currentContext = &kubeconfig.Contexts[i]
				break
			}
		}
	}

	if currentContext == nil {
		// failed to find a current context
		return nil, fmt.Errorf("failed to find current context in kubeconfig")
	}

	// find the cluster for the current context
	var cluster *kubectlv1.NamedCluster
	for i, c := range kubeconfig.Clusters {
		if currentContext.Context.Cluster == c.Name {
			cluster = &kubeconfig.Clusters[i]
			break
		}
	}

	if cluster == nil {
		// failed to find the current context's cluster
		return nil, fmt.Errorf("failed to find cluster %s in kubeconfig", currentContext.Context.Cluster)
	}

	// find the auth info for the current context
	var authInfo *kubectlv1.NamedAuthInfo
	for i, ai := range kubeconfig.AuthInfos {
		if currentContext.Context.AuthInfo == ai.Name {
			authInfo = &kubeconfig.AuthInfos[i]
			break
		}
	}

	if authInfo == nil {
		// failed to find the current context's auth info
		return nil, fmt.Errorf("failed to find auth info %s in kubeconfig", currentContext.Context.AuthInfo)
	}

	// we have a context, a cluster, and an auth info. let's fill out the cluster resource map
	kubeconfigData := map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey:   []byte(cluster.Cluster.Server),
		corev1alpha1.ResourceCredentialsSecretUserKey:       []byte(authInfo.AuthInfo.Username),
		corev1alpha1.ResourceCredentialsSecretPasswordKey:   []byte(authInfo.AuthInfo.Password),
		corev1alpha1.ResourceCredentialsSecretCAKey:         cluster.Cluster.CertificateAuthorityData,
		corev1alpha1.ResourceCredentialsSecretClientCertKey: authInfo.AuthInfo.ClientCertificateData,
		corev1alpha1.ResourceCredentialsSecretClientKeyKey:  authInfo.AuthInfo.ClientKeyData,
		corev1alpha1.ResourceCredentialsTokenKey:            []byte(authInfo.AuthInfo.Token),
	}

	return kubeconfigData, nil
}
