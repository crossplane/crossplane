/*
Copyright 2023 The Crossplane Authors.

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

// Package describe contains the describe command.
package describe

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/cmd/crank/beta/describe/internal/printer"
	"github.com/crossplane/crossplane/cmd/crank/beta/describe/internal/resource"
)

const (
	errGetResource            = "cannot get requested resource"
	errCliOutput              = "cannot print output"
	errKubeConfig             = "failed to get kubeconfig"
	errCouldntInitKubeClient  = "cannot init kubeclient"
	errCannotGetKindAndName   = "cannot get kind and name"
	errCannotGetMapping       = "cannot get mapping for resource"
	errCannotInitPrinter      = "cannot init new printer"
	errFmtInvalidResourceName = "invalid combined kind and name format, should be in the form of 'resource.group.example.org/name', got: %q"
)

// Cmd describes a Crossplane resource.
type Cmd struct {
	Resource string `arg:"" required:"" help:"'TYPE[.VERSION][.GROUP][/NAME]' identifying the Crossplane resource."`
	Name     string `arg:"" optional:"" help:"Name of the Crossplane resource. Ignored if already passed as part of the RESOURCE argument."`

	// TODO(phisco): add support for all the usual kubectl flags; configFlags := genericclioptions.NewConfigFlags(true).AddFlags(...)
	Namespace string `short:"n" name:"namespace" help:"Namespace of resource to describe." default:"default"`
	Output    string `short:"o" name:"output" help:"Output format. One of: default, json." enum:"default,json" default:"default"`
}

// Run runs the describe command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error {
	logger = logger.WithValues("Resource", c.Resource)

	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		logger.Debug(errKubeConfig, "error", err)
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	// Get client for k8s package
	client, err := resource.NewClient(kubeconfig)
	if err != nil {
		return errors.Wrap(err, errCouldntInitKubeClient)
	}
	logger.Debug("Built client")

	kind, name, err := c.getKindAndName()
	if err != nil {
		return errors.Wrap(err, errCannotGetKindAndName)
	}

	mapping, err := client.MappingFor(kind)
	if err != nil {
		return errors.Wrap(err, errCannotGetMapping)
	}

	// Init new printer
	p, err := printer.New(c.Output)
	if err != nil {
		return errors.Wrap(err, errCannotInitPrinter)
	}
	logger.Debug("Built printer", "output", c.Output)

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	rootRef := &v1.ObjectReference{
		Kind:       mapping.GroupVersionKind.Kind,
		APIVersion: mapping.GroupVersionKind.GroupVersion().String(),
		Name:       name,
	}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && c.Namespace != "" {
		rootRef.Namespace = c.Namespace
	}
	logger.Debug("Getting resource tree", "rootRef", rootRef.String())
	root, err := client.GetResourceTree(context.Background(), rootRef)
	if err != nil {
		logger.Debug(errGetResource, "error", err)
		return errors.Wrap(err, errGetResource)
	}
	logger.Debug("Got resource tree", "root", root)

	// Print resources
	err = p.Print(k.Stdout, root)
	if err != nil {
		return errors.Wrap(err, errCliOutput)
	}

	return nil
}

func (c *Cmd) getKindAndName() (string, string, error) {
	if c.Name != "" {
		return c.Resource, c.Name, nil
	}
	kindAndName := strings.SplitN(c.Resource, "/", 2)
	if len(kindAndName) != 2 {
		return "", "", errors.Errorf(errFmtInvalidResourceName, c.Resource)
	}
	return kindAndName[0], kindAndName[1], nil
}
