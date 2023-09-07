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
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
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
	"github.com/crossplane/crossplane/internal/transport"
	"github.com/crossplane/crossplane/internal/usage"
	"github.com/crossplane/crossplane/internal/validation/apiextensions/v1/composition"
	"github.com/crossplane/crossplane/internal/validation/apiextensions/v1/xrd"
	"github.com/crossplane/crossplane/internal/xfn"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const pprofPath = "/debug/pprof/"

// Command runs the core crossplane controllers
type Command struct {
	Start startCommand `cmd:"" help:"Start Crossplane controllers."`
	Init  initCommand  `cmd:"" help:"Make cluster ready for Crossplane controllers."`
}

// KongVars represent the kong variables associated with the CLI parser
// required for the Registry default variable interpolation.
var KongVars = kong.Vars{
	"default_registry":   name.DefaultRegistry,
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

	SyncInterval     time.Duration `short:"s" help:"How often all resources will be double-checked for drift from the desired state." default:"1h"`
	PollInterval     time.Duration `help:"How often individual resources will be checked for drift from the desired state." default:"1m"`
	MaxReconcileRate int           `help:"The global maximum rate per second at which resources may checked for drift from the desired state." default:"10"`

	ESSTLSSecretName     string `help:"The name of the TLS Secret that will be used by Crossplane and providers as clients of External Secret Store plugins." env:"ESS_TLS_SECRET_NAME"`
	ESSTLSCertsDir       string `help:"The path of the folder which will store TLS certificates to be used by Crossplane and providers for communicating with External Secret Store plugins." env:"ESS_TLS_CERTS_DIR"`
	WebhookTLSSecretName string `help:"The name of the TLS Secret that will be used by the webhook servers of core Crossplane and providers." env:"WEBHOOK_TLS_SECRET_NAME"`
	WebhookTLSCertDir    string `help:"The directory of TLS certificate that will be used by the webhook server of core Crossplane. There should be tls.crt and tls.key files." env:"WEBHOOK_TLS_CERT_DIR"`
	TLSServerSecretName  string `help:"The name of the TLS Secret that will store Crossplane's server certificate." env:"TLS_SERVER_SECRET_NAME"`
	TLSServerCertsDir    string `help:"The path of the folder which will store TLS server certificate of Crossplane." env:"TLS_SERVER_CERTS_DIR"`
	TLSClientSecretName  string `help:"The name of the TLS Secret that will be store Crossplane's client certificate." env:"TLS_CLIENT_SECRET_NAME"`
	TLSClientCertsDir    string `help:"The path of the folder which will store TLS client certificate of Crossplane." env:"TLS_CLIENT_CERTS_DIR"`

	EnableEnvironmentConfigs                 bool `group:"Alpha Features:" help:"Enable support for EnvironmentConfigs."`
	EnableExternalSecretStores               bool `group:"Alpha Features:" help:"Enable support for External Secret Stores."`
	EnableCompositionWebhookSchemaValidation bool `group:"Alpha Features:" help:"Enable support for Composition validation using schemas."`
	EnableUsages                             bool `group:"Alpha Features:" help:"Enable support for deletion ordering and resource protection with Usages."`

	EnableCompositionFunctions bool `group:"Beta Features:" default:"true" help:"Enable support for Composition Functions."`

	// These are GA features that previously had alpha or beta feature flags.
	// You can't turn off a GA feature. We maintain the flags to avoid breaking
	// folks who are passing them, but they do nothing. The flags are hidden so
	// they don't show up in the help output.
	EnableCompositionRevisions bool `default:"true" hidden:""`
}

