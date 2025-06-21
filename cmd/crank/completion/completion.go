// Package completion contains Crossplane CLI completions.
package completion

import (
	"context"
	"fmt"
	"strings"

	"github.com/posener/complete"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane/cmd/crank/internal"
)

// Predictors returns all supported predictors.
func Predictors() map[string]complete.Predictor {
	yamlPredictor := complete.PredictOr(
		complete.PredictFiles("*.yml"),
		complete.PredictFiles("*.yaml"),
	)
	return map[string]complete.Predictor{
		"file":                   complete.PredictFiles("*"),
		"xpkg_file":              complete.PredictFiles("*.xpkg"),
		"yaml_file":              yamlPredictor,
		"directory":              complete.PredictDirs("*"),
		"file_or_directory":      complete.PredictOr(complete.PredictFiles("*"), complete.PredictDirs("*")),
		"yaml_file_or_directory": complete.PredictOr(yamlPredictor, complete.PredictDirs("*")),
		"namespace":              namespacePredictor(),
		"context":                contextPredictor(),
		"k8s_resource":           kubernetesResourcePredictor(),
		"k8s_resource_name":      kubernetesResourceNamePredictor(),
	}
}

// kubernetesResourcePredictor returns a predictor that suggests Kubernetes resources based on
// the current context and namespace or the context and namespace specified in the command line arguments.
// It uses the Kubernetes client to retrieve the available resources and filters them based on the
// last completed argument.
func kubernetesResourcePredictor() complete.PredictFunc {
	return func(a complete.Args) []string {
		_, kubeconfig, _, err := kubernetesClient(parseConfigOverride(a))
		if err != nil {
			return nil
		}

		discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
		if err != nil {
			return nil
		}

		resources, err := discoveryClient.ServerPreferredResources()
		if err != nil {
			return nil
		}

		if len(resources) == 0 {
			return nil
		}

		var predictions []string
		for _, apiResources := range resources {
			for _, res := range apiResources.APIResources {
				var resourceName string

				// Write the resource name in a normalized format <name>.<version>.<group>
				// or <name>.<version> if the group is empty.
				// If the version is empty, just use the name.
				switch {
				case res.Group != "" && res.Version != "":
					resourceName = fmt.Sprintf("%s.%s.%s", res.Name, res.Version, res.Group)
				case res.Version != "":
					resourceName = fmt.Sprintf("%s.%s", res.Name, res.Version)
				default:
					resourceName = res.Name
				}

				// This way we can filter the resources by the last completed argument of any valid format
				// by just checking the prefix.
				if strings.HasPrefix(resourceName, a.Last) {
					predictions = append(predictions, resourceName)
				}
			}
		}
		return predictions
	}
}

// kubernetesResourceNamePredictor returns a predictor that suggests Kubernetes resource names based on
// the current context and namespace or the context and namespace specified in the command line arguments.
// It uses the Kubernetes client to retrieve the available resources and filters them based on the
// last completed argument.
func kubernetesResourceNamePredictor() complete.PredictFunc {
	return func(a complete.Args) []string {
		client, kubeconfig, clientconfig, err := kubernetesClient(parseConfigOverride(a))
		if err != nil {
			return nil
		}

		discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
		if err != nil {
			return nil
		}

		d := memory.NewMemCacheClient(discoveryClient)
		// If the previously completed argument (resource name) was used by its short form, we need to
		// get the full resource name to be able to list the resources.
		rmapper := restmapper.NewShortcutExpander(restmapper.NewDeferredDiscoveryRESTMapper(d), d, nil)
		mapping, err := internal.MappingFor(rmapper, a.LastCompleted)
		if err != nil {
			return nil
		}

		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    mapping.GroupVersionKind.Kind,
			Group:   mapping.GroupVersionKind.Group,
			Version: mapping.GroupVersionKind.Version,
		})

		// Limit the search results to the current context and namespace or the context and namespace specified in the command line arguments.
		// If no namespace is specified, it will try to use the current context namespace.
		// If that fails, it will use the default namespace.
		namespace := parseNamespaceOverride(a)
		if namespace == "" {
			namespace, _, err = clientconfig.Namespace()
			if err != nil || namespace == "" {
				namespace = metav1.NamespaceDefault
			}
		}
		err = client.List(context.Background(), u, controllerClient.InNamespace(namespace))
		if err != nil {
			return nil
		}

		// Find predictions by filtering the resource names that start with the currently completed argument.
		var predictions []string
		for _, res := range u.Items {
			if strings.HasPrefix(res.GetName(), a.Last) {
				predictions = append(predictions, res.GetName())
			}
		}
		return predictions
	}
}

// contextPredictor returns a predictor that suggests Kubernetes contexts from the KUBECONFIG.
func contextPredictor() complete.PredictFunc {
	return func(a complete.Args) []string {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		kubeConfig, err := clientConfig.RawConfig()
		if err != nil {
			return nil
		}

		var predictions []string
		for name := range kubeConfig.Contexts {
			if strings.HasPrefix(name, a.Last) {
				predictions = append(predictions, name)
			}
		}
		return predictions
	}
}

// namespacePredictor returns a predictor that suggests Kubernetes namespaces from the current context.
// It uses the Kubernetes client to retrieve the available namespaces and filters them based on the
// last completed argument.
func namespacePredictor() complete.PredictFunc {
	return func(a complete.Args) []string {
		client, err := kubernetesClientset()
		if err != nil {
			return nil
		}

		namespaceList, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil
		}

		var predictions []string
		for _, ns := range namespaceList.Items {
			if strings.HasPrefix(ns.GetName(), a.Last) {
				predictions = append(predictions, ns.GetName())
			}
		}
		return predictions
	}
}

// kubernetesClientset returns a Kubernetes clientset using the default kubeconfig.
func kubernetesClientset() (*kubernetes.Clientset, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	kubeConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(kubeConfig)
}

// kubernetesClient returns a Kubernetes client and a rest.Config using the provided config overrides.
func kubernetesClient(configOverrides *clientcmd.ConfigOverrides) (controllerClient.Client, *rest.Config, clientcmd.ClientConfig, error) {
	clientconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		configOverrides,
	)

	kubeconfig, err := clientconfig.ClientConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	client, err := controllerClient.New(rest.CopyConfig(kubeconfig), controllerClient.Options{})
	if err != nil {
		return nil, nil, nil, err
	}

	return client, rest.CopyConfig(kubeconfig), clientconfig, nil
}

// parseConfigOverride parses ConfigOverrides for the k8s client from the completed command line arguments.
func parseConfigOverride(a complete.Args) *clientcmd.ConfigOverrides {
	context := ""
	for i, arg := range a.All {
		if (arg == "--context" || arg == "-c") && i < len(a.All) {
			context = a.All[i+1]
			break
		}
	}
	return &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}
}

// parseNamespaceOverride parses the namespace override from the completed command line arguments.
func parseNamespaceOverride(a complete.Args) string {
	namespace := ""
	for i, arg := range a.All {
		if (arg == "--namespace" || arg == "-n") && i < len(a.All) {
			namespace = a.All[i+1]
			break
		}
	}
	return namespace
}
