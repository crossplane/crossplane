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
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	kcache "k8s.io/client-go/tools/cache"
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
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	"github.com/crossplane/crossplane/internal/controller/apiextensions"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/controller/pkg"
	pkgcontroller "github.com/crossplane/crossplane/internal/controller/pkg/controller"
	"github.com/crossplane/crossplane/internal/engine"
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

// Command runs the core crossplane controllers.
type Command struct {
	Start startCommand `cmd:"" help:"Start Crossplane controllers."`
	Init  initCommand  `cmd:"" help:"Make cluster ready for Crossplane controllers."`
}

// KongVars represent the kong variables associated with the CLI parser
// required for the Registry default variable interpolation.
var KongVars = kong.Vars{ //nolint:gochecknoglobals // We treat these as constants.
	"default_registry":   xpkg.DefaultRegistry,
	"default_user_agent": transport.DefaultUserAgent(),
}

// Run is the no-op method required for kong call tree
// Kong requires each node in the calling path to have associated
// Run method.
func (c *Command) Run() error {
	return nil
}

type startCommand struct {
	Profile string `help:"Serve runtime profiling data via HTTP at /debug/pprof." placeholder:"host:port"`

	Namespace      string `default:"crossplane-system"     env:"POD_NAMESPACE"                                                      help:"Namespace used to unpack and run packages."                         short:"n"`
	ServiceAccount string `default:"crossplane"            env:"POD_SERVICE_ACCOUNT"                                                help:"Name of the Crossplane Service Account."`
	CacheDir       string `default:"/cache"                env:"CACHE_DIR"                                                          help:"Directory used for caching package images."                         short:"c"`
	LeaderElection bool   `default:"false"                 env:"LEADER_ELECTION"                                                    help:"Use leader election for the controller manager."                    short:"l"`
	Registry       string `default:"${default_registry}"   env:"REGISTRY"                                                           help:"Default registry used to fetch packages when not specified in tag." short:"r"`
	CABundlePath   string `env:"CA_BUNDLE_PATH"            help:"Additional CA bundle to use when fetching packages from registry."`
	UserAgent      string `default:"${default_user_agent}" env:"USER_AGENT"                                                         help:"The User-Agent header that will be set on all package requests."`

	PackageRuntime string `default:"Deployment" env:"PACKAGE_RUNTIME" help:"The package runtime to use for packages with a runtime (e.g. Providers and Functions)"`

	SyncInterval                     time.Duration `default:"1h"  help:"How often all resources will be double-checked for drift from the desired state."                      short:"s"`
	PollInterval                     time.Duration `default:"1m"  help:"How often individual resources will be checked for drift from the desired state."`
	MaxReconcileRate                 int           `default:"100" help:"The global maximum rate per second at which resources may checked for drift from the desired state."`
	MaxConcurrentPackageEstablishers int           `default:"10"  help:"The the maximum number of goroutines to use for establishing Providers, Configurations and Functions."`

	WebhookEnabled                      bool `default:"true"  env:"WEBHOOK_ENABLED"                        help:"Enable webhook configuration."`
	AutomaticDependencyDowngradeEnabled bool `default:"false" env:"AUTOMATIC_DEPENDENCY_DOWNGRADE_ENABLED" help:"Enable automatic dependency version downgrades. This configuration requires the 'EnableDependencyVersionUpgrades' feature flag to be enabled."`

	TLSServerSecretName string `env:"TLS_SERVER_SECRET_NAME" help:"The name of the TLS Secret that will store Crossplane's server certificate."`
	TLSServerCertsDir   string `env:"TLS_SERVER_CERTS_DIR"   help:"The path of the folder which will store TLS server certificate of Crossplane."`
	TLSClientSecretName string `env:"TLS_CLIENT_SECRET_NAME" help:"The name of the TLS Secret that will be store Crossplane's client certificate."`
	TLSClientCertsDir   string `env:"TLS_CLIENT_CERTS_DIR"   help:"The path of the folder which will store TLS client certificate of Crossplane."`

	EnableExternalSecretStores      bool `group:"Alpha Features:" help:"Enable support for External Secret Stores."`
	EnableRealtimeCompositions      bool `group:"Alpha Features:" help:"Enable support for realtime compositions, i.e. watching composed resources and reconciling compositions immediately when any of the composed resources is updated."`
	EnableDependencyVersionUpgrades bool `group:"Alpha Features:" help:"Enable support for upgrading dependency versions when the parent package is updated."`
	EnableSignatureVerification     bool `group:"Alpha Features:" help:"Enable support for package signature verification via ImageConfig API."`

	EnableCompositionWebhookSchemaValidation bool `default:"true" group:"Beta Features:" help:"Enable support for Composition validation using schemas."`
	EnableDeploymentRuntimeConfigs           bool `default:"true" group:"Beta Features:" help:"Enable support for Deployment Runtime Configs."`
	EnableUsages                             bool `default:"true" group:"Beta Features:" help:"Enable support for deletion ordering and resource protection with Usages."`
	EnableSSAClaims                          bool `default:"true" group:"Beta Features:" help:"Enable support for using Kubernetes server-side apply to sync claims with composite resources (XRs)."`

	// These are GA features that previously had alpha or beta feature flags.
	// You can't turn off a GA feature. We maintain the flags to avoid breaking
	// folks who are passing them, but they do nothing. The flags are hidden so
	// they don't show up in the help output.
	EnableCompositionRevisions               bool `default:"true" hidden:""`
	EnableCompositionFunctions               bool `default:"true" hidden:""`
	EnableCompositionFunctionsExtraResources bool `default:"true" hidden:""`

	// These are alpha features that we've removed support for. Crossplane
	// returns an error when you enable them. This ensures you'll see an
	// explicit and informative error on startup, instead of a potentially
	// surprising one later.
	EnableEnvironmentConfigs bool `hidden:""`
}

