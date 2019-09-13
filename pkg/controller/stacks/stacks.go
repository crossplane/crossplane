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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	install "github.com/crossplaneio/crossplane/pkg/controller/stacks/install"
	"github.com/crossplaneio/crossplane/pkg/controller/stacks/stack"
)

// Controllers passes down config and adds individual controllers to the manager.
type Controllers struct{}

// SetupWithManager adds all Stack controllers to the manager.
func (c *Controllers) SetupWithManager(mgr ctrl.Manager) error {
	creators := []func() (string, func() v1alpha1.StackInstaller){
		newStackInstall, newClusterStackInstall,
	}

	for _, creator := range creators {
		if err := (&install.Controller{
			StackInstallCreator: creator,
		}).SetupWithManager(mgr); err != nil {
			return err
		}
	}

	if err := (&stack.Controller{}).SetupWithManager(mgr); err != nil {
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
