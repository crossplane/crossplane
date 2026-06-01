/*
Copyright 2025 The Crossplane Authors.

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

// Package check implements the `crossplane beta upgrade check` command.
package check

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kong"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis"
)

// Cmd checks a Crossplane control plane for features that are removed or
// broken in Crossplane v2.
type Cmd struct {
	Kubeconfig           string `help:"Path to the kubeconfig file. Defaults to $KUBECONFIG or ~/.kube/config."       name:"kubeconfig"                                                                                                                 type:"existingfile"`
	Context              string `help:"Kubernetes context to use from the kubeconfig."                                name:"context"                                                                                                                    predictor:"context"                       short:"c"`
	Namespace            string `help:"Restrict namespaced checks to a single namespace. Defaults to all namespaces." name:"namespace"                                                                                                                  predictor:"namespace"                     short:"n"`
	CrossplaneNamespace  string `default:"crossplane-system"                                                          help:"Namespace where the Crossplane Deployment runs."                                                                            name:"crossplane-namespace"`
	CrossplaneSelector   string `default:"app=crossplane"                                                             help:"Label selector for the Crossplane Deployment."                                                                              name:"crossplane-selector"`
	Output               string `default:"text"                                                                       enum:"text,json"                                                                                                                  help:"Output format. One of: text, json." name:"output" short:"o"`
	SkipManagedResources bool   `default:"false"                                                                      help:"Skip scanning managed resources for external secret stores usage. Speeds up the check on clusters with many provider CRDs." name:"skip-managed-resources"`
	Concurrency          int    `default:"10"                                                                         help:"Maximum number of resources to process in parallel."                                                                        name:"concurrency"`
}

// Help returns help text for the check command.
func (c *Cmd) Help() string {
	return `
Checks a Crossplane control plane to see if it is ready to safely upgrade to
Crossplane v2 by checking for all features that were removed or had
breaking changes in Crossplane v2. Run this against a v1.x control plane to
surface any usage of APIs that will not work after upgrading to v2.

By default the check sweeps the entire control plane: cluster-scoped
resources and all namespaces. Use --namespace to restrict namespaced checks
(e.g. Claims) to a single namespace.

Exits non-zero if any issue-severity findings are produced or any check is
incomplete (errored out before producing a result). Informational findings
(flagged with [i]) do not trigger a non-zero exit as they highlight
functionality that isn't strictly affected by a v2 upgrade, but is worth
knowing about nonetheless.

To learn more about the breaking changes in Crossplane v2, see the docs:
https://docs.crossplane.io/latest/whats-new/#backward-compatibility

Examples:
  # Check a control plane using the current kubeconfig context and all namespaces
  crossplane beta upgrade check

  # Point at a specific kubeconfig and context
  crossplane beta upgrade check --kubeconfig ~/.kube/prod.yaml --context prod

  # Restrict Claim checks to a single namespace
  crossplane beta upgrade check -n team-a

  # Output JSON for CI/automation
  crossplane beta upgrade check -o json
`
}

// Run executes the check command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error {
	// Support cancelling the tool (and all in-flight API calls) on Ctrl-C or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if c.Kubeconfig != "" {
		loadingRules.ExplicitPath = c.Kubeconfig
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{CurrentContext: c.Context},
	).ClientConfig()
	if err != nil {
		return errors.Wrap(err, "cannot load kubeconfig")
	}
	// Give ourselves a bit more QPS/burst than the defaults for our API calls,
	// we have a lot of checks to get through, and this is a one time tool run, not an
	// always on controller, so the potential perf impact has a limited duration.
	if cfg.QPS == 0 {
		cfg.QPS = 20
	}
	if cfg.Burst == 0 {
		cfg.Burst = 30
	}

	sch := runtime.NewScheme()
	if err := scheme.AddToScheme(sch); err != nil {
		return errors.Wrap(err, "cannot register Crossplane types with scheme")
	}
	if err := extv1.AddToScheme(sch); err != nil {
		return errors.Wrap(err, "cannot register Crossplane types with scheme")
	}
	if err := apis.AddToScheme(sch); err != nil {
		return errors.Wrap(err, "cannot register Crossplane types with scheme")
	}

	kube, err := client.New(cfg, client.Options{Scheme: sch})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes client")
	}

	checks := []Check{
		&NativePatchAndTransform{Client: kube},
		&ControllerConfigCheck{Client: kube},
		&ExternalSecretStores{
			Client:               kube,
			CrossplaneNamespace:  c.CrossplaneNamespace,
			Selector:             c.CrossplaneSelector,
			ClaimNamespace:       c.Namespace,
			SkipManagedResources: c.SkipManagedResources,
			Concurrency:          c.Concurrency,
		},
		&CompositeConnectionDetails{Client: kube, Namespace: c.Namespace},
		&UnqualifiedPackageSources{Client: kube},
	}

	runner := &Runner{Checks: checks, Logger: logger}
	report := runner.Run(ctx)

	p, err := NewPrinter(c.Output)
	if err != nil {
		return err
	}
	if err := p.Print(k.Stdout, report); err != nil {
		return errors.Wrap(err, "cannot print report")
	}

	// Returning a non-nil error here makes kong exit non-zero; the report
	// is already printed. The blank line on stderr separates the report
	// from kong's "crossplane: error: ..." line, which lands on stderr
	// immediately after this return.
	if report.HasBlockers() {
		_, _ = fmt.Fprintln(k.Stderr)
		return errors.New("blockers found")
	}
	return nil
}
