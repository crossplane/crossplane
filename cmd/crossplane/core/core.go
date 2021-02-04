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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/apis"
	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// Cmd starts core Crossplane controllers.
type Cmd struct {
	Namespace      string        `short:"n" default:"crossplane-system" env:"POD_NAMESPACE" help:"Namespace used to unpack and run packages."`
	CacheDir       string        `short:"c" default:"/cache" env:"CACHE_DIR" type:"existingdir" help:"Directory used for caching package images."`
	Sync           time.Duration `short:"s" default:"1h" help:"Controller manager sync period duration such as 300ms, 1.5h or 2h45m."`
	LeaderElection bool          `short:"l" default:"false" env:"LEADER_ELECTION" help:"Use leader election for the conroller manager."`
}

// Run core Crossplane controllers.
func (c *Cmd) Run(zl *logr.Logger) error {
	log := logging.NewLogrLogger((*zl).WithName("crossplane"))
	log.Debug("Starting", "sync-period", c.Sync.String())

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   c.LeaderElection,
		LeaderElectionID: "crossplane-leader-election-core",
		SyncPeriod:       &c.Sync,
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	// Note that the controller managers scheme must be a superset of the
	// package manager's object scheme; it must contain all object types that
	// may appear in a Crossplane package. This is because the package manager
	// uses the controller manager's client (and thus scheme) to create packaged
	// objects.

	if err := extv1.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add CustomResourceDefinition v1 API to scheme")
	}

	if err := extv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add CustomResourceDefinition v1beta1 API to scheme")
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		return errors.Wrap(err, "Cannot add core Crossplane APIs to scheme")
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
