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
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	"github.com/crossplane/crossplane/v2/apis/pkg"
	"github.com/crossplane/crossplane/v2/cmd/crank/beta/trace/internal/printer"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource/xpkg"
	"github.com/crossplane/crossplane/v2/cmd/crank/common/resource/xrm"
	"github.com/crossplane/crossplane/v2/cmd/crank/internal"
)

const (
	errGetResource            = "cannot get requested resource"
	errCliOutput              = "cannot print output"
	errKubeConfig             = "failed to get kubeconfig"
	errKubeNamespace          = "failed to get namespace from kubeconfig"
	errInitKubeClient         = "cannot init kubeclient"
	errGetDiscoveryClient     = "cannot get discovery client"
	errGetMapping             = "cannot get mapping for resource"
	errInitPrinter            = "cannot init new printer"
	errNameDoubled            = "name provided twice, must be provided separately 'TYPE[.VERSION][.GROUP] [NAME]' or in the 'TYPE[.VERSION][.GROUP][/NAME]' format"
	errInvalidResource        = "invalid resource, must be provided in the 'TYPE[.VERSION][.GROUP][/NAME]' format"
	errInvalidResourceAndName = "invalid resource and name"
)

// Cmd builds the trace tree for a Crossplane resource.
type Cmd struct {
	Resource string `arg:"" help:"Kind of the Crossplane resource, accepts the 'TYPE[.VERSION][.GROUP][/NAME]' format." predictor:"k8s_resource"`
	Name     string `arg:"" help:"Name of the Crossplane resource, can be passed as part of the resource too."          optional:""              predictor:"k8s_resource_name"`

	// TODO(phisco): add support for all the usual kubectl flags; configFlags := genericclioptions.NewConfigFlags(true).AddFlags(...)
	Context                   string `default:""                                    help:"Kubernetes context."                             name:"context"                                                             predictor:"context"              short:"c"`
	Namespace                 string `default:""                                    help:"Namespace of the resource."                      name:"namespace"                                                           predictor:"namespace"            short:"n"`
	Output                    string `default:"default"                             enum:"default,wide,json,dot"                           help:"Output format. One of: default, wide, json, dot."                    name:"output"                    short:"o"`
	ShowConnectionSecrets     bool   `help:"Show connection secrets in the output." name:"show-connection-secrets"                         short:"s"`
	ShowPackageDependencies   string `default:"unique"                              enum:"unique,all,none"                                 help:"Show package dependencies in the output. One of: unique, all, none." name:"show-package-dependencies"`
	ShowPackageRevisions      string `default:"active"                              enum:"active,all,none"                                 help:"Show package revisions in the output. One of: active, all, none."    name:"show-package-revisions"`
	ShowPackageRuntimeConfigs bool   `default:"false"                               help:"Show package runtime configs in the output."     name:"show-package-runtime-configs"`
	Concurrency               int    `default:"5"                                   help:"load concurrency"                                name:"concurrency"`
	Watch                     bool   `default:"false"                               help:"Watch for changes until the resource is deleted" name:"watch"                                                               short:"w"`
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

  # Trace all MyKind resources (mykinds.example.org/v1alpha1) in the namespace 'my-ns'
  crossplane beta trace mykind -n my-ns

  # Output wide format, showing full errors and condition messages, and other useful info 
  # depending on the target type, e.g. composed resources names for composite resources or image used for packages
  crossplane beta trace mykind my-res -n my-ns -o wide

  # Show connection secrets in the output
  crossplane beta trace mykind my-res -n my-ns --show-connection-secrets

  # Output a graph in dot format and pipe to dot to generate a png
  crossplane beta trace mykind my-res -n my-ns -o dot | dot -Tpng -o output.png

  # Output all retrieved resources to json and pipe to jq to have it coloured
  crossplane beta trace mykind my-res -n my-ns -o json | jq

  # Output debug logs to stderr while redirecting a dot formatted graph to dot
  crossplane beta trace mykind my-res -n my-ns -o dot --verbose | dot -Tpng -o output.png

  # Watch a resource continuously until it is deleted
  crossplane beta trace mykind my-res -n my-ns --watch
`
}

// Run runs the trace command.
func (c *Cmd) Run(k *kong.Context, logger logging.Logger) error {
	ctx := context.Background()
	logger = logger.WithValues("Resource", c.Resource, "Name", c.Name)

	// Init new printer
	p, err := printer.New(c.Output)
	if err != nil {
		return errors.Wrap(err, errInitPrinter)
	}

	logger.Debug("Built printer", "output", c.Output)

	clientconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: c.Context},
	)

	kubeconfig, err := clientconfig.ClientConfig()
	if err != nil {
		return errors.Wrap(err, errKubeConfig)
	}

	// NOTE(phisco): We used to get them set as part of
	// https://github.com/kubernetes-sigs/controller-runtime/blob/2e9781e9fc6054387cf0901c70db56f0b0a63083/pkg/client/config/config.go#L96,
	// this new approach doesn't set them, so we need to set them here to avoid
	// being utterly slow.
	// TODO(phisco): make this configurable.
	if kubeconfig.QPS == 0 {
		kubeconfig.QPS = 20
	}

	if kubeconfig.Burst == 0 {
		kubeconfig.Burst = 30
	}

	logger.Debug("Found kubeconfig")

	client, err := client.NewWithWatch(kubeconfig, client.Options{
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
	rmapper := restmapper.NewShortcutExpander(restmapper.NewDeferredDiscoveryRESTMapper(d), d, nil)

	res, name, err := c.getResourceAndName()
	if err != nil {
		return errors.Wrap(err, errInvalidResourceAndName)
	}

	mapping, err := internal.MappingFor(rmapper, res)
	if err != nil {
		return errors.Wrap(err, errGetMapping)
	}

	// Get Resource object. Contains k8s resource and all its children, also as Resource.
	rootRef := &v1.ObjectReference{
		Kind:       mapping.GroupVersionKind.Kind,
		APIVersion: mapping.GroupVersionKind.GroupVersion().String(),
		Name:       name,
	}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		namespace := c.Namespace
		if namespace == "" {
			namespace, _, err = clientconfig.Namespace()
			if err != nil {
				return errors.Wrap(err, errKubeNamespace)
			}
		}

		logger.Debug("Requested resource is namespaced", "namespace", namespace)
		rootRef.Namespace = namespace
	}

	// If no name is provided, we should print a list of resources.
	shouldPrintAsList := name == ""

	logger.Debug("Getting resource tree", "rootRef", rootRef.String())
	var resourceList *resource.ResourceList
	if shouldPrintAsList {
		// If no name is provided, we list all resources of the kind.
		logger.Debug("No name provided, listing all resources of the kind")
		resourceList = resource.ListResources(ctx, client, rootRef)
	} else {
		// If a name is provided, we get the specific resource.
		logger.Debug("Name provided, getting specific resource", "name", name)
		res := resource.GetResource(ctx, client, rootRef)
		resourceList = &resource.ResourceList{
			Items: []*resource.Resource{res},
			Error: res.Error,
		}
	}

	// We should just surface any error getting the root resource immediately.
	if err := resourceList.Error; err != nil {
		return errors.Wrap(err, errGetResource)
	}

	for i := range resourceList.Items {
		root := resourceList.Items[i]
		root, err = c.getResourceTree(ctx, root, mapping, client, logger)
		if err != nil {
			logger.Debug(errGetResource, "error", err)
			return errors.Wrap(err, errGetResource)
		}

		logger.Debug("Got resource tree", "root", root)

		resourceList.Items[i] = root
	}

	// Watch mode for a single resource
	if c.Watch && !shouldPrintAsList && len(resourceList.Items) > 0 {
		root := resourceList.Items[0]
		return c.watchResourceTree(ctx, k, logger, client, root, mapping, p)
	}

	if shouldPrintAsList {
		// Print list of resources
		err = p.PrintList(k.Stdout, resourceList)
	} else {
		// Print a single resource
		err = p.Print(k.Stdout, resourceList.Items[0])
	}

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

func (c *Cmd) getResourceTree(ctx context.Context, root *resource.Resource, mapping *meta.RESTMapping, client client.Client, logger logging.Logger) (*resource.Resource, error) {
	var treeClient resource.TreeClient
	var err error
	switch {
	case xpkg.IsPackageType(mapping.GroupVersionKind.GroupKind()):
		logger.Debug("Requested resource is a Package")
		treeClient, err = xpkg.NewClient(client,
			xpkg.WithDependencyOutput(xpkg.DependencyOutput(c.ShowPackageDependencies)),
			xpkg.WithPackageRuntimeConfigs(c.ShowPackageRuntimeConfigs),
			xpkg.WithRevisionOutput(xpkg.RevisionOutput(c.ShowPackageRevisions)))
		if err != nil {
			return nil, errors.Wrap(err, errInitKubeClient)
		}
	default:
		logger.Debug("Requested resource is not a package, assumed to be an XR, XRC or MR")
		treeClient, err = xrm.NewClient(client,
			xrm.WithConnectionSecrets(c.ShowConnectionSecrets),
			xrm.WithConcurrency(c.Concurrency),
		)
		if err != nil {
			return nil, errors.Wrap(err, errInitKubeClient)
		}
	}
	logger.Debug("Built client")

	return treeClient.GetResourceTree(ctx, root)
}
