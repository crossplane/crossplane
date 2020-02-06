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

package hosted

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	errMissingOption = "host aware mode activated but %s is not set"
)

// Config is the configuration for Host Aware Mode where different Kubernetes API's are used for pod
// scheduling and custom resources.
type Config struct {
	// HostControllerNamespace is the namespace on Host Cluster where install and controller jobs/deployments will be
	// deployed.
	HostControllerNamespace string
	// TenantAPIServiceHost is Kubernetes Apiserver Host for custom resources (a.k.a Tenant Kubernetes)
	TenantAPIServiceHost string
	// TenantAPIServicePort is Kubernetes Apiserver Port for custom resources (a.k.a Tenant Kubernetes)
	TenantAPIServicePort string
}

// NewConfig returns a new host aware config based on the input parameters.
func NewConfig(hostControllerNamespace, tenantAPIServiceHost, tenantAPIServicePort string) (*Config, error) {
	if hostControllerNamespace == "" {
		return nil, errors.Errorf(errMissingOption, "hostControllerNamespace")
	}
	if tenantAPIServiceHost == "" {
		return nil, errors.Errorf(errMissingOption, "tenantAPIServiceHost")
	}
	if tenantAPIServicePort == "" {
		return nil, errors.Errorf(errMissingOption, "tenantAPIServicePort")
	}

	return &Config{
		HostControllerNamespace: hostControllerNamespace,
		TenantAPIServiceHost:    tenantAPIServiceHost,
		TenantAPIServicePort:    tenantAPIServicePort,
	}, nil
}

// ObjectReferenceOnHost maps object with given name and namespace into single controller namespace on Host Cluster.
func (c *Config) ObjectReferenceOnHost(name, namespace string) corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:      fmt.Sprintf("%s.%s", namespace, name),
		Namespace: c.HostControllerNamespace,
	}
}

// NewConfigForHost returns a host aware config given a controller namespace
// and a Host string, assumed to be in the format accepted by rest.Config. It
// returns a nil Config if either the supplied namespace or host are empty.
// https://pkg.go.dev/k8s.io/client-go/rest?tab=doc#Config
func NewConfigForHost(hostControllerNamespace, host string) (*Config, error) {
	if hostControllerNamespace == "" || host == "" {
		return nil, nil
	}

	hostname, port, err := getHostPort(host)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get host port from tenant kubeconfig")
	}

	hc, err := NewConfig(hostControllerNamespace, hostname, port)
	return hc, errors.Wrap(err, "cannot create hosted config")

}

func getHostPort(urlHost string) (host string, port string, err error) {
	u, err := url.Parse(urlHost)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot parse URL")
	}

	if u.Port() == "" {
		if u.Scheme == "http" {
			return u.Host, "80", nil
		}
		if u.Scheme == "https" {
			return u.Host, "443", nil
		}
	}
	return u.Hostname(), u.Port(), nil
}