// Run core Crossplane controllers.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error { //nolint:gocyclo // Only slightly over.
	if c.Profile != "" {
		// NOTE(negz): These log messages attempt to match those emitted by
		// controller-runtime's metrics HTTP server when it starts.
		log.Debug("Profiling server is starting to listen", "addr", c.Profile)
		go func() {

			// Registering these explicitly ensures they're only served by the
			// HTTP server we start explicitly for profiling.
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
		return errors.Wrap(err, "Cannot get config")
	}

	cfg.WarningHandler = rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
		// Warnings from API requests should be deduplicated so they are only logged once
		Deduplicate: true,
	})

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, c.MaxReconcileRate), ctrl.Options{
		Scheme: s,
		Cache: cache.Options{
			SyncPeriod: &c.SyncInterval,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: c.WebhookTLSCertDir,
			TLSOpts: []func(*tls.Config){
				func(t *tls.Config) {
					t.MinVersion = tls.VersionTLS13
				},
			},
		}),

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

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: c.MaxReconcileRate,
		PollInterval:            c.PollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(c.MaxReconcileRate),
		Features:                &feature.Flags{},
	}

	ao := apiextensionscontroller.Options{
		Options:        o,
		Namespace:      c.Namespace,
		ServiceAccount: c.ServiceAccount,
		Registry:       c.Registry,
	}

	clienttls, err := certificates.LoadMTLSConfig(
		filepath.Join(c.TLSClientCertsDir, initializer.SecretKeyCACert),
		filepath.Join(c.TLSClientCertsDir, corev1.TLSCertKey),
		filepath.Join(c.TLSClientCertsDir, corev1.TLSPrivateKeyKey),
		false)
	if err != nil {
		return errors.Wrap(err, "cannot load client TLS certificates ")
	}

	if !c.EnableCompositionRevisions {
		log.Info("CompositionRevisions feature is GA and cannot be disabled. The --enable-composition-revisions flag will be removed in a future release.")
	}
	if c.EnableCompositionFunctions {
		o.Features.Enable(features.EnableBetaCompositionFunctions)
		log.Info("Beta feature enabled", "flag", features.EnableBetaCompositionFunctions)

		// We want all XR controllers to share the same gRPC clients.
		runner := xfn.NewPackagedFunctionRunner(mgr.GetClient(),
			xfn.WithLogger(log),
			xfn.WithTLSConfig(clienttls))

		// Periodically remove clients for Functions that no longer exist.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go runner.GarbageCollectConnections(ctx, 10*time.Minute)

		ao.FunctionRunner = runner
	}
	if c.EnableEnvironmentConfigs {
		o.Features.Enable(features.EnableAlphaEnvironmentConfigs)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaEnvironmentConfigs)
	}
	if c.EnableCompositionWebhookSchemaValidation {
		o.Features.Enable(features.EnableAlphaCompositionWebhookSchemaValidation)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaCompositionWebhookSchemaValidation)
	}
	if c.EnableUsages {
		o.Features.Enable(features.EnableAlphaUsages)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaUsages)
	}
	if c.EnableExternalSecretStores {
		o.Features.Enable(features.EnableAlphaExternalSecretStores)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaExternalSecretStores)

		tcfg, err := certificates.LoadMTLSConfig(
			filepath.Join(c.ESSTLSCertsDir, initializer.SecretKeyCACert),
			filepath.Join(c.ESSTLSCertsDir, corev1.TLSCertKey),
			filepath.Join(c.ESSTLSCertsDir, corev1.TLSPrivateKeyKey),
			false)
		if err != nil {
			return errors.Wrap(err, "Cannot load TLS certificates for external secret stores")
		}

		o.ESSOptions = &controller.ESSOptions{
			TLSConfig:     tcfg,
			TLSSecretName: &c.ESSTLSSecretName,
		}
	}

	if err := apiextensions.Setup(mgr, ao); err != nil {
		return errors.Wrap(err, "Cannot setup API extension controllers")
	}

	po := pkgcontroller.Options{
		Options:         o,
		Cache:           xpkg.NewFsPackageCache(c.CacheDir, afero.NewOsFs()),
		Namespace:       c.Namespace,
		ServiceAccount:  c.ServiceAccount,
		DefaultRegistry: c.Registry,
		FetcherOptions:  []xpkg.FetcherOpt{xpkg.WithUserAgent(c.UserAgent)},

		// TODO(negz): I think this should be ServerTLSSecretName now.
		WebhookTLSSecretName: c.WebhookTLSSecretName,
	}

	if c.CABundlePath != "" {
		rootCAs, err := ParseCertificatesFromPath(c.CABundlePath)
		if err != nil {
			return errors.Wrap(err, "Cannot parse CA bundle")
		}
		po.FetcherOptions = append(po.FetcherOptions, xpkg.WithCustomCA(rootCAs))
	}

	if err := pkg.Setup(mgr, po); err != nil {
		return errors.Wrap(err, "Cannot add packages controllers to manager")
	}

	// Registering webhooks with the manager is what actually starts the webhook
	// server.
	if c.WebhookTLSCertDir != "" {
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

	return errors.Wrap(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
