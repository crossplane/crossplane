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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"

	"github.com/crossplane/crossplane/v2/internal/controller/rbac"
	rbaccontroller "github.com/crossplane/crossplane/v2/internal/controller/rbac/controller"
)

// Command runs the crossplane RBAC controllers.
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
	Profile string `help:"Serve runtime profiling data via HTTP at /debug/pprof." placeholder:"host:port"`

	ProviderClusterRole string `help:"A ClusterRole enumerating the permissions provider packages may request." name:"provider-clusterrole"`
	LeaderElection      bool   `env:"LEADER_ELECTION"                                                           help:"Use leader election for the controller manager." name:"leader-election" short:"l"`

	SyncInterval     time.Duration `default:"1h" help:"How often all resources will be double-checked for drift from the desired state."                    short:"s"`
	PollInterval     time.Duration `default:"1m" help:"How often individual resources will be checked for drift from the desired state."`
	MaxReconcileRate int           `default:"10" help:"The global maximum rate per second at which resources may checked for drift from the desired state."`

	// These are features that we've removed support for. Crossplane returns an
	// error when you enable them. This ensures you'll see an explicit and
	// informative error on startup, instead of a potentially surprising one
	// later.
	Registry string `hidden:""`
}

// Run the RBAC manager.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	if c.Registry != "" {
		return errors.New("the --registry flag is no longer supported since support for a default registry value has been removed. Please ensure that all packages have fully qualified names that explicitly state their registry. This also applies to all of a packages dependencies")
	}

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
	}

	if err := rbac.Setup(mgr, o); err != nil {
		return errors.Wrap(err, "cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}
