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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
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

// TODO: Fix Enum Support and Remove Hardcoding

// Cmd starts Crossplane RBAC Manager controllers.
type Cmd struct {
	Sync                time.Duration `short:"s" default:"1h" help:"Controller manager sync period duration such as 300ms, 1.5h or 2h45m."`
	LeaderElection      bool          `short:"l" default:"false" env:"LEADER_ELECTION" help:"Use leader election for the conroller manager."`
	ManagementPolicy    string        `short:"m" name:"manage" default:"${rbac_manage_default_var}" enum:"${rbac_manage_enum_var}" help:"RBAC management policy."`
	ProviderClusterRole string        `help:"A ClusterRole enumerating the permissions provider packages may request."`
}

// Run the RBAC manager.
func (c *Cmd) Run(zl *logr.Logger) error {
	log := logging.NewLogrLogger((*zl).WithName("rbac"))
	log.Debug("Starting", "sync-period", c.Sync.String(), "policy", c.ManagementPolicy)

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: "crossplane-leader-election-rbac",
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
