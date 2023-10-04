package k8s_resource

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type KubeClient struct {
	dclient   *dynamic.DynamicClient
	clientset *kubernetes.Clientset
	rmapper   meta.RESTMapper
	dc        *discovery.DiscoveryClient
}

// GetResource takes a the kind, name, namespace of a resource and a kubeconfig as input.
// The function then returns a type Resource struct, containing itself and all its children as Resource.
func GetResource(resourceKind string, resourceName string, namespace string, kubeconfig string) (*Resource, error) {
	kubeClient, err := newKubeClient(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Couldn't init kubeclient -> %w", err)
	}

	// Set manifest for root resource
	root := Resource{}
	root.manifest, err = kubeClient.getManifest(resourceKind, resourceName, "", namespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get root resource manifest -> %w", err)
	}

	// Get all children for root resource by checking resourceRef(s) in manifest
	root, err = kubeClient.getChildren(root)
	if err != nil {
		return &root, fmt.Errorf("Couldn't get children of root resource -> %w", err)
	}

	return &root, nil
}

// getManifest returns the k8s manifest of a resource as unstructured.
func (kc *KubeClient) getManifest(resourceKind string, resourceName string, apiVersion string, namespace string) (*unstructured.Unstructured, error) {
	gr := schema.ParseGroupResource(resourceKind)

	// Set GVK for resource in new manifest
	manifest := &unstructured.Unstructured{}
	manifest.SetName(resourceName)
	manifest.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gr.Group,
		Version: apiVersion,
		Kind:    gr.Resource,
	})

	// Check if resource is namespaced as the namespace parameter has to bet set in the kc.client.Resource() call below
	isNamespaced, err := kc.isResourceNamespaced(gr.Resource, apiVersion)
	if err != nil {
		return nil, fmt.Errorf("Couldn't detect if resource is namespaced -> %w", err)
	}
	if isNamespaced {
		manifest.SetNamespace(namespace)
	}

	// Built GVR schema for API server call below.
	gvr, err := kc.rmapper.ResourceFor(schema.GroupVersionResource{
		Group:    manifest.GroupVersionKind().Group,
		Version:  manifest.GroupVersionKind().Version,
		Resource: manifest.GetKind(),
	})
	if err != nil {
		return nil, fmt.Errorf("Couldn't build GVR schema for resource -> %w", err)
	}

	// Get manifest for resource
	result, err := kc.dclient.Resource(gvr).Namespace(manifest.GetNamespace()).Get(context.TODO(), manifest.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't get resource manifest from KubeAPI -> %w", err)
	}

	return result, nil
}

// The getChildren function returns the r Resource that is passed to it on function call.
// The function checks the `spec.resourceRef` and `spec.resourceRefs` path for child resources.
// If resources are discovered they are added as children to the passed r Resource.
func (kc *KubeClient) getChildren(r Resource) (Resource, error) {
	// Check both singular and plural for spec.resourceRef(s)
	if resourceRefMap, found, err := getStringMapFromNestedField(*r.manifest, "spec", "resourceRef"); found && err == nil {
		r, err = kc.setChildren(resourceRefMap, r)
	} else if resourceRefs, found, err := getSliceOfMapsFromNestedField(*r.manifest, "spec", "resourceRefs"); found && err == nil {
		for _, resourceRefMap := range resourceRefs {
			r, err = kc.setChildren(resourceRefMap, r)
		}
	} else if err != nil {
		return r, fmt.Errorf("Couldn't get children of resource -> %w", err)
	}

	return r, nil
}

// The setChildren function is a helper for the getChildren function.
// It calls the getManifest function and then adds the children to the list of children.
// It returns the r Resource that was passed to it, containing the children that was set during this function call.
func (kc *KubeClient) setChildren(resourceRefMap map[string]string, r Resource) (Resource, error) {
	// Get info about child
	name := resourceRefMap["name"]
	kind := resourceRefMap["kind"]
	apiVersion := resourceRefMap["apiVersion"]

	// Get manifest. Assumes children is in same namespace as claim if resouce is namespaced.
	// TODO: Not sure if namespace is set in namespaced resources in `spec.resourceRef(s)`
	u, err := kc.getManifest(kind, name, apiVersion, r.GetNamespace())
	if err != nil {
		return r, fmt.Errorf("Couldn't get manifest of children -> %w", err)
	}

	// Get event
	event, err := kc.getEvent(name, kind, apiVersion, r.GetNamespace())
	if err != nil {
		return r, fmt.Errorf("Couldn't get event for resource %s -> %w", name+kind, err)
	}
	// Set child
	child := Resource{
		manifest: u,
		event:    event,
	}
	// Get children of children
	child, err = kc.getChildren(child)
	if err != nil {
		return r, fmt.Errorf("Couldn't get children of children -> %w", err)
	}
	r.children = append(r.children, child)

	return r, nil
}

