package diffprocessor

import (
	"context"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
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

	// Create a resource ID for logging
	resourceID := m.createResourceID(gvk, namespace, name, generateName)

	m.logger.Debug("Fetching current object state",
		"resource", resourceID,
		"hasName", name != "",
		"hasGenerateName", generateName != "")

	// Try direct lookup by name if available
	if name != "" {
		current, err := m.client.GetResource(ctx, gvk, namespace, name)
		if err == nil && current != nil {
			m.logger.Debug("Found resource by direct lookup",
				"resource", resourceID,
				"resourceVersion", current.GetResourceVersion())

			m.checkCompositeOwnership(current, composite)
			return current, false, nil
		}

		// If it's not a NotFound error, propagate it
		if err != nil && !apierrors.IsNotFound(err) {
			m.logger.Debug("Error getting resource",
				"resource", resourceID,
				"error", err)
			return nil, false, err
		}
	}

	// If direct lookup failed, try looking up by labels and annotations
	if composite != nil {
		current, found, err := m.lookupByComposite(ctx, composite, desired)
		if err != nil {
			// For resources that primarily use generateName, errors in label-based lookup
			// should result in a new resource rather than an error.
			// This matches the original behavior.
			if generateName != "" {
				m.logger.Debug("Error during label-based lookup for resource with generateName (treating as new)",
					"resource", resourceID,
					"error", err)
				return nil, true, nil
			}

			// For direct name lookups, propagate the error
			m.logger.Debug("Error during label-based lookup",
				"resource", resourceID,
				"error", err)
			return nil, false, err
		}

		if found {
			return current, false, nil
		}
	}

	// We didn't find a matching resource using any strategy
	m.logger.Debug("No matching resource found", "resource", resourceID)
	return nil, true, nil
}

// createResourceID generates a resource ID string for logging purposes
func (m *DefaultResourceManager) createResourceID(gvk schema.GroupVersionKind, namespace, name, generateName string) string {
	// Handle case with a proper name
	if name != "" {
		if namespace != "" {
			return fmt.Sprintf("%s/%s/%s", gvk.String(), namespace, name)
		}
		return fmt.Sprintf("%s/%s", gvk.String(), name)
	}

	// Handle case with generateName
	if generateName != "" {
		if namespace != "" {
			return fmt.Sprintf("%s/%s/%s*", gvk.String(), namespace, generateName)
		}
		return fmt.Sprintf("%s/%s*", gvk.String(), generateName)
	}

	// Fallback case when neither name nor generateName is provided
	return fmt.Sprintf("%s/<no-name>", gvk.String())
}

// checkCompositeOwnership logs a warning if the resource is owned by a different composite
func (m *DefaultResourceManager) checkCompositeOwnership(current *unstructured.Unstructured, composite *unstructured.Unstructured) {
	if composite == nil {
		return
	}

	if labels := current.GetLabels(); labels != nil {
		if owner, exists := labels["crossplane.io/composite"]; exists && owner != composite.GetName() {
			// Log a warning if the resource is owned by a different composite
			m.logger.Info(
				// TODO:  should we fail by default here?  maybe require a --force flag to proceed?
				"Warning: Resource already belongs to another composite.  Applying this diff will assume ownership!",
				"resource", fmt.Sprintf("%s/%s", current.GetKind(), current.GetName()),
				"currentOwner", owner,
				"newOwner", composite.GetName(),
			)
		}
	}
}

