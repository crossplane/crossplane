package crossplane

import (
	"context"
	"fmt"

	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/core"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/client/kubernetes"
)

// CompositionClient handles operations related to Compositions.
type CompositionClient interface {
	core.Initializable

	// FindMatchingComposition finds a composition that matches the given XR or claim
	FindMatchingComposition(ctx context.Context, res *un.Unstructured) (*apiextensionsv1.Composition, error)

	// ListCompositions lists all compositions in the cluster
	ListCompositions(ctx context.Context) ([]*apiextensionsv1.Composition, error)

	// GetComposition gets a composition by name
	GetComposition(ctx context.Context, name string) (*apiextensionsv1.Composition, error)
}

// DefaultCompositionClient implements CompositionClient.
type DefaultCompositionClient struct {
	resourceClient kubernetes.ResourceClient
	logger         logging.Logger

	// Cache of compositions
	compositions map[string]*apiextensionsv1.Composition
}

// NewCompositionClient creates a new DefaultCompositionClient.
func NewCompositionClient(resourceClient kubernetes.ResourceClient, logger logging.Logger) CompositionClient {
	return &DefaultCompositionClient{
		resourceClient: resourceClient,
		logger:         logger,
		compositions:   make(map[string]*apiextensionsv1.Composition),
	}
}

// Initialize loads compositions into the cache.
func (c *DefaultCompositionClient) Initialize(ctx context.Context) error {
	c.logger.Debug("Initializing composition client")

	// List compositions to populate the cache
	comps, err := c.ListCompositions(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot list compositions")
	}

	// Store in cache
	for _, comp := range comps {
		c.compositions[comp.GetName()] = comp
	}

	c.logger.Debug("Composition client initialized", "compositionsCount", len(c.compositions))
	return nil
}

// ListCompositions lists all compositions in the cluster.
func (c *DefaultCompositionClient) ListCompositions(ctx context.Context) ([]*apiextensionsv1.Composition, error) {
	c.logger.Debug("Listing compositions from cluster")

	// Define the composition GVK
	gvk := schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v1",
		Kind:    "Composition",
	}

	// Get all compositions using the resource client
	unComps, err := c.resourceClient.ListResources(ctx, gvk, "")
	if err != nil {
		c.logger.Debug("Failed to list compositions", "error", err)
		return nil, errors.Wrap(err, "cannot list compositions from cluster")
	}

	// Convert unstructured to typed
	compositions := make([]*apiextensionsv1.Composition, 0, len(unComps))
	for _, obj := range unComps {
		comp := &apiextensionsv1.Composition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, comp); err != nil {
			c.logger.Debug("Failed to convert composition from unstructured",
				"name", obj.GetName(),
				"error", err)
			return nil, errors.Wrap(err, "cannot convert unstructured to Composition")
		}
		compositions = append(compositions, comp)
	}

	c.logger.Debug("Successfully retrieved compositions", "count", len(compositions))
	return compositions, nil
}

// GetComposition gets a composition by name.
func (c *DefaultCompositionClient) GetComposition(ctx context.Context, name string) (*apiextensionsv1.Composition, error) {
	// Check cache first
	if comp, ok := c.compositions[name]; ok {
		return comp, nil
	}

	// Not in cache, fetch from cluster
	gvk := schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v1",
		Kind:    "Composition",
	}

	unComp, err := c.resourceClient.GetResource(ctx, gvk, "", name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get composition %s", name)
	}

	// Convert to typed
	comp := &apiextensionsv1.Composition{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unComp.Object, comp); err != nil {
		return nil, errors.Wrap(err, "cannot convert unstructured to Composition")
	}

	// Update cache
	c.compositions[name] = comp

	return comp, nil
}

// FindMatchingComposition finds a composition matching the given resource.
func (c *DefaultCompositionClient) FindMatchingComposition(ctx context.Context, res *un.Unstructured) (*apiextensionsv1.Composition, error) {
	gvk := res.GroupVersionKind()
	resourceID := fmt.Sprintf("%s/%s", gvk.String(), res.GetName())

	c.logger.Debug("Finding matching composition",
		"resource_name", res.GetName(),
		"gvk", gvk.String())

	// First, check if this is a claim by looking for an XRD that defines this as a claim
	xrdForClaim, err := c.findClaimXRD(ctx, gvk)
	if err != nil {
		c.logger.Debug("Error checking if resource is claim type",
			"resource", resourceID,
			"error", err)
		// Continue as if not a claim - we'll try normal composition matching
	}

	// If it's a claim, we need to find compositions for the corresponding XR type
	var targetGVK schema.GroupVersionKind
	if xrdForClaim != nil {
		targetGVK, err = c.getXRTypeFromXRD(xrdForClaim, resourceID)
		if err != nil {
			return nil, errors.Wrapf(err, "claim %s requires its XR type to find a composition", resourceID)
		}
	} else {
		// Not a claim or couldn't determine XRD - use the actual resource GVK
		targetGVK = gvk
	}

	// Case 1: Check for direct composition reference in spec.compositionRef.name
	comp, err := c.findByDirectReference(ctx, res, targetGVK, resourceID)
	if err != nil || comp != nil {
		return comp, err
	}

	// Case 2: Check for selector-based composition reference
	comp, err = c.findByLabelSelector(ctx, res, targetGVK, resourceID)
	if err != nil || comp != nil {
		return comp, err
	}

	// Case 3: Look up by composite type reference (default behavior)
	return c.findByTypeReference(ctx, targetGVK, resourceID)
}

