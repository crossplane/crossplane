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
	"github.com/crossplane/crossplane/cmd/crank/render"
	"k8s.io/client-go/rest"

	"context"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	dp "github.com/crossplane/crossplane/cmd/crank/beta/diff/diffprocessor"
	"k8s.io/client-go/tools/clientcmd"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Cmd represents the diff command.
type Cmd struct {
	Namespace string   `default:"crossplane-system" help:"Namespace to compare resources against." name:"namespace" short:"n"`
	Files     []string `arg:"" optional:"" help:"YAML files containing Crossplane resources to diff."`
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
func (c *Cmd) Run(ctx context.Context) error {
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig")
	}

	client, err := ClusterClientFactory(config)
	if err != nil {
		return errors.Wrap(err, "cannot initialize cluster client")
	}

	if err := client.Initialize(ctx); err != nil {
		return errors.Wrap(err, "cannot initialize diff processor")
	}

	resources, err := ResourceLoader(c.Files)
	if err != nil {
		return errors.Wrap(err, "failed to load resources")
	}

	processor, err := DiffProcessorFactory(config, client, c.Namespace, render.Render)
	if err != nil {
		return errors.Wrap(err, "cannot create diff processor")
	}

	if err := processor.ProcessAll(ctx, resources); err != nil {
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
	DiffProcessorFactory = func(config *rest.Config, client cc.ClusterClient, namespace string, renderFunc dp.RenderFunc) (dp.DiffProcessor, error) {
		return dp.NewDiffProcessor(config, client, namespace, renderFunc, nil)
	}

	// ResourceLoader Function for loading resources, which can be mocked in tests
	ResourceLoader = LoadResources
)
