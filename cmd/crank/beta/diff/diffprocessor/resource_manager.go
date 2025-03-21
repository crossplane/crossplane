package diffprocessor

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// ResourceManager handles resource-related operations like fetching, updating owner refs,
// and identifying resources to be removed.
type ResourceManager interface {
	// FetchCurrentObject retrieves the current state of an object from the cluster
	FetchCurrentObject(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error)

	// UpdateOwnerRefs ensures all OwnerReferences have valid UIDs
	UpdateOwnerRefs(parent *unstructured.Unstructured, child *unstructured.Unstructured)

	// FindResourcesToBeRemoved identifies resources that exist in the current state but are not in the processed list
	FindResourcesToBeRemoved(ctx context.Context, composite string, processedResources map[string]bool) ([]*unstructured.Unstructured, error)
}

// DefaultResourceManager implements ResourceManager interface
type DefaultResourceManager struct {
	client cc.ClusterClient
	logger logging.Logger
}

// NewResourceManager creates a new DefaultResourceManager
func NewResourceManager(client cc.ClusterClient, logger logging.Logger) ResourceManager {
	return &DefaultResourceManager{
		client: client,
		logger: logger,
	}
}

// FetchCurrentObject retrieves the current state of the object from the cluster
// It returns the current object, a boolean indicating if it's a new object, and any error
func (m *DefaultResourceManager) FetchCurrentObject(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	// Get the GroupVersionKind and name/namespace for lookup
	gvk := desired.GroupVersionKind()
	name := desired.GetName()
	generateName := desired.GetGenerateName()
	namespace := desired.GetNamespace()

	// For logging - create a resource ID that might use generateName
	var resourceID string
	if name != "" {
		resourceID = fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
	} else if generateName != "" {
		resourceID = fmt.Sprintf("%s/%s/%s*", gvk.String(), namespace, generateName)
	} else {
		resourceID = fmt.Sprintf("%s/<no-name>", gvk.String())
	}

	m.logger.Debug("Fetching current object state",
		"resource", resourceID,
		"hasName", name != "",
		"hasGenerateName", generateName != "")

	var current *unstructured.Unstructured
	var err error
	isNewObject := false

	// If there's a name, try direct lookup first
	if name != "" {
		current, err = m.client.GetResource(ctx, gvk, namespace, name)
		if err == nil && current != nil {
			m.logger.Debug("Found resource by direct lookup",
				"resource", resourceID,
				"resourceVersion", current.GetResourceVersion())

			// Check if this resource is already owned by a different composite
			if composite != nil {
				if labels := current.GetLabels(); labels != nil {
					if owner, exists := labels["crossplane.io/composite"]; exists && owner != composite.GetName() {
						// Log a warning if the resource is owned by a different composite
						m.logger.Info(
							"Warning: Resource already belongs to another composite",
							"resource", resourceID,
							"currentOwner", owner,
							"newOwner", composite.GetName(),
						)
					}
				}
			}
			return current, false, nil
		}
	}

	// Handle the resource not found case or resources with only generateName
	// This might be a genuinely new resource or one we need to look up differently
	if name == "" || apierrors.IsNotFound(err) {
		// If this is the XR itself (composite is nil), it's genuinely new
		if composite == nil {
			m.logger.Debug("XR not found, creating new", "resource", resourceID)
			return nil, true, nil
		}

		// Check if we have annotations
		annotations := desired.GetAnnotations()
		if annotations == nil {
			m.logger.Debug("Resource not found and has no annotations, creating new",
				"resource", resourceID)
			return nil, true, nil
		}

		// Look for composition resource name annotation
		var compResourceName string
		var hasCompResourceName bool

		// First check standard annotation
		if value, exists := annotations["crossplane.io/composition-resource-name"]; exists {
			compResourceName = value
			hasCompResourceName = true
		}

		// Then check function-specific variations if not found
		if !hasCompResourceName {
			for key, value := range annotations {
				if strings.HasSuffix(key, "/composition-resource-name") {
					compResourceName = value
					hasCompResourceName = true
					break
				}
			}
		}

		// If we don't have a composition resource name, it's a new resource
		if !hasCompResourceName {
			m.logger.Debug("Resource not found and has no composition-resource-name, creating new",
				"resource", resourceID)
			return nil, true, nil
		}

		m.logger.Debug("Resource needs lookup by labels and annotations",
			"resource", resourceID,
			"compositeName", composite.GetName(),
			"compositionResourceName", compResourceName,
			"hasGenerateName", generateName != "")

		// Only proceed if we have necessary identifiers
		if composite.GetName() != "" {
			// Create a label selector to find resources managed by this composite
			labelSelector := metav1.LabelSelector{
				MatchLabels: map[string]string{
					"crossplane.io/composite": composite.GetName(),
				},
			}

			// Convert the GVK to GVR for the client call
			gvr := schema.GroupVersionResource{
				Group:    gvk.Group,
				Version:  gvk.Version,
				Resource: strings.ToLower(gvk.Kind) + "s", // Naive pluralization
			}

			// Handle special cases for some well-known types
			switch gvk.Kind {
			case "Ingress":
				gvr.Resource = "ingresses"
			case "Endpoints":
				gvr.Resource = "endpoints"
			case "ConfigMap":
				gvr.Resource = "configmaps"
				// Add other special cases as needed
			}

			// Look up resources with the composite label
			resources, err := m.client.GetResourcesByLabel(ctx, namespace, gvr, labelSelector)
			if err != nil {
				m.logger.Debug("Error looking up resources by label",
					"resource", resourceID,
					"composite", composite.GetName(),
					"error", err)
			} else if len(resources) > 0 {
				m.logger.Debug("Found potential matches by label",
					"resource", resourceID,
					"matchCount", len(resources))

				// Iterate through results to find one with matching composition-resource-name
				for _, res := range resources {
					resAnnotations := res.GetAnnotations()
					if resAnnotations == nil {
						continue
					}

					// Check both the standard annotation and function-specific variations
					resourceNameMatch := false
					for key, value := range resAnnotations {
						if (key == "crossplane.io/composition-resource-name" ||
							strings.HasSuffix(key, "/composition-resource-name")) &&
							value == compResourceName {
							resourceNameMatch = true
							break
						}
					}

					if resourceNameMatch {
						// If this resource has generateName and we found a match,
						// verify the match starts with our generateName prefix if provided
						if generateName != "" {
							resName := res.GetName()
							if !strings.HasPrefix(resName, generateName) {
								m.logger.Debug("Found resource with matching composition name but wrong generateName prefix",
									"expectedPrefix", generateName,
									"actualName", resName)
								continue
							}
						}

						m.logger.Debug("Found matching resource by composition-resource-name",
							"resource", fmt.Sprintf("%s/%s", res.GetKind(), res.GetName()),
							"annotation", compResourceName)
						return res, false, nil
					}
				}
			}
		}

		// We didn't find a matching resource using any strategy
		m.logger.Debug("No matching resource found by label and annotation",
			"resource", resourceID,
			"compResourceName", compResourceName)
		isNewObject = true
		err = nil // Clear the error since this is an expected condition
	}

	return nil, isNewObject, err
}