// findClaimXRD checks if the given GVK is a claim type and returns the corresponding XRD if found.
func (c *DefaultCompositionClient) findClaimXRD(ctx context.Context, gvk schema.GroupVersionKind) (*un.Unstructured, error) {
	c.logger.Debug("Checking if resource is a claim type",
		"gvk", gvk.String())

	// Define XRD GVK
	xrdGVK := schema.GroupVersionKind{
		Group:   "apiextensions.crossplane.io",
		Version: "v1",
		Kind:    "CompositeResourceDefinition",
	}

	// List all XRDs
	xrds, err := c.resourceClient.ListResources(ctx, xrdGVK, "")
	if err != nil {
		c.logger.Debug("Error getting XRDs",
			"error", err)
		return nil, errors.Wrap(err, "cannot get XRDs")
	}

	// Loop through XRDs to find one that defines this GVK as a claim
	for _, xrd := range xrds {
		claimGroup, found, _ := un.NestedString(xrd.Object, "spec", "group")

		// Skip if group doesn't match
		if !found || claimGroup != gvk.Group {
			continue
		}

		// Check claim kind
		claimNames, found, _ := un.NestedMap(xrd.Object, "spec", "claimNames")
		if !found || claimNames == nil {
			continue
		}

		claimKind, found, _ := un.NestedString(claimNames, "kind")
		if !found || claimKind != gvk.Kind {
			continue
		}

		c.logger.Debug("Found matching XRD for claim type",
			"gvk", gvk.String(),
			"xrd", xrd.GetName())

		return xrd, nil
	}

	// No matching XRD found - not a claim type
	return nil, nil
}

// getXRTypeFromXRD extracts the XR GroupVersionKind from an XRD.
func (c *DefaultCompositionClient) getXRTypeFromXRD(xrdForClaim *un.Unstructured, resourceID string) (schema.GroupVersionKind, error) {
	// Get the XR type from the XRD
	xrGroup, found, _ := un.NestedString(xrdForClaim.Object, "spec", "group")
	xrKind, kindFound, _ := un.NestedString(xrdForClaim.Object, "spec", "names", "kind")

	if !found || !kindFound {
		return schema.GroupVersionKind{}, errors.New("could not determine group or kind from XRD")
	}

	// Find the referenceable version - there should be exactly one
	xrVersion := ""
	versions, versionsFound, _ := un.NestedSlice(xrdForClaim.Object, "spec", "versions")
	if versionsFound && len(versions) > 0 {
		// Look for the one version that is marked referenceable
		for _, versionObj := range versions {
			if version, ok := versionObj.(map[string]interface{}); ok {
				ref, refFound, _ := un.NestedBool(version, "referenceable")
				if refFound && ref {
					name, nameFound, _ := un.NestedString(version, "name")
					if nameFound {
						xrVersion = name
						break
					}
				}
			}
		}
	}

	// If no referenceable version found, we shouldn't guess
	if xrVersion == "" {
		return schema.GroupVersionKind{}, errors.New("no referenceable version found in XRD")
	}

	targetGVK := schema.GroupVersionKind{
		Group:   xrGroup,
		Version: xrVersion,
		Kind:    xrKind,
	}

	c.logger.Debug("Claim resource detected - targeting XR type for composition matching",
		"claim", resourceID,
		"targetXR", targetGVK.String())

	return targetGVK, nil
}

// isCompositionCompatible checks if a composition is compatible with a GVK.
func (c *DefaultCompositionClient) isCompositionCompatible(comp *apiextensionsv1.Composition, xrGVK schema.GroupVersionKind) bool {
	return comp.Spec.CompositeTypeRef.APIVersion == xrGVK.GroupVersion().String() &&
		comp.Spec.CompositeTypeRef.Kind == xrGVK.Kind
}

