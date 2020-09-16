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

package core

import (
	"time"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	crds "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"

	oamapis "github.com/crossplane/oam-kubernetes-runtime/apis/core"
	"github.com/crossplane/oam-kubernetes-runtime/pkg/controller/oam"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/pkg/controller/apiextensions"
	"github.com/crossplane/crossplane/pkg/controller/pkg"
	"github.com/crossplane/crossplane/pkg/controller/workload"
)

// Command configuration for the core Crossplane controllers.
type Command struct {
	Name             string
	Namespace        string
	TenantKubeConfig string
	Sync             time.Duration
}

// FromKingpin produces the core Crossplane command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("package-namespace", "Namespace used to unpack and run packages.").Short('n').Default("crossplane-system").OverrideDefaultFromEnvar("PACKAGE_NAMESPACE").StringVar(&c.Namespace)
	cmd.Flag("tenant-kubeconfig", "The absolute path of the kubeconfig file to Tenant Kubernetes instance (required for host aware mode, ignored otherwise).").ExistingFileVar(&c.TenantKubeConfig)
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	return c
}

// Run core Crossplane controllers.
func (c *Command) Run(log logging.Logger) error { // nolint:gocyclo
	log.Debug("Starting", "sync-period", c.Sync.String())

	// If running in hosted mode, we use the mounted kubeconfig to make manager
	// run against tenant.
	cfg, err := getRestConfig(c.TenantKubeConfig)
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	// If running in hosted mode, we use the in-cluster config to run any
	// workloads. If not in hosted mode, this config will match the one above.
	hostCfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "failed to initialize host config")
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{SyncPeriod: &c.Sync})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	if err := crds.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add CustomResourceDefinition API to scheme")
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add core Crossplane APIs to scheme")
	}

	if err := oamapis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add core OAM APIs to scheme")
	}

	if err := oam.Setup(mgr, log); err != nil {
		return errors.Wrap(err, "Cannot setup OAM controllers")
	}

	if err := workload.Setup(mgr, log); err != nil {
		return errors.Wrap(err, "Cannot setup workload controllers")
	}

	if err := apiextensions.Setup(mgr, log); err != nil {
		return errors.Wrap(err, "Cannot setup API extension controllers")
	}

	if err := pkg.Setup(mgr, hostCfg, log, c.Namespace); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
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
