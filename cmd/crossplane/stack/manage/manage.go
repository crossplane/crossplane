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

package manage

import (
	"time"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/pkg/controller/stacks"
	"github.com/crossplane/crossplane/pkg/controller/stacks/templates"
)

// Command configuration for the stack manager.
type Command struct {
	Name                      string
	Sync                      time.Duration
	RestrictCoreAPIGroups     bool
	EnableTemplateStacks      bool
	TemplatingControllerImage string
	HostControllerNamespace   string
	TenantKubeConfig          string
}

// FromKingpin produces the stack manager command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("restrict-core-apigroups", "Enable API group restrictions for Stacks. When enabled, APIs that Stacks depend on and own must contain a dot (\".\") and may not end with \"k8s.io\". When omitted, all groups are permitted.").Default("false").BoolVar(&c.RestrictCoreAPIGroups)
	cmd.Flag("templates", "Enable support for template stacks").BoolVar(&c.EnableTemplateStacks)
	cmd.Flag("templating-controller-image", "The image of the template stacks controller").StringVar(&c.TemplatingControllerImage)
	cmd.Flag("host-controller-namespace", "The namespace on Host Cluster where install and controller jobs/deployments will be created. Setting this will activate host aware mode of Stack Manager").StringVar(&c.HostControllerNamespace)
	cmd.Flag("tenant-kubeconfig", "The absolute path of the kubeconfig file to Tenant Kubernetes instance (required for host aware mode, ignored otherwise).").ExistingFileVar(&c.TenantKubeConfig)
	return c
}

// Run the stack manager.
func (c *Command) Run(log logging.Logger) error {
	log.Debug("Starting", "sync-period", c.Sync.String())

	if c.RestrictCoreAPIGroups {
		log.Debug("Restricting core group use in the Stacks")
	}

	cfg, err := getRestConfig(c.TenantKubeConfig)
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: &c.Sync})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add core Crossplane APIs to scheme")
	}

	if err := apiextensionsv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add API extensions to scheme")
	}

	if err := stacks.Setup(mgr, log, c.HostControllerNamespace, c.TemplatingControllerImage, c.RestrictCoreAPIGroups); err != nil {
		return errors.Wrap(err, "Cannot add stacks controllers to manager")
	}

	if c.EnableTemplateStacks {
		if c.TemplatingControllerImage == "" {
			return errors.New("--templating-controller-image is required with --templates")
		}

		if err := templates.SetupStackDefinitions(mgr, log); err != nil {
			return errors.Wrap(err, "Cannot add stack definition controller to manager")
		}
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}

func getRestConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		return ctrl.GetConfig()
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{}).ClientConfig()
}
