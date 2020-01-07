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
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// GetClients is the function to get Host Kubernetes (where install and controller pods scheduled) clients with in
// cluster config. This function is called regardless of hosted mode being enabled:
// Hosted Mode Off (Standard Installation):
// - resource (tenant) kube client => in cluster config
// - host kube clients => in cluster config
// Hosted Mode On:
// - resource (tenant) kube client => via EnvTenantKubeconfig
// - host kube clients => in cluster config
func GetClients() (client.Client, *kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host config with in cluster config")
		return nil, nil, err
	}
	hostKube, err := client.New(cfg, client.Options{})
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host client with in cluster config")
		return nil, nil, err
	}
	hostClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		err = errors.Wrap(err, "failed to initialize host clientset with in cluster config")
		return nil, nil, err
	}

	return hostKube, hostClient, nil
}
