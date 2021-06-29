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
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/controller/rbac"
)

// Available RBAC management policies.
const (
	ManagementPolicyAll   = string(rbac.ManagementPolicyAll)
	ManagementPolicyBasic = string(rbac.ManagementPolicyBasic)
)

// KongVars represent the kong variables associated with the CLI parser
// required for the RBAC enum interpolation.
var KongVars = kong.Vars{
	"rbac_manage_default_var": ManagementPolicyAll,
	"rbac_manage_enum_var": strings.Join(
		[]string{
			ManagementPolicyAll,
			ManagementPolicyBasic,
		},
		", "),
}

// Command runs the crossplane RBAC controllers
type Command struct {
	Start startCommand `cmd:"" help:"Start Crossplane RBAC controllers."`
	Init  initCommand  `cmd:"" help:"Initialize RBAC Manager."`
}

// Run is the no-op method required for kong call tree.
// Kong requires each node in the calling path to have associated
// Run method.
func (c *Command) Run() error {
	return nil
}

type startCommand struct {
	ProviderClusterRole string        `name:"provider-clusterrole" help:"A ClusterRole enumerating the permissions provider packages may request."`
	LeaderElection      bool          `name:"leader-election" short:"l" help:"Use leader election for the conroller manager." env:"LEADER_ELECTION"`
	Sync                time.Duration `short:"s" help:"Controller manager sync period duration such as 300ms, 1.5h or 2h45m" default:"1h"`
	ManagementPolicy    string        `name:"manage" short:"m" help:"RBAC management policy." default:"${rbac_manage_default_var}" enum:"${rbac_manage_enum_var}"`
}

// Run the RBAC manager.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	log.Debug("Starting", "sync-period", c.Sync.String(), "policy", c.ManagementPolicy)
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           s,
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: "crossplane-leader-election-rbac",
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create manager")
	}

	if err := rbac.Setup(mgr, log, rbac.ManagementPolicy(c.ManagementPolicy), c.ProviderClusterRole); err != nil {
		return errors.Wrap(err, "cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}
