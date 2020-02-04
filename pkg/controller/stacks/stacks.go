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

package stacks

import (
	"net/url"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/hosted"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/install"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/stack"
)

// Controllers passes down config and adds individual controllers to the manager.
type Controllers struct{}

// SetupWithManager adds all Stack controllers to the manager.
func (c *Controllers) SetupWithManager(mgr ctrl.Manager, hostControllerNamespace string) error {
	creators := []func() (string, func() v1alpha1.StackInstaller){
		newStackInstall, newClusterStackInstall,
	}

	smo, err := getSMOptions(mgr.GetConfig().Host, hostControllerNamespace)
	if err != nil {
		return err
	}

	for _, creator := range creators {
		if err := (&install.Controller{
			StackInstallCreator: creator,
		}).SetupWithManager(mgr, smo...); err != nil {
			return err
		}
	}

	if err := (&stack.Controller{}).SetupWithManager(mgr, smo...); err != nil {
		return err
	}

	return nil
}

// StackInstall and ClusterStackInstall controllers differ by only their name and the type they accept
// These differences have been abstracted away through StackInstaller so they can be treated the same.
func newStackInstall() (string, func() v1alpha1.StackInstaller) {
	return "stackinstall.stacks.crossplane.io", func() v1alpha1.StackInstaller { return &v1alpha1.StackInstall{} }
}

func newClusterStackInstall() (string, func() v1alpha1.StackInstaller) {
	return "clusterstackinstall.stacks.crossplane.io", func() v1alpha1.StackInstaller { return &v1alpha1.ClusterStackInstall{} }
}

func getSMOptions(server, hostControllerNamespace string) ([]stack.SMReconcilerOption, error) {
	var smo []stack.SMReconcilerOption
	if hostControllerNamespace != "" {
		//hostControllerNamespace is set => stack manager host aware mode enabled
		host, port, err := getHostPort(server)
		if err != nil {
			return nil, errors.Wrap(err, "Cannot get host port from tenant kubeconfig")
		}
		hc, err := hosted.NewConfig(hostControllerNamespace, host, port)
		if err != nil {
			return nil, err
		}
		smo = append(smo, stack.WithHostedConfig(hc))
	}
	return smo, nil
}

func getHostPort(urlHost string) (host string, port string, err error) {
	u, err := url.Parse(urlHost)
	if err != nil {
		return "", "", err
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
