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

type Client struct {
	dclient   *dynamic.DynamicClient
	clientset *kubernetes.Clientset
	rmapper   meta.RESTMapper
	dc        *discovery.DiscoveryClient
}

type Resource struct {
	Manifest           *unstructured.Unstructured
	Children           []*Resource
	LatestEventMessage string
}

// Returns resource kind as string
func (r *Resource) GetKind() string {
	return r.Manifest.GetKind()
}

// Returns resource name as string
func (r *Resource) GetName() string {
	return r.Manifest.GetName()
}

// Returns resource namespace as string
func (r *Resource) GetNamespace() string {
	return r.Manifest.GetNamespace()
}

// Returns resource apiversion as string
func (r *Resource) GetApiVersion() string {
	return r.Manifest.GetAPIVersion()
}

// This function takes a certain conditionType as input e.g. "Ready" or "Synced"
// Returns the Status of the map with the conditionType as string
func (r *Resource) GetConditionStatus(conditionKey string) string {
	conditions, _, _ := unstructured.NestedSlice(r.Manifest.Object, "status", "conditions")
	for _, condition := range conditions {
		conditionMap, _ := condition.(map[string]interface{})
		conditionType, _ := conditionMap["type"].(string)
		conditionStatus, _ := conditionMap["status"].(string)

		if conditionType == conditionKey {
			return conditionStatus
		}
	}
	return ""
}

// Returns the message as string if set under `status.conditions` in the manifest. Else return empty string
func (r *Resource) GetConditionMessage() string {
	conditions, _, _ := unstructured.NestedSlice(r.Manifest.Object, "status", "conditions")

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

// Returns the latest event of the resource as string
func (r *Resource) GetEvent() string {
	return r.LatestEventMessage
}

// Returns true if the Resource has children set.
func (r *Resource) HasChildren() bool {
	return len(r.Children) > 0
}

// The main function of resource. Returns a Resource and all its child resources.
func GetResource(resourceKind string, resourceName string, namespace string, kubeconfig *rest.Config) (*Resource, error) {
	// Init new client
	client, err := newClient(kubeconfig)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't init kubeclient")
	}

	// Get manifest for root resource
	root := Resource{}
	root.Manifest, err = client.getManifest(resourceKind, resourceName, "", namespace)
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
	if resourceRefMap, found, err := getStringMapFromNestedField(*r.Manifest, "spec", "resourceRef"); found && err == nil {
		r, err = kc.setChild(resourceRefMap, r)
	} else if resourceRefs, found, err := getSliceOfMapsFromNestedField(*r.Manifest, "spec", "resourceRefs"); found && err == nil {
		for _, resourceRefMap := range resourceRefs {
			r, err = kc.setChild(resourceRefMap, r)
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

	// Get manifest. Assumes children is in same namespace as claim if resouce is namespaced.
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
		Manifest:           u,
		LatestEventMessage: event,
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
// E.g both Azure and AWS provide a "group" resouce. So the function is not able to identify for which resource kind the namespace is checked and chooses the first match.
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

// The event function returns the latest occuring event of a resource.
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
