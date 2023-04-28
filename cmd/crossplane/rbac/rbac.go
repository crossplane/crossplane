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
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"

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

const pprofPath = "/debug/pprof/"

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
	LeaderElection      bool   `name:"leader-election" short:"l" help:"Use leader election for the conroller manager." env:"LEADER_ELECTION"`
	ManagementPolicy    string `name:"manage" short:"m" help:"RBAC management policy." default:"${rbac_manage_default_var}" enum:"${rbac_manage_enum_var}"`
	Registry            string `short:"r" help:"Default registry used to fetch packages when not specified in tag." default:"${default_registry}" env:"REGISTRY"`

	SyncInterval     time.Duration `short:"s" help:"How often all resources will be double-checked for drift from the desired state." default:"1h"`
	PollInterval     time.Duration `help:"How often individual resources will be checked for drift from the desired state." default:"1m"`
	MaxReconcileRate int           `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`
}

// Run the RBAC manager.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	log.Debug("Starting", "policy", c.ManagementPolicy)
	if c.Profile != "" {
		// NOTE(negz): These log messages attempt to match those emitted by
		// controller-runtime's metrics HTTP server when it starts.
		log.Debug("Profiling server is starting to listen", "addr", c.Profile)
		go func() {

			// Registering these explicitly ensures they're only served by the
			// HTTP server we start specifically for profiling.
			mux := http.NewServeMux()
			mux.HandleFunc(pprofPath, pprof.Index)
			mux.HandleFunc(filepath.Join(pprofPath, "cmdline"), pprof.Cmdline)
			mux.HandleFunc(filepath.Join(pprofPath, "profile"), pprof.Profile)
			mux.HandleFunc(filepath.Join(pprofPath, "symbol"), pprof.Symbol)
			mux.HandleFunc(filepath.Join(pprofPath, "trace"), pprof.Trace)

			s := &http.Server{
				Addr:         c.Profile,
				ReadTimeout:  2 * time.Minute,
				WriteTimeout: 2 * time.Minute,
				Handler:      mux,
			}
			log.Debug("Starting server", "type", "pprof", "path", pprofPath, "addr", s.Addr)
			err := s.ListenAndServe()
			log.Debug("Profiling server has stopped listening", "error", err)
		}()
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
		SyncPeriod:                 &c.SyncInterval,
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
		ManagementPolicy: rbaccontroller.ManagementPolicy(c.ManagementPolicy),
		DefaultRegistry:  c.Registry,
	}

	if err := rbac.Setup(mgr, o); err != nil {
		return errors.Wrap(err, "cannot add RBAC controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}
