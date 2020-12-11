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

package rbac

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/internal/controller/rbac"
)

// Available RBAC management policies.
const (
	ManagementPolicyAll   = string(rbac.ManagementPolicyAll)
	ManagementPolicyBasic = string(rbac.ManagementPolicyBasic)
)

// Command configuration for the RBAC manager.
type Command struct {
	Name                string
	Sync                time.Duration
	LeaderElection      bool
	ManagementPolicy    string
	ProviderClusterRole string
}

// FromKingpin produces the RBAC manager command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("manage", "RBAC management policy.").Short('m').Default(ManagementPolicyAll).EnumVar(&c.ManagementPolicy, ManagementPolicyAll, ManagementPolicyBasic)
	cmd.Flag("provider-clusterrole", "A ClusterRole enumerating the permissions provider packages may request.").StringVar(&c.ProviderClusterRole)
	cmd.Flag("leader-election", "Use leader election for the conroller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").BoolVar(&c.LeaderElection)

	return c
}

// Run the RBAC manager.
func (c *Command) Run(log logging.Logger) error {
	log.Debug("Starting", "sync-period", c.Sync.String(), "policy", c.ManagementPolicy)

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: fmt.Sprintf("crossplane-leader-election-%s", c.Name),
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add core Crossplane APIs to scheme")
	}

	if err := extv1.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add Kubernetes API extensions to scheme")
	}

	if err := rbac.Setup(mgr, log, rbac.ManagementPolicy(c.ManagementPolicy), c.ProviderClusterRole); err != nil {
		return errors.Wrap(err, "Cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