// The isResourceNamespaced function returns true is passed resource is namespaced, else false.
// The functions works by getting all k8s API resources and then checking for the specific resourceKind and apiVersion passed.
// Once a match is found it is checked if the resource is namespaced.
// If an empty apiVersion string is passed the function also works. But issues may occur in case some kind exists more then once.
// E.g both Azure and AWS provide a group resouce. So the function is not able to identify for which resource kind the namespace is checked and chooses the first match.
func (kc *KubeClient) isResourceNamespaced(resourceKind string, apiVersion string) (bool, error) {
	// Retrieve the API resource list
	apiResourceLists, err := kc.dc.ServerPreferredResources()
	if err != nil {
		return false, fmt.Errorf("Couldn't get API resources of k8s API server -> %w", err)
	}

	// Trim version if set
	apiVersion = strings.Split(apiVersion, "/")[0]

	// Find kind and apiVersion (if set) in the resource list
	for _, apiResourceList := range apiResourceLists {
		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Group == apiVersion || apiVersion == "" {
				resourceKind = strings.ToLower(resourceKind)
				if apiResource.Name == resourceKind || apiResource.SingularName == resourceKind {
					return apiResource.Namespaced, nil
				}
			}

		}
	}
	return false, fmt.Errorf("resource not found in API server -> Kind:%s ApiVersion %s", resourceKind, apiVersion)
}

// The getEvent function returns the latest occuring event of a resource.
func (kc *KubeClient) getEvent(resourceName string, resourceKind string, apiVersion string, namespace string) (string, error) {
	// List events for the resource.
	eventList, err := kc.clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s,involvedObject.apiVersion=%s", resourceName, resourceKind, apiVersion),
	})
	if err != nil {
		return "", fmt.Errorf("Couldn't get event list for resource %s -> %w", resourceKind+resourceName, err)
	}

	// Check if there are any events.
	if len(eventList.Items) == 0 {
		return "", nil
	}

	// Get the latest event.
	latestEvent := eventList.Items[0]
	return latestEvent.Message, nil
}

// The newKubeClient function returns a KubeClient struct which consists of 3 client types.
// The dynamic client dclient, the "regular" k8s client clientset, and the discoveryClient dc
// The rmapper can be used to set the GVR of a resource.
func newKubeClient(kubeconfig string) (*KubeClient, error) {
	// Initialize a Kubernetes client.
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	// Use to get custom resources
	dclient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Use to discover API resources
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	// Use to get events
	clientset, _ := kubernetes.NewForConfig(config)

	discoveryCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery")
	httpCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
	discoveryClient, err := disk.NewCachedDiscoveryClientForConfig(
		config,
		discoveryCacheDir,
		httpCacheDir,
		10*time.Minute)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	rMapper := restmapper.NewShortcutExpander(mapper, discoveryClient)

	return &KubeClient{
		dclient:   dclient,
		clientset: clientset,
		rmapper:   rMapper,
		dc:        dc,
	}, nil
}

// This is a helper function for getChildren()
// It returns a map which should consist of the keys "name", "kind", and "apiversion"
func getStringMapFromNestedField(obj unstructured.Unstructured, fields ...string) (map[string]string, bool, error) {
	nestedField, found, err := unstructured.NestedStringMap(obj.Object, fields...)
	if !found || err != nil {
		return nil, false, err
	}

	result := make(map[string]string)
	for key, value := range nestedField {
		result[key] = value
	}

	return result, true, nil
}

// This is a helper function for getChildren()
// It returns a list of maps which should consist of the keys "name", "kind", and "apiversion"
func getSliceOfMapsFromNestedField(obj unstructured.Unstructured, fields ...string) ([]map[string]string, bool, error) {
	nestedField, found, err := unstructured.NestedFieldNoCopy(obj.Object, fields...)
	if !found || err != nil {
		return nil, false, err
	}

	var result []map[string]string
	if slice, ok := nestedField.([]interface{}); ok {
		for _, item := range slice {
			if m, ok := item.(map[string]interface{}); ok {
				stringMap := make(map[string]string)
				for key, value := range m {
					if str, ok := value.(string); ok {
						stringMap[key] = str
					}
				}
				result = append(result, stringMap)
			}
		}
	}

	return result, true, nil
}