// UpdateOwnerRefs ensures all OwnerReferences have valid UIDs
func (m *DefaultResourceManager) UpdateOwnerRefs(parent *unstructured.Unstructured, child *unstructured.Unstructured) {
	// if there's no parent, we are the parent.
	if parent == nil {
		m.logger.Debug("No parent provided for owner references update")
		return
	}

	uid := parent.GetUID()
	m.logger.Debug("Updating owner references",
		"parentKind", parent.GetKind(),
		"parentName", parent.GetName(),
		"parentUID", uid,
		"childKind", child.GetKind(),
		"childName", child.GetName())

	// Get the current owner references
	refs := child.GetOwnerReferences()
	m.logger.Debug("Current owner references", "count", len(refs))

	// Create new slice to hold the updated references
	updatedRefs := make([]metav1.OwnerReference, 0, len(refs))

	// Set a valid UID for each reference
	for _, ref := range refs {
		originalUID := ref.UID

		// if there is an owner ref on the dependent that we are pretty sure comes from us,
		// point the UID to the parent.
		if ref.Name == parent.GetName() &&
			ref.APIVersion == parent.GetAPIVersion() &&
			ref.Kind == parent.GetKind() &&
			ref.UID == "" {
			ref.UID = uid
			m.logger.Debug("Updated matching owner reference with parent UID",
				"refName", ref.Name,
				"oldUID", originalUID,
				"newUID", ref.UID)
		}

		// if we have a non-matching owner ref don't use the parent UID.
		if ref.UID == "" {
			ref.UID = uuid.NewUUID()
			m.logger.Debug("Generated new random UID for owner reference",
				"refName", ref.Name,
				"oldUID", originalUID,
				"newUID", ref.UID)
		}

		updatedRefs = append(updatedRefs, ref)
	}

	// Update the object with the modified owner references
	child.SetOwnerReferences(updatedRefs)
	m.logger.Debug("Updated owner references",
		"newCount", len(updatedRefs))
}

// FindResourcesToBeRemoved identifies resources that exist in the current state but are not in the processed list
func (m *DefaultResourceManager) FindResourcesToBeRemoved(ctx context.Context, composite string, processedResources map[string]bool) ([]*unstructured.Unstructured, error) {
	// Find the XR
	xrRes, err := m.client.GetResource(ctx, schema.GroupVersionKind{
		Group:   "example.org", // This needs to be determined dynamically based on the XR
		Version: "v1alpha1",    // This needs to be determined dynamically
		Kind:    "XRKind",      // This needs to be determined dynamically
	}, "", composite)
	if err != nil {
		return nil, errors.Wrap(err, "cannot find composite resource")
	}

	// Get the resource tree
	resourceTree, err := m.client.GetResourceTree(ctx, xrRes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get resource tree")
	}

	// Find resources that weren't processed (meaning they would be removed)
	var toBeRemoved []*unstructured.Unstructured

	// Function to recursively traverse the tree and find composed resources
	var findComposedResources func(node *resource.Resource)
	findComposedResources = func(node *resource.Resource) {
		// Skip the root (XR) node
		if node.Unstructured.GetAnnotations()["crossplane.io/composition-resource-name"] != "" {
			key := resourceKey(&node.Unstructured)
			if !processedResources[key] {
				// This resource exists but wasn't in our desired resources - it will be removed
				toBeRemoved = append(toBeRemoved, &node.Unstructured)
			}
		}

		for _, child := range node.Children {
			findComposedResources(child)
		}
	}

	// Start the traversal from the root's children to skip the XR itself
	for _, child := range resourceTree.Children {
		findComposedResources(child)
	}

	return toBeRemoved, nil
}

// resourceKey generates a unique key for a resource based on GVK and name
func resourceKey(res *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s",
		res.GetAPIVersion(),
		res.GetKind(),
		res.GetName())
}
