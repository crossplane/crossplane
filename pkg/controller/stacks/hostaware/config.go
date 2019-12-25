package hostaware

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	envTenantKubeconfig            = "TENANT_KUBECONFIG"
	envControllerNamespace         = "CONTROLLER_NAMESPACE"
	envTenantKubernetesServiceHost = "TENANT_KUBERNETES_SERVICE_HOST"
	envTenantKubernetesServicePort = "TENANT_KUBERNETES_SERVICE_PORT"

	errMissingEnvVar = "host aware mode activated but %s env var is not set"
)

// Config is the configuration for Host Aware Mode where different Kubernetes API's are used for workload
// scheduling and custom resources.
type Config struct {
	HostControllerNamespace string
	TenantAPIServiceHost    string
	TenantAPIServicePort    string
}

// NewConfig returns a new HostAwareConfig based on the available environment variables.
func NewConfig() (*Config, error) {
	tenantKubeconfig := os.Getenv(envTenantKubeconfig)
	if tenantKubeconfig == "" {
		return nil, nil
	}
	ns := os.Getenv(envControllerNamespace)
	if ns == "" {
		return nil, errors.New(fmt.Sprintf(errMissingEnvVar, envControllerNamespace))
	}
	apiHost := os.Getenv(envTenantKubernetesServiceHost)
	if apiHost == "" {
		return nil, errors.New(fmt.Sprintf(errMissingEnvVar, envTenantKubernetesServiceHost))
	}
	apiPort := os.Getenv(envTenantKubernetesServicePort)
	if apiPort == "" {
		return nil, errors.New(fmt.Sprintf(errMissingEnvVar, envTenantKubernetesServicePort))
	}

	return &Config{
		HostControllerNamespace: ns,
		TenantAPIServiceHost:    apiHost,
		TenantAPIServicePort:    apiPort,
	}, nil
}

// ObjectReferenceOnHost maps object with given name and namespace into single controller namespace
func (c *Config) ObjectReferenceOnHost(name, namespace string) corev1.ObjectReference {
	return corev1.ObjectReference{
		Name:      fmt.Sprintf("%s-%s", namespace, name),
		Namespace: c.HostControllerNamespace,
	}
}
