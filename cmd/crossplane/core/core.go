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

// Package core implements Crossplane's core controller manager.
package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/crossplane/crossplane-runtime/pkg/certificates"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"

	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	pkgcontroller "github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/features"
	"github.com/crossplane/crossplane/internal/initializer"
	"github.com/crossplane/crossplane/internal/metrics"
	"github.com/crossplane/crossplane/internal/transport"
	"github.com/crossplane/crossplane/internal/usage"
	"github.com/crossplane/crossplane/internal/validation/apiextensions/v1/composition"
	"github.com/crossplane/crossplane/internal/validation/apiextensions/v1/xrd"
	"github.com/crossplane/crossplane/internal/xfn"
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
	"default_registry":   xpkg.DefaultRegistry,
	"default_user_agent": transport.DefaultUserAgent,
}

// Run is the no-op method required for kong call tree
// Kong requires each node in the calling path to have associated
// Run method.
func (c *Command) Run() error {
	return nil
}

type startCommand struct {
	Profile string `placeholder:"host:port" help:"Serve runtime profiling data via HTTP at /debug/pprof."`

	Namespace      string `short:"n" help:"Namespace used to unpack and run packages." default:"crossplane-system" env:"POD_NAMESPACE"`
	ServiceAccount string `help:"Name of the Crossplane Service Account." default:"crossplane" env:"POD_SERVICE_ACCOUNT"`
	CacheDir       string `short:"c" help:"Directory used for caching package images." default:"/cache" env:"CACHE_DIR"`
	LeaderElection bool   `short:"l" help:"Use leader election for the controller manager." default:"false" env:"LEADER_ELECTION"`
	Registry       string `short:"r" help:"Default registry used to fetch packages when not specified in tag." default:"${default_registry}" env:"REGISTRY"`
	CABundlePath   string `help:"Additional CA bundle to use when fetching packages from registry." env:"CA_BUNDLE_PATH"`
	UserAgent      string `help:"The User-Agent header that will be set on all package requests." default:"${default_user_agent}" env:"USER_AGENT"`

	PackageRuntime string `helm:"The package runtime to use for packages with a runtime (e.g. Providers and Functions)" default:"Deployment" env:"PACKAGE_RUNTIME"`

	SyncInterval     time.Duration `short:"s" help:"How often all resources will be double-checked for drift from the desired state." default:"1h"`
	PollInterval     time.Duration `help:"How often individual resources will be checked for drift from the desired state." default:"1m"`
	MaxReconcileRate int           `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`

	WebhookEnabled bool `help:"Enable webhook configuration." default:"true" env:"WEBHOOK_ENABLED"`

	TLSServerSecretName string `help:"The name of the TLS Secret that will store Crossplane's server certificate." env:"TLS_SERVER_SECRET_NAME"`
	TLSServerCertsDir   string `help:"The path of the folder which will store TLS server certificate of Crossplane." env:"TLS_SERVER_CERTS_DIR"`
	TLSClientSecretName string `help:"The name of the TLS Secret that will be store Crossplane's client certificate." env:"TLS_CLIENT_SECRET_NAME"`
	TLSClientCertsDir   string `help:"The path of the folder which will store TLS client certificate of Crossplane." env:"TLS_CLIENT_CERTS_DIR"`

	EnableEnvironmentConfigs   bool `group:"Alpha Features:" help:"Enable support for EnvironmentConfigs."`
	EnableExternalSecretStores bool `group:"Alpha Features:" help:"Enable support for External Secret Stores."`
	EnableUsages               bool `group:"Alpha Features:" help:"Enable support for deletion ordering and resource protection with Usages."`
	EnableRealtimeCompositions bool `group:"Alpha Features:" help:"Enable support for realtime compositions, i.e. watching composed resources and reconciling compositions immediately when any of the composed resources is updated."`

	EnableCompositionFunctions               bool `group:"Beta Features:" default:"true" help:"Enable support for Composition Functions."`
	EnableCompositionFunctionsExtraResources bool `group:"Beta Features:" default:"true" help:"Enable support for Composition Functions Extra Resources. Only respected if --enable-composition-functions is set to true."`
	EnableCompositionWebhookSchemaValidation bool `group:"Beta Features:" default:"true" help:"Enable support for Composition validation using schemas."`
	EnableDeploymentRuntimeConfigs           bool `group:"Beta Features:" default:"true" help:"Enable support for Deployment Runtime Configs."`

	// These are GA features that previously had alpha or beta feature flags.
	// You can't turn off a GA feature. We maintain the flags to avoid breaking
	// folks who are passing them, but they do nothing. The flags are hidden so
	// they don't show up in the help output.
	EnableCompositionRevisions bool `default:"true" hidden:""`
}