// labelsMatch checks if a resource's labels match a selector.
func (c *DefaultCompositionClient) labelsMatch(labels, selector map[string]string) bool {
	// A resource matches a selector if all the selector's labels exist in the resource's labels
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// findByDirectReference attempts to find a composition directly referenced by name.
func (c *DefaultCompositionClient) findByDirectReference(ctx context.Context, res *un.Unstructured, targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	compositionRefName, compositionRefFound, err := un.NestedString(res.Object, "spec", "compositionRef", "name")
	if err == nil && compositionRefFound && compositionRefName != "" {
		c.logger.Debug("Found direct composition reference",
			"resource", resourceID,
			"compositionName", compositionRefName)

		// Look up composition by name
		comp, err := c.GetComposition(ctx, compositionRefName)
		if err != nil {
			return nil, errors.Errorf("composition %s referenced in %s not found",
				compositionRefName, resourceID)
		}

		// Validate that the composition's compositeTypeRef matches the target GVK
		if !c.isCompositionCompatible(comp, targetGVK) {
			return nil, errors.Errorf("composition %s is not compatible with %s",
				compositionRefName, targetGVK.String())
		}

		c.logger.Debug("Found composition by direct reference",
			"resource", resourceID,
			"composition", comp.GetName())
		return comp, nil
	}

	return nil, nil // No direct reference found
}

// findByLabelSelector attempts to find compositions that match label selectors.
func (c *DefaultCompositionClient) findByLabelSelector(ctx context.Context, res *un.Unstructured, targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	matchLabels, selectorFound, err := un.NestedMap(res.Object, "spec", "compositionSelector", "matchLabels")
	if err == nil && selectorFound && len(matchLabels) > 0 {
		c.logger.Debug("Found composition selector",
			"resource", resourceID,
			"matchLabels", matchLabels)

		// Convert matchLabels to string map for comparison
		stringLabels := make(map[string]string)
		for k, v := range matchLabels {
			if strVal, ok := v.(string); ok {
				stringLabels[k] = strVal
			}
		}

		// Find compositions matching the labels
		var matchingCompositions []*apiextensionsv1.Composition

		// Get all compositions if we haven't loaded them yet
		if len(c.compositions) == 0 {
			if _, err := c.ListCompositions(ctx); err != nil {
				return nil, errors.Wrap(err, "cannot list compositions to match selector")
			}
		}

		// Search through all compositions looking for compatible ones with matching labels
		for _, comp := range c.compositions {
			// Check if this composition is for the right XR type
			if c.isCompositionCompatible(comp, targetGVK) {
				// Check if labels match
				if c.labelsMatch(comp.GetLabels(), stringLabels) {
					matchingCompositions = append(matchingCompositions, comp)
				}
			}
		}

		// Handle matching results
		switch len(matchingCompositions) {
		case 0:
			return nil, errors.Errorf("no compatible composition found matching labels %v for %s",
				stringLabels, resourceID)
		case 1:
			c.logger.Debug("Found composition by label selector",
				"resource", resourceID,
				"composition", matchingCompositions[0].GetName())
			return matchingCompositions[0], nil
		default:
			// Multiple matches - this is ambiguous and should fail
			names := make([]string, len(matchingCompositions))
			for i, comp := range matchingCompositions {
				names[i] = comp.GetName()
			}
			return nil, errors.New("ambiguous composition selection: multiple compositions match")
		}
	}

	return nil, nil // No label selector found or no matches
}

// findByTypeReference attempts to find a composition by matching the type reference.
func (c *DefaultCompositionClient) findByTypeReference(ctx context.Context, targetGVK schema.GroupVersionKind, resourceID string) (*apiextensionsv1.Composition, error) {
	// Get all compositions if we haven't loaded them yet
	if len(c.compositions) == 0 {
		if _, err := c.ListCompositions(ctx); err != nil {
			return nil, errors.Wrap(err, "cannot list compositions to match type")
		}
	}

	// Find all compositions that match this target type
	var compatibleCompositions []*apiextensionsv1.Composition

	for _, comp := range c.compositions {
		if c.isCompositionCompatible(comp, targetGVK) {
			compatibleCompositions = append(compatibleCompositions, comp)
		}
	}

	if len(compatibleCompositions) == 0 {
		c.logger.Debug("No matching composition found",
			"targetGVK", targetGVK.String())
		return nil, errors.Errorf("no composition found for %s", targetGVK.String())
	}

	if len(compatibleCompositions) > 1 {
		// Multiple compositions match, but no selection criteria was provided
		// This is an ambiguous situation
		names := make([]string, len(compatibleCompositions))
		for i, comp := range compatibleCompositions {
			names[i] = comp.GetName()
		}
		return nil, errors.Errorf("ambiguous composition selection: multiple compositions exist for %s", targetGVK.String())
	}

	// We have exactly one matching composition
	c.logger.Debug("Found matching composition by type reference",
		"resource_name", resourceID,
		"composition_name", compatibleCompositions[0].GetName())
	return compatibleCompositions[0], nil
}
