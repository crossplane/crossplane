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

// Package trace contains the trace command.
package trace

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	"github.com/crossplane/crossplane/apis/pkg"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/printer"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource/xpkg"
	"github.com/crossplane/crossplane/cmd/crank/beta/trace/internal/resource/xrm"
)

const (
	errGetResource            = "cannot get requested resource"
	errCliOutput              = "cannot print output"
	errKubeConfig             = "failed to get kubeconfig"
	errInitKubeClient         = "cannot init kubeclient"
	errGetDiscoveryClient     = "cannot get discovery client"
	errGetMapping             = "cannot get mapping for resource"
	errInitPrinter            = "cannot init new printer"
	errMissingName            = "missing name, must be provided separately 'TYPE[.VERSION][.GROUP] [NAME]' or in the 'TYPE[.VERSION][.GROUP][/NAME]' format"
	errNameDoubled            = "name provided twice, must be provided separately 'TYPE[.VERSION][.GROUP] [NAME]' or in the 'TYPE[.VERSION][.GROUP][/NAME]' format"
	errInvalidResource        = "invalid resource, must be provided in the 'TYPE[.VERSION][.GROUP][/NAME]' format"
	errInvalidResourceAndName = "invalid resource and name"
)

// Cmd builds the trace tree for a Crossplane resource.
type Cmd struct {
	Resource string `arg:"" help:"Kind of the Crossplane resource, accepts the 'TYPE[.VERSION][.GROUP][/NAME]' format."`
	Name     string `arg:"" optional:"" help:"Name of the Crossplane resource, can be passed as part of the resource too."`

	// TODO(phisco): add support for all the usual kubectl flags; configFlags := genericclioptions.NewConfigFlags(true).AddFlags(...)
	// TODO(phisco): move to namespace defaulting to "" and use the current context's namespace
	Namespace                 string `short:"n" name:"namespace" help:"Namespace of the resource." default:"default"`
	Output                    string `short:"o" name:"output" help:"Output format. One of: default, wide, json, dot." enum:"default,wide,json,dot" default:"default"`
	ShowConnectionSecrets     bool   `short:"s" name:"show-connection-secrets" help:"Show connection secrets in the output."`
	ShowPackageDependencies   string `name:"show-package-dependencies" help:"Show package dependencies in the output. One of: unique, all, none." enum:"unique,all,none" default:"unique"`
	ShowPackageRevisions      string `name:"show-package-revisions" help:"Show package revisions in the output. One of: active, all, none." enum:"active,all,none" default:"active"`
	ShowPackageRuntimeConfigs bool   `name:"show-package-runtime-configs" help:"Show package runtime configs in the output." default:"false"`
}

// Help returns help message for the trace command.
func (c *Cmd) Help() string {
	return `
This command trace a Crossplane resource (Claim, Composite, or Managed Resource)
to get a detailed output of its relationships, helpful for troubleshooting.

If needed the resource kind can be also specified further,
'TYPE[.VERSION][.GROUP]', e.g. mykind.example.org or
mykind.v1alpha1.example.org.

Examples:
  # Trace a MyKind resource (mykinds.example.org/v1alpha1) named 'my-res' in the namespace 'my-ns'
  crossplane beta trace mykind my-res -n my-ns

  # Output wide format, showing full errors and condition messages
  crossplane beta trace mykind my-res -n my-ns -o wide

  # Show connection secrets in the output
  crossplane beta trace mykind my-res -n my-ns --show-connection-secrets

  # Output a graph in dot format and pipe to dot to generate a png
  crossplane beta trace mykind my-res -n my-ns -o dot | dot -Tpng -o output.png

  # Output all retrieved resources to json and pipe to jq to have it coloured
  crossplane beta trace mykind my-res -n my-ns -o json | jq

  # Output debug logs to stderr while redirecting a dot formatted graph to dot
  crossplane beta trace mykind my-res -n my-ns -o dot --verbose | dot -Tpng -o output.png
`
}

