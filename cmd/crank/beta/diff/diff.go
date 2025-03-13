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
type Cmd struct {
	Namespace string   `default:"crossplane-system" help:"Namespace to compare resources against." name:"namespace" short:"n"`
	Files     []string `arg:"" optional:"" help:"YAML files containing Crossplane resources to diff."`

	Timeout time.Duration `default:"1m" help:"How long to run before timing out."`
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

  # Show the changes that would result from applying xr.yaml (via file) in the 'foobar' namespace.
  crossplane diff xr.yaml -n foobar
`
}

// Run executes the diff command.
func (c *Cmd) Run(k *kong.Context, log logging.Logger, config *rest.Config) error {

	// the rest config here is provided by a function in main.go that's only invoked for commands that request it
	// in their arguments.  that means we won't get errors for cases where the config isn't asked for.

	client, err := ClusterClientFactory(config)
	if err != nil {
		return errors.Wrap(err, "cannot initialize cluster client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	if err := client.Initialize(ctx); err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	loader, err := internal.NewCompositeLoader(c.Files)
	if err != nil {
		return errors.Wrap(err, "cannot create resource loader")
	}

	resources, err := loader.Load()
	if err != nil {
		return errors.Wrap(err, "cannot load resources")
	}

	processor, err := DiffProcessorFactory(config, client, c.Namespace, render.Render, log)
	if err != nil {
		return errors.Wrap(err, "cannot create diff processor")
	}

	err = processor.Initialize(k.Stdout, ctx)
	if err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	if err := processor.ProcessAll(k.Stdout, ctx, resources); err != nil {
		return errors.Wrap(err, "unable to process one or more resources")
	}

	return nil
}

var (
	// ClusterClientFactory Factory function for creating a new cluster client
	ClusterClientFactory = func(config *rest.Config) (cc.ClusterClient, error) {
		return cc.NewClusterClient(config)
	}

	// DiffProcessorFactory Factory function for creating a new diff processor
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc, logger logging.Logger) (dp.DiffProcessor, error) {
		return dp.NewDiffProcessor(config, client, namespace, renderFunc, logger)
	}
)