// lookupByComposite attempts to find a resource by looking at composite ownership and composition resource name
func (m *DefaultResourceManager) lookupByComposite(ctx context.Context, composite *unstructured.Unstructured, desired *unstructured.Unstructured) (*unstructured.Unstructured, bool, error) {
	// Derive parameters from the provided arguments
	gvk := desired.GroupVersionKind()
	namespace := desired.GetNamespace()
	generateName := desired.GetGenerateName()
	resourceID := m.createResourceID(gvk, namespace, desired.GetName(), generateName)

	// Check if we have annotations
	annotations := desired.GetAnnotations()
	if annotations == nil {
		m.logger.Debug("Resource has no annotations, creating new",
			"resource", resourceID)
		return nil, false, nil
	}

	// Extract the composition resource name from annotations
	compResourceName := m.getCompositionResourceName(annotations)
	if compResourceName == "" {
		m.logger.Debug("Resource has no composition-resource-name, creating new",
			"resource", resourceID)
		return nil, false, nil
	}

	m.logger.Debug("Looking up resource by labels and annotations",
		"resource", resourceID,
		"compositeName", composite.GetName(),
		"compositionResourceName", compResourceName,
		"hasGenerateName", generateName != "")

	// Only proceed if we have a composite name
	if composite.GetName() == "" {
		return nil, false, nil
	}

	// Create a label selector to find resources managed by this composite
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			"crossplane.io/composite": composite.GetName(),
		},
	}

	// Look up resources with the composite label
	resources, err := m.client.GetResourcesByLabel(ctx, namespace, gvk, labelSelector)
	if err != nil {
		return nil, false, errors.Wrapf(err, "cannot list resources for composite %s", composite.GetName())
	}

	if len(resources) == 0 {
		m.logger.Debug("No resources found with composite owner label",
			"composite", composite.GetName())
		return nil, false, nil
	}

	m.logger.Debug("Found potential matches by label",
		"resource", resourceID,
		"matchCount", len(resources))

	// Find a resource with matching composition-resource-name
	return m.findMatchingResource(resources, compResourceName, generateName)
}

// getCompositionResourceName extracts the composition resource name from annotations
func (m *DefaultResourceManager) getCompositionResourceName(annotations map[string]string) string {
	// First check standard annotation
	if value, exists := annotations["crossplane.io/composition-resource-name"]; exists {
		return value
	}

	// Then check function-specific variations
	for key, value := range annotations {
		if strings.HasSuffix(key, "/composition-resource-name") {
			return value
		}
	}

	return ""
}

// findMatchingResource looks through resources to find one matching the composition resource name
func (m *DefaultResourceManager) findMatchingResource(
	resources []*unstructured.Unstructured,
	compResourceName string,
	generateName string,
) (*unstructured.Unstructured, bool, error) {
	for _, res := range resources {
		resAnnotations := res.GetAnnotations()
		if resAnnotations == nil {
			continue
		}

		// Check if this resource has a matching composition resource name
		if !m.hasMatchingResourceName(resAnnotations, compResourceName) {
			continue
		}

		// If we have a generateName, verify the match has the right prefix
		if generateName != "" {
			resName := res.GetName()
			if !strings.HasPrefix(resName, generateName) {
				m.logger.Debug("Found resource with matching composition name but wrong generateName prefix",
					"expectedPrefix", generateName,
					"actualName", resName)
				continue
			}
		}

		// We found a match!
		m.logger.Debug("Found resource by label and annotation",
			"resource", res.GetName(),
			"compositionResourceName", compResourceName)
		return res, true, nil
	}

	m.logger.Debug("No matching resource found with composition resource name",
		"compositionResourceName", compResourceName)
	return nil, false, nil
}

// hasMatchingResourceName checks if annotations have a matching composition-resource-name
func (m *DefaultResourceManager) hasMatchingResourceName(annotations map[string]string, compResourceName string) bool {
	// Check standard annotation
	if value, exists := annotations["crossplane.io/composition-resource-name"]; exists && value == compResourceName {
		return true
	}

	// Check function-specific variations
	for key, value := range annotations {
		if strings.HasSuffix(key, "/composition-resource-name") && value == compResourceName {
			return true
		}
	}

	return false
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

	// Update composite owner label
	m.updateCompositeOwnerLabel(parent, child)

	m.logger.Debug("Updated owner references and labels",
		"newCount", len(updatedRefs))
}

// updateCompositeOwnerLabel updates the crossplane.io/composite label on the child
func (m *DefaultResourceManager) updateCompositeOwnerLabel(parent, child *unstructured.Unstructured) {
	if parent == nil {
		return
	}

	// Get current labels or create a new map
	labels := child.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Set the composite owner label
	parentName := parent.GetName()
	if parentName == "" && parent.GetGenerateName() != "" {
		// For XRs with only generateName, use the generateName prefix
		parentName = parent.GetGenerateName()
	}

	if parentName != "" {
		labels["crossplane.io/composite"] = parentName
		child.SetLabels(labels)
		m.logger.Debug("Updated composite owner label",
			"label", parentName,
			"child", child.GetName())
	}
}