// Run runs the trace command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error { //nolint:gocyclo // TODO(phisco): refactor
	ctx := context.Background()
	logger = logger.WithValues("Resource", c.Resource, "Name", c.Name)

	// Init new printer
	p, err := printer.New(c.Output)
	if err != nil {
		return errors.Wrap(err, errInitPrinter)
	}
	logger.Debug("Built printer", "output", c.Output)

	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return errors.Wrap(err, errKubeConfig)
	}
	logger.Debug("Found kubeconfig")

	client, err := client.New(kubeconfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		return errors.Wrap(err, errInitKubeClient)
	}

	// add package scheme
	_ = pkg.AddToScheme(client.Scheme())

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return errors.Wrap(err, errGetDiscoveryClient)
	}
	// TODO(phisco): properly handle flags and switch to file backed cache
	// 	(restmapper.NewDeferredDiscoveryRESTMapper), as cli-runtime
	// 	pkg/resource Builder does.
	d := memory.NewMemCacheClient(discoveryClient)
	rmapper := restmapper.NewShortcutExpander(restmapper.NewDeferredDiscoveryRESTMapper(d), d)

	res, name, err := c.getResourceAndName()
	if err != nil {
		return errors.Wrap(err, errInvalidResourceAndName)
	}

	mapping, err := resource.MappingFor(rmapper, res)
	if err != nil {
		return errors.Wrap(err, errGetMapping)
	}

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	rootRef := &v1.ObjectReference{
		Kind:       mapping.GroupVersionKind.Kind,
		APIVersion: mapping.GroupVersionKind.GroupVersion().String(),
		Name:       name,
	}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && c.Namespace != "" {
		logger.Debug("Requested resource is namespaced", "namespace", c.Namespace)
		rootRef.Namespace = c.Namespace
	}
	logger.Debug("Getting resource tree", "rootRef", rootRef.String())
	// Get client for k8s package
	root := resource.GetResource(ctx, client, rootRef)
	// We should just surface any error getting the root resource immediately.
	if err := root.Error; err != nil {
		return errors.Wrap(err, errGetResource)
	}

	var treeClient resource.TreeClient
	switch {
	case xpkg.IsPackageType(mapping.GroupVersionKind.GroupKind()):
		logger.Debug("Requested resource is an Package")
		treeClient, err = xpkg.NewClient(client,
			xpkg.WithDependencyOutput(xpkg.DependencyOutput(c.ShowPackageDependencies)),
			xpkg.WithPackageRuntimeConfigs(c.ShowPackageRuntimeConfigs),
			xpkg.WithRevisionOutput(xpkg.RevisionOutput(c.ShowPackageRevisions)))
		if err != nil {
			return errors.Wrap(err, errInitKubeClient)
		}
	default:
		logger.Debug("Requested resource is not a package, assumed to be an XR, XRC or MR")
		treeClient, err = xrm.NewClient(client, xrm.WithConnectionSecrets(c.ShowConnectionSecrets))
		if err != nil {
			return errors.Wrap(err, errInitKubeClient)
		}
	}
	logger.Debug("Built client")

	root, err = treeClient.GetResourceTree(ctx, root)
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

func (c *Cmd) getResourceAndName() (string, string, error) {
	// If no resource was provided, error out (should never happen as it's
	// required by Kong)
	if c.Resource == "" {
		return "", "", errors.New(errInvalidResource)
	}

	// Split the resource into its components
	splittedResource := strings.Split(c.Resource, "/")
	length := len(splittedResource)

	if length == 1 {
		// If no name is provided, error out
		if c.Name == "" {
			return "", "", errors.New(errMissingName)
		}

		// Resource has only kind and the name is separately provided
		return splittedResource[0], c.Name, nil
	}

	if length == 2 {
		// If a name is separately provided, error out
		if c.Name != "" {
			return "", "", errors.New(errNameDoubled)
		}

		// Resource includes both kind and name
		return splittedResource[0], splittedResource[1], nil
	}

	// Handle the case when resource format is invalid
	return "", "", errors.New(errInvalidResource)
}
