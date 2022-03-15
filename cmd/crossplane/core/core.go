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
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	pkgcontroller "github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/features"
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
	Namespace            string `short:"n" help:"Namespace used to unpack and run packages." default:"crossplane-system" env:"POD_NAMESPACE"`
	CacheDir             string `short:"c" help:"Directory used for caching package images." default:"/cache" env:"CACHE_DIR"`
	LeaderElection       bool   `short:"l" help:"Use leader election for the controller manager." default:"false" env:"LEADER_ELECTION"`
	Registry             string `short:"r" help:"Default registry used to fetch packages when not specified in tag." default:"${default_registry}" env:"REGISTRY"`
	CABundlePath         string `help:"Additional CA bundle to use when fetching packages from registry." env:"CA_BUNDLE_PATH"`
	WebhookTLSSecretName string `help:"The name of the TLS Secret that will be used by the webhook servers of core Crossplane and providers." env:"WEBHOOK_TLS_SECRET_NAME"`
	WebhookTLSCertDir    string `help:"The directory of TLS certificate that will be used by the webhook server of core Crossplane. There should be tls.crt and tls.key files." env:"WEBHOOK_TLS_CERT_DIR"`

	SyncInterval     time.Duration `short:"s" help:"How often all resources will be double-checked for drift from the desired state." default:"1h"`
	PollInterval     time.Duration `help:"How often individual resources will be checked for drift from the desired state." default:"1m"`
	MaxReconcileRate int           `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`

	EnableCompositionRevisions bool `group:"Alpha Features:" help:"Enable support for CompositionRevisions."`
	EnableExternalSecretStores bool `group:"Alpha Features:" help:"Enable support for ExternalSecretStores."`
}

// Run core Crossplane controllers.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error { //nolint:gocyclo
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "Cannot get config")
	}

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, c.MaxReconcileRate), ctrl.Options{
		Scheme:     s,
		SyncPeriod: &c.SyncInterval,

		// controller-runtime uses both ConfigMaps and Leases for leader
		// election by default. Leases expire after 15 seconds, with a
		// 10 second renewal deadline. We've observed leader loss due to
		// renewal deadlines being exceeded when under high load - i.e.
		// hundreds of reconciles per second and ~200rps to the API
		// server. Switching to Leases only and longer leases appears to
		// alleviate this.
		LeaderElection:             c.LeaderElection,
		LeaderElectionID:           "crossplane-leader-election-core",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
	})
	if err != nil {
		return errors.Wrap(err, "Cannot create manager")
	}

	feats := &feature.Flags{}
	if c.EnableCompositionRevisions {
		feats.Enable(features.EnableAlphaCompositionRevisions)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaCompositionRevisions)
	}
	if c.EnableExternalSecretStores {
		feats.Enable(features.EnableAlphaExternalSecretStores)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaExternalSecretStores)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: c.MaxReconcileRate,
		PollInterval:            c.PollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(c.MaxReconcileRate),
		Features:                feats,
	}

	if err := apiextensions.Setup(mgr, o); err != nil {
		return errors.Wrap(err, "Cannot setup API extension controllers")
	}

	po := pkgcontroller.Options{
		Options:              o,
		Cache:                xpkg.NewFsPackageCache(c.CacheDir, afero.NewOsFs()),
		Namespace:            c.Namespace,
		DefaultRegistry:      c.Registry,
		Features:             feats,
		WebhookTLSSecretName: c.WebhookTLSSecretName,
	}

	if c.CABundlePath != "" {
		rootCAs, err := xpkg.ParseCertificatesFromPath(c.CABundlePath)
		if err != nil {
			return errors.Wrap(err, "Cannot parse CA bundle")
		}
		po.FetcherOptions = []xpkg.FetcherOpt{xpkg.WithCustomCA(rootCAs)}
	}

	if err := pkg.Setup(mgr, po); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
	}

	if c.WebhookTLSCertDir != "" {
		ws := mgr.GetWebhookServer()
		ws.Port = 9443
		ws.CertDir = c.WebhookTLSCertDir
		ws.TLSMinVersion = "1.3"
		// TODO(muvaf): Once the implementation of other webhook handlers are
		// fleshed out, implement a registration pattern similar to scheme
		// registrations.
		if err := (&apiextensionsv1.CompositeResourceDefinition{}).SetupWebhookWithManager(mgr); err != nil {
			return errors.Wrap(err, "cannot setup webhook for compositeresourcedefinitions")
		}
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
