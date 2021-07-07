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

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Command configuration for the core Crossplane controllers.
type Command struct {
	Name           string
	Namespace      string
	Registry       string
	CacheDir       string
	LeaderElection bool
	Sync           time.Duration
}

// FromKingpin produces the core Crossplane command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) (*Command, *InitCommand) {
	startCmd := cmd.Command("start", "Start Crossplane controllers.")
	c := &Command{Name: startCmd.FullCommand()}
	cmd.Flag("namespace", "Namespace used to unpack and run packages.").Short('n').Default("crossplane-system").OverrideDefaultFromEnvar("POD_NAMESPACE").StringVar(&c.Namespace)
	cmd.Flag("registry", "Default registry used to fetch packages when not specified in tag.").Short('r').Default(name.DefaultRegistry).Envar("REGISTRY").StringVar(&c.Registry)
	cmd.Flag("cache-dir", "Directory used for caching package images.").Short('c').Default("/cache").OverrideDefaultFromEnvar("CACHE_DIR").StringVar(&c.CacheDir)
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("leader-election", "Use leader election for the conroller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").BoolVar(&c.LeaderElection)
	initCmd := cmd.Command("init", "Make cluster ready for Crossplane controllers.")
	init := &InitCommand{Name: initCmd.FullCommand()}
	initCmd.Flag("provider", "Pre-install a Provider by giving its image URI. This argument can be repeated.").StringsVar(&init.Providers)
	initCmd.Flag("configuration", "Pre-install a Configuration by giving its image URI. This argument can be repeated.").StringsVar(&init.Configurations)
	return c, init
}

// Run core Crossplane controllers.
func (c *Command) Run(s *runtime.Scheme, log logging.Logger) error {
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
