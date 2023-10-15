// Package k8s implements a Resource struct, that represents a Crossplane k8s resource (usually a claim or composite resource).
// The package also contains helper methods and functions for the Resource struct.
package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/homedir"
)

// Client struct contains the following k8s client types:
// dynamic, discovery, static and a restmapper
type Client struct {
	dclient   *dynamic.DynamicClient
	clientset *kubernetes.Clientset
	rmapper   meta.RESTMapper
	dc        *discovery.DiscoveryClient
}

// Resource struct represents a Crossplane k8s resource.
type Resource struct {
	manifest           *unstructured.Unstructured
	Children           []*Resource
	latestEventMessage string
}

// GetKind returns resource kind as string
func (r *Resource) GetKind() string {
	return r.manifest.GetKind()
}

// GetName returns resource name as string
func (r *Resource) GetName() string {
	return r.manifest.GetName()
}

// GetNamespace returns resource namespace as string
func (r *Resource) GetNamespace() string {
	return r.manifest.GetNamespace()
}

// GetAPIVersion returns resource apiversion as string
func (r *Resource) GetAPIVersion() string {
	return r.manifest.GetAPIVersion()
}

// GetConditionStatus returns the Status of the map with the conditionType as string
// This function takes a certain conditionType as input e.g. "Ready" or "Synced"
func (r *Resource) GetConditionStatus(conditionKey string) string {
	conditions, _, _ := unstructured.NestedSlice(r.manifest.Object, "status", "conditions")
	for _, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}
		conditionType, ok := conditionMap["type"].(string)
		if !ok {
			continue
		}
		conditionStatus, ok := conditionMap["status"].(string)
		if !ok {
			continue
		}

		if conditionType == conditionKey {
			return conditionStatus
		}
	}
	return ""
}

// GetConditionMessage returns the message as string if set under `status.conditions` in the manifest. Else return empty string
func (r *Resource) GetConditionMessage() string {
	conditions, _, _ := unstructured.NestedSlice(r.manifest.Object, "status", "conditions")

	for _, item := range conditions {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if message, exists := itemMap["message"]; exists {
				if messageStr, ok := message.(string); ok {
					return messageStr
				}
			}
		}
	}

	return ""
}

// GetEvent returns the latest event of the resource as string
func (r *Resource) GetEvent() string {
	return r.latestEventMessage
}

// HasChildren returns true if the Resource has children set.
func (r *Resource) HasChildren() bool {
	return len(r.Children) > 0
}

// GetResource returns a Resource and all its child resources. The main function of resource.
func GetResource(resourceKind string, resourceName string, namespace string, kubeconfig *rest.Config) (*Resource, error) {
	// Init new client
	client, err := newClient(kubeconfig)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't init kubeclient")
	}

	// Get manifest for root resource
	root := Resource{}
	root.manifest, err = client.getManifest(resourceKind, resourceName, "", namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't get root resource manifest")
	}

	// Get all children for root resource by checking resourceRef(s) in manifest
	root, err = client.getChildren(root)
	if err != nil {
		return &root, errors.Wrap(err, "Couldn't get children of root resource")
	}

	return &root, nil
}

// getManifest returns the k8s manifest of a resource as unstructured.
func (kc *Client) getManifest(resourceKind string, resourceName string, apiVersion string, namespace string) (*unstructured.Unstructured, error) {
	// Set GVK for resource in new manifest
	gr := schema.ParseGroupResource(resourceKind)

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
		return nil, errors.Wrap(err, "Couldn't detect if resource is namespaced")
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
		return nil, errors.Wrap(err, "Couldn't build GVR schema for resource")
	}

	// Get manifest for resource
	result, err := kc.dclient.Resource(gvr).Namespace(manifest.GetNamespace()).Get(context.TODO(), manifest.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't get resource manifest from KubeAPI")
	}

	return result, nil
}

