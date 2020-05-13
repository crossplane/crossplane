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
	"github.com/crossplane/crossplane/pkg/controller/packages"
	"github.com/crossplane/crossplane/pkg/controller/packages/templates"
)

// Command configuration for the package manager.
type Command struct {
	Name                      string
	Sync                      time.Duration
	AllowAllAPIGroups         bool
	PassFullDeployment        bool
	EnableTemplateStacks      bool
	TemplatingControllerImage string
	HostControllerNamespace   string
	TenantKubeConfig          string
	ForceImagePullPolicy      string
}

// FromKingpin produces the package manager command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("insecure-allow-all-apigroups", "Enable core Kubernetes API group permissions for Packages. When enabled, Packages may declare dependency on core Kubernetes API types. When omitted, APIs that Packages depend on and own must contain a dot (\".\") and may not end with \"k8s.io\".").Default("false").BoolVar(&c.AllowAllAPIGroups)
	cmd.Flag("insecure-pass-full-deployment", "Enable packagess to pass their full deployment, including security context. When omitted, Packages deployments will have security context removed and all containers will have allowPrivilegeEscalation set to false.").Default("false").BoolVar(&c.PassFullDeployment)
	cmd.Flag("templates", "Enable support for template stacks").BoolVar(&c.EnableTemplateStacks)
	cmd.Flag("templating-controller-image", "The image of the template stacks controller").StringVar(&c.TemplatingControllerImage)
	cmd.Flag("host-controller-namespace", "The namespace on Host Cluster where install and controller jobs/deployments will be created. Setting this will activate host aware mode of Package Manager").StringVar(&c.HostControllerNamespace)
	cmd.Flag("tenant-kubeconfig", "The absolute path of the kubeconfig file to Tenant Kubernetes instance (required for host aware mode, ignored otherwise).").ExistingFileVar(&c.TenantKubeConfig)
	cmd.Flag("force-image-pull-policy", "All containers created by the PackageManager in service of PackageInstall and Package resources will use the specified imagePullPolicy").StringVar(&c.ForceImagePullPolicy)
	return c
}

// Run the package manager.
// nolint:gocyclo
func (c *Command) Run(log logging.Logger) error {
	log.Debug("Starting", "sync-period", c.Sync.String())

	if c.AllowAllAPIGroups {
		log.Debug("Allowing core group use in Packages")
	}

	if c.PassFullDeployment {
		log.Debug("Allowing Packages to pass full deployment manifests")
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

	if err := packages.Setup(mgr, log, c.HostControllerNamespace, c.TemplatingControllerImage, c.AllowAllAPIGroups, c.PassFullDeployment, c.ForceImagePullPolicy); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
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
