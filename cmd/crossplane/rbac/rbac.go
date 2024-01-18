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

// Package rbac implements Crossplane's RBAC controller manager.
package rbac

import (
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	"github.com/crossplane/crossplane/internal/controller/rbac"
	rbaccontroller "github.com/crossplane/crossplane/internal/controller/rbac/controller"
)

// Available RBAC management policies.
const (
	ManagementPolicyAll   = string(rbaccontroller.ManagementPolicyAll)
	ManagementPolicyBasic = string(rbaccontroller.ManagementPolicyBasic)
)

// KongVars represent the kong variables associated with the CLI parser
// required for the RBAC enum interpolation.
var KongVars = kong.Vars{
	"rbac_manage_default_var": ManagementPolicyBasic,
	"rbac_manage_enum_var": strings.Join(
		[]string{
			ManagementPolicyAll,
			ManagementPolicyBasic,
		},
		", "),
	"default_registry": name.DefaultRegistry,
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
	Profile string `placeholder:"host:port" help:"Serve runtime profiling data via HTTP at /debug/pprof."`

	ProviderClusterRole string `name:"provider-clusterrole" help:"A ClusterRole enumerating the permissions provider packages may request."`
	LeaderElection      bool   `name:"leader-election" short:"l" help:"Use leader election for the controller manager." env:"LEADER_ELECTION"`
	Registry            string `short:"r" help:"Default registry used to fetch packages when not specified in tag." default:"${default_registry}" env:"REGISTRY"`

	ManagementPolicy           string `name:"manage" short:"m" hidden:""`
	DeprecatedManagementPolicy string `name:"deprecated-manage" hidden:"" default:"${rbac_manage_default_var}" enum:"${rbac_manage_enum_var}"`

	SyncInterval     time.Duration `short:"s" help:"How often all resources will be double-checked for drift from the desired state." default:"1h"`
	PollInterval     time.Duration `help:"How often individual resources will be checked for drift from the desired state." default:"1m"`
	MaxReconcileRate int           `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`
}

// Run the RBAC manager.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	if c.ManagementPolicy != "" {
		return errors.New("--manage is deprecated, you can use --deprecated-manage until it is removed: see https://github.com/crossplane/crossplane/issues/5227")
	}

	log.Debug("Starting", "policy", c.DeprecatedManagementPolicy)

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, c.MaxReconcileRate), ctrl.Options{
		Scheme:                     s,
		LeaderElection:             c.LeaderElection,
		LeaderElectionID:           "crossplane-leader-election-rbac",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		Cache: cache.Options{
			SyncPeriod: &c.SyncInterval,
		},
		PprofBindAddress: c.Profile,
	})
	if err != nil {
		return errors.Wrap(err, "cannot create manager")
	}

	o := rbaccontroller.Options{
		Options: controller.Options{
			Logger:                  log,
			MaxConcurrentReconciles: c.MaxReconcileRate,
			PollInterval:            c.PollInterval,
			GlobalRateLimiter:       ratelimiter.NewGlobal(c.MaxReconcileRate),
		},
		AllowClusterRole: c.ProviderClusterRole,
		ManagementPolicy: rbaccontroller.ManagementPolicy(c.DeprecatedManagementPolicy),
		DefaultRegistry:  c.Registry,
	}

	if err := rbac.Setup(mgr, o); err != nil {
		return errors.Wrap(err, "cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}