// The function checks the `spec.resourceRef` and `spec.resourceRefs` path for child resources.
// If resources are discovered they are added as getChildren to the passed r Resource.
func (kc *Client) getChildren(r Resource) (Resource, error) {
	// Check both singular and plural for spec.resourceRef(s)
	if resourceRefMap, found, err := getStringMapFromNestedField(*r.manifest, "spec", "resourceRef"); found && err == nil {
		r, err = kc.setChild(resourceRefMap, r)
		if err != nil {
			return r, err
		}
	} else if resourceRefs, found, err := getSliceOfMapsFromNestedField(*r.manifest, "spec", "resourceRefs"); found && err == nil {
		for _, resourceRefMap := range resourceRefs {
			r, err = kc.setChild(resourceRefMap, r)
			if err != nil {
				return r, err
			}
		}
	} else if err != nil {
		return r, errors.Wrap(err, "Couldn't get children of resource")
	}

	return r, nil
}

// The setChild function is a helper for the getChildren function.
// It calls the getManifest function and then adds the children to the list of children.
// It returns the r Resource that was passed to it, containing the children that was set during this function call.
func (kc *Client) setChild(resourceRefMap map[string]string, r Resource) (Resource, error) {
	// Get info about child
	name := resourceRefMap["name"]
	kind := resourceRefMap["kind"]
	apiVersion := resourceRefMap["apiVersion"]

	// Get manifest. Assumes children is in same namespace as claim if resource is namespaced.
	// TODO: Not sure if namespace is set in namespaced resources in `spec.resourceRef(s)`
	u, err := kc.getManifest(kind, name, apiVersion, r.GetNamespace())
	if err != nil {
		return r, errors.Wrap(err, "Couldn't get manifest of children")
	}

	// Get event
	event, err := kc.event(name, kind, apiVersion, r.GetNamespace())
	if err != nil {
		return r, errors.Wrap(err, "Couldn't get event for resource")
	}
	// Set child
	child := Resource{
		manifest:           u,
		latestEventMessage: event,
	}
	// Get children of children
	child, err = kc.getChildren(child)
	if err != nil {
		return r, errors.Wrap(err, "Couldn't get children of children")
	}
	r.Children = append(r.Children, &child)

	return r, nil
}

// The isResourceNamespaced function returns true if the passed resource is namespaced, else false.
// The functions works by getting all k8s API resources and then checking for the specific resourceKind and apiVersion passed.
// If an empty apiVersion string is passed the function also works. In that case issues may occur in case some kind exists more then once.
// E.g both Azure and AWS provide a "group" resource. So the function is not able to identify for which resource kind the namespace is checked and chooses the first match.
func (kc *Client) isResourceNamespaced(resourceKind string, apiVersion string) (bool, error) {
	// Retrieve the API resource list
	apiResourceLists, err := kc.dc.ServerPreferredResources()
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get API resources of k8s API server")
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
	return false, errors.Wrap(err, "resource not found in API server")
}

// The event function returns the latest occurring event of a resource.
func (kc *Client) event(resourceName string, resourceKind string, apiVersion string, namespace string) (string, error) {
	// List events for the resource.
	eventList, err := kc.clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s,involvedObject.apiVersion=%s", resourceName, resourceKind, apiVersion),
	})
	if err != nil {
		return "", errors.Wrap(err, "Couldn't get event list for resource")
	}

	// Check if there are any events.
	if len(eventList.Items) == 0 {
		return "", nil
	}

	// Get the latest event.
	latestEvent := eventList.Items[0]
	return latestEvent.Message, nil
}

// The newClient function initializes and returns a Client struct
func newClient(kubeconfig *rest.Config) (*Client, error) {

	// Use to get custom resources
	dclient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	// Use to discover API resources
	dc, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	// Use to get events
	clientset, _ := kubernetes.NewForConfig(kubeconfig)

	discoveryCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery")
	httpCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
	discoveryClient, err := disk.NewCachedDiscoveryClientForConfig(
		kubeconfig,
		discoveryCacheDir,
		httpCacheDir,
		10*time.Minute)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	rMapper := restmapper.NewShortcutExpander(mapper, discoveryClient)

	return &Client{
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
