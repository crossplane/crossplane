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

// Package diff contains the diff command.
package diff

import (
	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/render"
	"k8s.io/client-go/rest"
	"time"

	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
)

// Cmd represents the diff command.
// Cmd represents the diff command.
type Cmd struct {
	Namespace string   `default:"crossplane-system" help:"Namespace to compare resources against." name:"namespace" short:"n"`
	Files     []string `arg:"" optional:"" help:"YAML files containing Crossplane resources to diff."`

	// Configuration options
	NoColor bool          `help:"Disable colorized output." name:"no-color"`
	Compact bool          `help:"Show compact diffs with minimal context." name:"compact"`
	Timeout time.Duration `default:"1m" help:"How long to run before timing out."`
	QPS     float32       `help:"Maximum QPS to the API server." default:"0"`
	Burst   int           `help:"Maximum burst for throttle." default:"0"`
}

// Help returns help instructions for the diff command.
func (c *Cmd) Help() string {
	return `
This command returns a diff of the in-cluster resources that would be modified if the provided Crossplane resources were applied.

Similar to kubectl diff, it requires Crossplane to be operating in the live cluster found in your kubeconfig.

Examples:
  # Show the changes that would result from applying xr.yaml (via file) in the default 'crossplane-system' namespace.
  crossplane diff xr.yaml

  # Show the changes that would result from applying xr.yaml (via stdin) in the default 'crossplane-system' namespace.
  cat xr.yaml | crossplane diff --

  # Show the changes that would result from applying xr.yaml, xr1.yaml, and xr2.yaml in the default 'crossplane-system' namespace.
  cat xr.yaml | crossplane diff xr1.yaml xr2.yaml --

  # Show the changes that would result from applying xr.yaml (via file) in the 'foobar' namespace with no color output.
  crossplane diff xr.yaml -n foobar --no-color

  # Show the changes in a compact format with minimal context.
  crossplane diff xr.yaml --compact
`
}

// AfterApply implements kong's AfterApply method to bind our dependencies
func (c *Cmd) AfterApply(ctx *kong.Context, log logging.Logger, config *rest.Config) error {

	client, err := getDefaultClusterClient(c, config, log)
	if err != nil {
		return errors.Wrap(err, "unable to create cluster client for binding")
	}

	proc, err := getDefaultProc(c, config, log, client)
	if err != nil {
		return errors.Wrap(err, "unable to create diff processor for binding")
	}

	loader, err := getDefaultLoader(c)
	if err != nil {
		return errors.Wrap(err, "cannot create resource loader")
	}

	ctx.BindTo(client, (*cc.ClusterClient)(nil))
	ctx.BindTo(proc, (*dp.DiffProcessor)(nil))
	ctx.BindTo(loader, (*internal.Loader)(nil))
	return nil
}

func getDefaultProc(c *Cmd, config *rest.Config, log logging.Logger, client cc.ClusterClient) (dp.DiffProcessor, error) {
	// Create the options for the processor
	options := []dp.ProcessorOption{
		dp.WithRestConfig(config),
		dp.WithNamespace(c.Namespace),
		dp.WithLogger(log),
		dp.WithRenderFunc(render.Render),
		dp.WithColorize(!c.NoColor),
		dp.WithCompact(c.Compact),
	}
	return dp.NewDiffProcessor(client, options...)
}

func getDefaultClusterClient(c *Cmd, config *rest.Config, log logging.Logger) (cc.ClusterClient, error) {
	return cc.NewClusterClient(config,
		cc.WithLogger(log),
		cc.WithQPS(c.QPS),
		cc.WithBurst(c.Burst))
}

func getDefaultLoader(c *Cmd) (internal.Loader, error) {
	return internal.NewCompositeLoader(c.Files)
}

// Run executes the diff command.
func (c *Cmd) Run(k *kong.Context, _ logging.Logger, client cc.ClusterClient, proc dp.DiffProcessor, loader internal.Loader) error {
	// the rest config here is provided by a function in main.go that's only invoked for commands that request it
	// in their arguments.  that means we won't get "can't find kubeconfig" errors for cases where the config isn't asked for.

	// TODO:  add a file output option
	// TODO:  make sure namespacing works everywhere; what to do with the -n argument?
	// TODO:  test for the case of applying a namespaced object inside a composition using fn-gotemplating inside fn-kubectl?
	// TODO:  add test for new vs updated XRs with downstream fields plumbed from Status field
	// TODO:  diff against upgraded schema that isn't applied yet
	// TODO:  diff against upgraded composition that isn't applied yet
	// TODO:  diff against upgraded composition version that is already available

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		return errors.Wrap(err, "cannot initialize client")
	}

	resources, err := loader.Load()
	if err != nil {
		return errors.Wrap(err, "cannot load resources")
	}

	err = proc.Initialize(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	if err := proc.PerformDiff(ctx, k.Stdout, resources); err != nil {
		return errors.Wrap(err, "unable to process one or more resources")
	}

	return nil
}
