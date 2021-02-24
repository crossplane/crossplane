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
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Command configuration for the core Crossplane controllers.
type Command struct {
	Name           string
	Namespace      string
	CacheDir       string
	LeaderElection bool
	Sync           time.Duration
	Providers      []string
	Configurations []string
}

// FromKingpin produces the core Crossplane command from a Kingpin command.
func FromKingpin(cmd *kingpin.CmdClause) *Command {
	c := &Command{Name: cmd.FullCommand()}
	cmd.Flag("namespace", "Namespace used to unpack and run packages.").Short('n').Default("crossplane-system").OverrideDefaultFromEnvar("POD_NAMESPACE").StringVar(&c.Namespace)
	cmd.Flag("cache-dir", "Directory used for caching package images.").Short('c').Default("/cache").OverrideDefaultFromEnvar("CACHE_DIR").ExistingDirVar(&c.CacheDir)
	cmd.Flag("sync", "Controller manager sync period duration such as 300ms, 1.5h or 2h45m").Short('s').Default("1h").DurationVar(&c.Sync)
	cmd.Flag("leader-election", "Use leader election for the conroller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").BoolVar(&c.LeaderElection)
	cmd.Flag("provider", "Pre-install a Provider by giving its image URI. This argument can be repeated.").StringsVar(&c.Providers)
	cmd.Flag("configuration", "Pre-install a Configuration by giving its image URI. This argument can be repeated.").StringsVar(&c.Configurations)
	return c
}

// Run core Crossplane controllers.
func (c *Command) Run(log logging.Logger) error {
	s := runtime.NewScheme()
	// Note that the controller managers scheme must be a superset of the
	// package manager's object scheme; it must contain all object types that
	// may appear in a Crossplane package. This is because the package manager
	// uses the controller manager's client (and thus scheme) to create packaged
	// objects.
	for _, f := range []func(scheme *runtime.Scheme) error{
		extv1.AddToScheme,
		extv1beta1.AddToScheme,
		apis.AddToScheme,
	} {
		if err := f(s); err != nil {
			return err
		}
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	cl, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		return errors.Wrap(err, "cannot create new kubernetes client")
	}
	i := initializer.New(cl,
		initializer.NewCoreCRDs("/crds"),
		initializer.NewLockObject(),
		initializer.NewPackageInstaller(c.Providers, c.Configurations),
	)
	if err := i.Init(context.TODO()); err != nil {
		return errors.Wrap(err, "cannot initialize core")
	}
	log.Debug("Initialization has been completed")
	log.Debug("Starting", "sync-period", c.Sync.String())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           s,
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: fmt.Sprintf("crossplane-leader-election-%s", c.Name),
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	if err := apiextensions.Setup(mgr, log); err != nil {
		return errors.Wrap(err, "Cannot setup API extension controllers")
	}

	pkgCache := xpkg.NewImageCache(c.CacheDir, afero.NewOsFs())

	if err := pkg.Setup(mgr, log, pkgCache, c.Namespace); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
