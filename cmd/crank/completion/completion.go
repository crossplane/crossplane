// Package completion contains Crossplane CLI completions.
package completion

import (
	"context"
	"fmt"
	"strings"

	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errFmtResourceTypeNotFound = "the server doesn't have a resource type %q"
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
	return func(a complete.Args) (prediction []string) {
		_, kubeconfig, err := kubernetesClient()
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
				// by just cheking the prefix.
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
	return func(a complete.Args) (prediction []string) {
		client, kubeconfig, err := kubernetesClient()
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
		mapping, err := mappingFor(rmapper, a.LastCompleted)
		if err != nil {
			return nil
		}

		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    mapping.GroupVersionKind.Kind,
			Group:   mapping.GroupVersionKind.Group,
			Version: mapping.GroupVersionKind.Version,
		})
		err = client.List(context.Background(), u)
		if err != nil {
			return nil
		}

		var predictions []string
		for _, res := range u.Items {
			if strings.HasPrefix(res.GetName(), a.Last) {
				predictions = append(predictions, res.GetName())
			}
		}
		return predictions
	}
}

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

		namespaceList, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
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

func kubernetesClient() (client.Client, *rest.Config, error) {
	// TODO: It's possible to specify context overrides using command line params. We could also try to read those.
	clientconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	kubeconfig, err := clientconfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := client.New(rest.CopyConfig(kubeconfig), client.Options{})
	if err != nil {
		return nil, nil, err
	}

	return client, rest.CopyConfig(kubeconfig), nil
}

// Copied over from cli-runtime pkg/resource Builder,
// https://github.com/kubernetes/cli-runtime/blob/9a91d944dd43186c52e0162e12b151b0e460354a/pkg/resource/builder.go#L768
func mappingFor(rmapper meta.RESTMapper, resourceOrKindArg string) (*meta.RESTMapping, error) {
	// TODO(phisco): actually use the Builder.
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}
	if fullySpecifiedGVR != nil {
		gvk, _ = rmapper.KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = rmapper.KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		return rmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}
	if !fullySpecifiedGVK.Empty() {
		if mapping, err := rmapper.RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}
	mapping, err := rmapper.RESTMapping(groupKind, gvk.Version)
	if err != nil {
		// if we error out here, it is because we could not match a resource or a kind
		// for the given argument. To maintain consistency with previous behavior,
		// announce that a resource type could not be found.
		// if the error is _not_ a *meta.NoKindMatchError, then we had trouble doing discovery,
		// so we should return the original error since it may help a user diagnose what is actually wrong
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf(errFmtResourceTypeNotFound, groupResource.Resource)
		}
		return nil, err
	}
	return mapping, nil
}