// Run core Crossplane controllers.
func (c *startCommand) Run(s *runtime.Scheme, log logging.Logger) error { //nolint:gocognit // Only slightly over.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, "cannot get config")
	}

	cfg.WarningHandler = rest.NewWarningWriter(os.Stderr, rest.WarningWriterOptions{
		// Warnings from API requests should be deduplicated so they are only logged once
		Deduplicate: true,
	})

	// The claim and XR controllers don't use the manager's cache or client.
	// They use their own. They're setup later in this method.
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
		log.Info("Composition Revisions are GA and cannot be disabled. The --enable-composition-revisions flag will be removed in a future release.")
	}
	if !c.EnableCompositionFunctions {
		log.Info("Composition Functions are GA and cannot be disabled. The --enable-composition-functions flag will be removed in a future release.")
	}
	if !c.EnableCompositionFunctionsExtraResources {
		log.Info("Extra Resources are GA and cannot be disabled. The --enable-composition-functions-extra-resources flag will be removed in a future release.")
	}

	// TODO(negz): Include a link to a migration guide.
	if c.EnableEnvironmentConfigs {
		//nolint:revive // This is long. It's easier to read with punctuation.
		return errors.New("Crossplane no longer supports loading and patching EnvironmentConfigs natively. Please use function-environment-configs instead. The --enable-environment-configs flag will be removed in a future release.")
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
	functionRunner := xfn.NewPackagedFunctionRunner(mgr.GetClient(),
		xfn.WithLogger(log),
		xfn.WithTLSConfig(clienttls),
		xfn.WithInterceptorCreators(m),
	)

	// Periodically remove clients for Functions that no longer exist.
	go functionRunner.GarbageCollectConnections(ctx, 10*time.Minute)

	if c.EnableCompositionWebhookSchemaValidation {
		o.Features.Enable(features.EnableBetaCompositionWebhookSchemaValidation)
		log.Info("Beta feature enabled", "flag", features.EnableBetaCompositionWebhookSchemaValidation)
	}
	if c.EnableUsages {
		o.Features.Enable(features.EnableBetaUsages)
		log.Info("Beta feature enabled", "flag", features.EnableBetaUsages)
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
		o.Features.Enable(features.EnableAlphaRealtimeCompositions)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaRealtimeCompositions)
	}
	if c.EnableDeploymentRuntimeConfigs {
		o.Features.Enable(features.EnableBetaDeploymentRuntimeConfigs)
		log.Info("Beta feature enabled", "flag", features.EnableBetaDeploymentRuntimeConfigs)
	}
	if c.EnableSSAClaims {
		o.Features.Enable(features.EnableBetaClaimSSA)
		log.Info("Beta feature enabled", "flag", features.EnableBetaClaimSSA)
	}
	if c.EnableDependencyVersionUpgrades {
		o.Features.Enable(features.EnableAlphaDependencyVersionUpgrades)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaDependencyVersionUpgrades)

		if c.AutomaticDependencyDowngradeEnabled {
			log.Info("Automatic dependency downgrade is enabled.")
		}
	}
	if c.EnableSignatureVerification {
		o.Features.Enable(features.EnableAlphaSignatureVerification)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaSignatureVerification)
	}

	// Claim and XR controllers are started and stopped dynamically by the
	// ControllerEngine below. When realtime compositions are enabled, they also
	// start and stop their watches (e.g. of composed resources) dynamically. To
	// do this, the ControllerEngine must have exclusive ownership of a cache.
	// This allows it to track what controllers are using the cache's informers.
	ca, err := cache.New(mgr.GetConfig(), cache.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     mgr.GetScheme(),
		Mapper:     mgr.GetRESTMapper(),
		SyncPeriod: &c.SyncInterval,

		// When a CRD is deleted, any informers for its GVKs will start trying
		// to restart their watches, and fail with scary errors. This should
		// only happen when realtime composition is enabled, and we should GC
		// the informer within 60 seconds. This handler tries to make the error
		// a little more informative, and less scary.
		DefaultWatchErrorHandler: func(_ *kcache.Reflector, err error) {
			if errors.Is(io.EOF, err) {
				// Watch closed normally.
				return
			}
			log.Debug("Watch error - probably due to CRD being uninstalled", "error", err)
		},
	})
	if err != nil {
		return errors.Wrap(err, "cannot create cache for API extension controllers")
	}

	go func() {
		// Don't start the cache until the manager is elected.
		<-mgr.Elected()

		if err := ca.Start(ctx); err != nil {
			log.Info("API extensions cache returned an error", "error", err)
		}

		log.Info("API extensions cache stopped")
	}()

	cl, err := client.New(mgr.GetConfig(), client.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Scheme:     mgr.GetScheme(),
		Mapper:     mgr.GetRESTMapper(),
		Cache: &client.CacheOptions{
			Reader: ca,

			// Don't cache secrets - there may be a lot of them.
			DisableFor: []client.Object{&corev1.Secret{}},

			// Cache unstructured resources (like XRs and MRs) on Get and List.
			Unstructured: true,
		},
	})
	if err != nil {
		return errors.Wrap(err, "cannot create client for API extension controllers")
	}

	// It's important the engine's client is wrapped with unstructured.NewClient
	// because controller-runtime always caches *unstructured.Unstructured, not
	// our wrapper types like *composite.Unstructured. This client takes care of
	// automatically wrapping and unwrapping *unstructured.Unstructured.
	ce := engine.New(mgr,
		engine.TrackInformers(ca, mgr.GetScheme()),
		unstructured.NewClient(cl),
		engine.WithLogger(log),
	)

	// TODO(negz): Garbage collect informers for CRs that are still defined
	// (i.e. still have CRDs) but aren't used? Currently if an XR starts
	// composing a kind of CR then stops, we won't stop the unused informer
	// until the CRD that defines the CR is deleted. That could never happen.
	// Consider for example composing two types of MR from the same provider,
	// then updating to compose only one.

	// Garbage collect informers for custom resources when their CRD is deleted.
	if err := ce.GarbageCollectCustomResourceInformers(ctx); err != nil {
		return errors.Wrap(err, "cannot start garbage collector for custom resource informers")
	}

	ao := apiextensionscontroller.Options{
		Options:          o,
		ControllerEngine: ce,
		FunctionRunner:   functionRunner,
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
		Options:                             o,
		Cache:                               xpkg.NewFsPackageCache(c.CacheDir, afero.NewOsFs()),
		Namespace:                           c.Namespace,
		ServiceAccount:                      c.ServiceAccount,
		DefaultRegistry:                     c.Registry,
		FetcherOptions:                      []xpkg.FetcherOpt{xpkg.WithUserAgent(c.UserAgent)},
		PackageRuntime:                      pr,
		MaxConcurrentPackageEstablishers:    c.MaxConcurrentPackageEstablishers,
		AutomaticDependencyDowngradeEnabled: c.AutomaticDependencyDowngradeEnabled,
	}

	// We need to set the TUF_ROOT environment variable so that the TUF client
	// knows where to store its data. A directory under CacheDir is a good place
	// for this because it's a place that Crossplane has write access to, and
	// we already use it for caching package images.
	// Check the following to see how it defaults otherwise and where those
	// ".sigstore/root" is coming from: https://github.com/sigstore/sigstore/blob/ecaaf75cf3a942cf224533ae15aee6eec19dc1e2/pkg/tuf/client.go#L558
	// Check the following to read more about what TUF is and why it exists
	// in this context: https://blog.sigstore.dev/the-update-framework-and-you-2f5cbaa964d5/
	if err = os.Setenv("TUF_ROOT", filepath.Join(c.CacheDir, ".sigstore", "root")); err != nil {
		return errors.Wrap(err, "cannot set TUF_ROOT environment variable")
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
		if o.Features.Enabled(features.EnableBetaUsages) {
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
