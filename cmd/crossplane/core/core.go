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

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Command runs the core crossplane controllers
type Command struct {
	Start startCommand `cmd:"" help:"Start Crossplane controllers."`
	Init  initCommand  `cmd:"" help:"Make cluster ready for Crossplane controllers."`
}

// KongVars represent the kong variables associated with the CLI parser
// required for the Registry default variable interpolation.
var KongVars = kong.Vars{
	"default_registry": name.DefaultRegistry,
}

// Run is the no-op method required for kong call tree
// Kong requires each node in the calling path to have associated
// Run method.
func (c *Command) Run() error {
	return nil
}

type startCommand struct {
	Namespace      string        `short:"n" help:"Namespace used to unpack and run packages." default:"crossplane-system" env:"POD_NAMESPACE"`
	CacheDir       string        `short:"c" help:"Directory used for caching package images." default:"/cache" env:"CACHE_DIR"`
	LeaderElection bool          `short:"l" help:"Use leader election for the controller manager." default:"false" env:"LEADER_ELECTION"`
	Registry       string        `short:"r" help:"Default registry used to fetch packages when not specified in tag." default:"${default_registry}" env:"REGISTRY"`
	Sync           time.Duration `short:"s" help:"Controller manager sync period duration such as 300ms, 1.5h or 2h45m" default:"1h"`
}

// Run core Crossplane controllers.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}
	log.Debug("Starting", "sync-period", c.Sync.String())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           s,
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: "crossplane-leader-election-core",
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	if err := apiextensions.Setup(mgr, log); err != nil {
		return errors.Wrap(err, "Cannot setup API extension controllers")
	}

	pkgCache := xpkg.NewImageCache(c.CacheDir, afero.NewOsFs())

	if err := pkg.Setup(mgr, log, pkgCache, c.Namespace, c.Registry); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