// Run core Crossplane controllers.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error { //nolint:gocyclo // Only slightly over.
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	cfg.WarningHandler = rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
		// Warnings from API requests should be deduplicated so they are only logged once
		Deduplicate: true,
	})

	eb := record.NewBroadcaster()
	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, c.MaxReconcileRate), ctrl.Options{
		Scheme: s,
		Cache: cache.Options{
			SyncPeriod: &c.SyncInterval,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: c.TLSServerCertsDir,
			TLSOpts: []func(*tls.Config){
				func(t *tls.Config) {
					t.MinVersion = tls.VersionTLS13
				},
			},
		}),
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor:   []client.Object{&corev1.Secret{}},
				Unstructured: false, // this is the default to not cache unstructured objects
			},
		},
		EventBroadcaster: eb,

		// controller-runtime uses both ConfigMaps and Leases for leader
		// election by default. Leases expire after 15 seconds, with a
		// 10 second renewal deadline. We've observed leader loss due to
		// renewal deadlines being exceeded when under high load - i.e.
		// hundreds of reconciles per second and ~200rps to the API
		// server. Switching to Leases only and longer leases appears to
		// alleviate this.
		LeaderElection:                c.LeaderElection,
		LeaderElectionID:              "crossplane-leader-election-core",
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:                 func() *time.Duration { d := 50 * time.Second; return &d }(),

		PprofBindAddress:       c.Profile,
		HealthProbeBindAddress: ":8081",
	})
	if err != nil {
		return errors.Wrap(err, "cannot create manager")
	}

	eb.StartLogging(func(format string, args ...interface{}) {
		log.Debug(fmt.Sprintf(format, args...))
	})
	defer eb.Shutdown()

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: c.MaxReconcileRate,
		PollInterval:            c.PollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(c.MaxReconcileRate),
		Features:                &feature.Flags{},
	}

	if !c.EnableCompositionRevisions {
		log.Info("CompositionRevisions feature is GA and cannot be disabled. The --enable-composition-revisions flag will be removed in a future release.")
	}

	var functionRunner *xfn.PackagedFunctionRunner
	if c.EnableCompositionFunctions {
		o.Features.Enable(features.EnableBetaCompositionFunctions)
		log.Info("Beta feature enabled", "flag", features.EnableBetaCompositionFunctions)

		if c.EnableCompositionFunctionsExtraResources {
			o.Features.Enable(features.EnableBetaCompositionFunctionsExtraResources)
			log.Info("Beta feature enabled", "flag", features.EnableBetaCompositionFunctionsExtraResources)
		}

		clienttls, err := certificates.LoadMTLSConfig(
			filepath.Join(c.TLSClientCertsDir, initializer.SecretKeyCACert),
			filepath.Join(c.TLSClientCertsDir, corev1.TLSCertKey),
			filepath.Join(c.TLSClientCertsDir, corev1.TLSPrivateKeyKey),
			false)
		if err != nil {
			return errors.Wrap(err, "cannot load client TLS certificates")
		}

		m := xfn.NewMetrics()
		metrics.Registry.MustRegister(m)

		// We want all XR controllers to share the same gRPC clients.
		functionRunner = xfn.NewPackagedFunctionRunner(mgr.GetClient(),
			xfn.WithLogger(log),
			xfn.WithTLSConfig(clienttls),
			xfn.WithInterceptorCreators(m),
		)

		// Periodically remove clients for Functions that no longer exist.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go functionRunner.GarbageCollectConnections(ctx, 10*time.Minute)
	}
	if c.EnableEnvironmentConfigs {
		o.Features.Enable(features.EnableAlphaEnvironmentConfigs)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaEnvironmentConfigs)
	}
	if c.EnableCompositionWebhookSchemaValidation {
		o.Features.Enable(features.EnableBetaCompositionWebhookSchemaValidation)
		log.Info("Beta feature enabled", "flag", features.EnableBetaCompositionWebhookSchemaValidation)
	}
	if c.EnableUsages {
		o.Features.Enable(features.EnableAlphaUsages)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaUsages)
	}
	if c.EnableExternalSecretStores {
		o.Features.Enable(features.EnableAlphaExternalSecretStores)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaExternalSecretStores)

		tcfg, err := certificates.LoadMTLSConfig(
			filepath.Join(c.TLSClientCertsDir, initializer.SecretKeyCACert),
			filepath.Join(c.TLSClientCertsDir, corev1.TLSCertKey),
			filepath.Join(c.TLSClientCertsDir, corev1.TLSPrivateKeyKey),
			false)
		if err != nil {
			return errors.Wrap(err, "cannot load TLS certificates for external secret stores")
		}

		o.ESSOptions = &controller.ESSOptions{
			TLSConfig: tcfg,
		}
	}
	if c.EnableRealtimeCompositions {
		o.Features.Enable(features.EnableRealtimeCompositions)
		log.Info("Alpha feature enabled", "flag", features.EnableRealtimeCompositions)
	}
	if c.EnableDeploymentRuntimeConfigs {
		o.Features.Enable(features.EnableBetaDeploymentRuntimeConfigs)
		log.Info("Beta feature enabled", "flag", features.EnableBetaDeploymentRuntimeConfigs)
	}

	ao := apiextensionscontroller.Options{
		Options:        o,
		Namespace:      c.Namespace,
		ServiceAccount: c.ServiceAccount,
		FunctionRunner: functionRunner,
	}

	if err := apiextensions.Setup(mgr, ao); err != nil {
		return errors.Wrap(err, "cannot setup API extension controllers")
	}

	var pr pkgcontroller.PackageRuntime
	switch c.PackageRuntime {
	case string(pkgcontroller.PackageRuntimeDeployment):
		pr = pkgcontroller.PackageRuntimeDeployment
	case string(pkgcontroller.PackageRuntimeExternal):
		pr = pkgcontroller.PackageRuntimeExternal
	default:
		return errors.Errorf("unsupported package runtime %q, supported runtimes are %q and %q",
			c.PackageRuntime, pkgcontroller.PackageRuntimeDeployment, pkgcontroller.PackageRuntimeExternal)
	}

	po := pkgcontroller.Options{
		Options:         o,
		Cache:           xpkg.NewFsPackageCache(c.CacheDir, afero.NewOsFs()),
		Namespace:       c.Namespace,
		ServiceAccount:  c.ServiceAccount,
		DefaultRegistry: c.Registry,
		FetcherOptions:  []xpkg.FetcherOpt{xpkg.WithUserAgent(c.UserAgent)},
		PackageRuntime:  pr,
	}

	if c.CABundlePath != "" {
		rootCAs, err := ParseCertificatesFromPath(c.CABundlePath)
		if err != nil {
			return errors.Wrap(err, "cannot parse CA bundle")
		}
		po.FetcherOptions = append(po.FetcherOptions, xpkg.WithCustomCA(rootCAs))
	}

	if err := pkg.Setup(mgr, po); err != nil {
		return errors.Wrap(err, "cannot add packages controllers to manager")
	}

	// Registering webhooks with the manager is what actually starts the webhook
	// server.
	if c.WebhookEnabled {
		// TODO(muvaf): Once the implementation of other webhook handlers are
		// fleshed out, implement a registration pattern similar to scheme
		// registrations.
		if err := xrd.SetupWebhookWithManager(mgr, o); err != nil {
			return errors.Wrap(err, "cannot setup webhook for compositeresourcedefinitions")
		}
		if err := composition.SetupWebhookWithManager(mgr, o); err != nil {
			return errors.Wrap(err, "cannot setup webhook for compositions")
		}
		if o.Features.Enabled(features.EnableAlphaUsages) {
			if err := usage.SetupWebhookWithManager(mgr, o); err != nil {
				return errors.Wrap(err, "cannot setup webhook for usages")
			}
		}
	}

	if err := c.SetupProbes(mgr); err != nil {
		return errors.Wrap(err, "cannot setup probes")
	}

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "cannot start controller manager")
}

// SetupProbes sets up the health and readiness probes.
func (c *startCommand) SetupProbes(mgr ctrl.Manager) error {
	// Add default readiness probe
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		return errors.Wrap(err, "cannot create ping ready check")
	}

	// Add default health probe
	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return errors.Wrap(err, "cannot create ping health check")
	}

	// Add probes waiting for the webhook server if webhooks are enabled
	if c.WebhookEnabled {
		hookServer := mgr.GetWebhookServer()
		if err := mgr.AddReadyzCheck("webhook", hookServer.StartedChecker()); err != nil {
			return errors.Wrap(err, "cannot create webhook ready check")
		}
		if err := mgr.AddHealthzCheck("webhook", hookServer.StartedChecker()); err != nil {
			return errors.Wrap(err, "cannot create webhook health check")
		}
	}
	return nil
}
