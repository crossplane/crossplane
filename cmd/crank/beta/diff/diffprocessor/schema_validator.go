package diffprocessor

import (
	"context"
	"fmt"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	"github.com/crossplane/crossplane/cmd/crank/beta/internal"
	"github.com/crossplane/crossplane/cmd/crank/beta/validate"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemaValidator handles validation of resources against CRD schemas
type SchemaValidator interface {
	// ValidateResources validates resources using schema validation
	ValidateResources(ctx context.Context, xr *unstructured.Unstructured, composed []composed.Unstructured) error

	// EnsureComposedResourceCRDs ensures we have all required CRDs for validation
	EnsureComposedResourceCRDs(ctx context.Context, resources []*unstructured.Unstructured) error
}

// DefaultSchemaValidator implements SchemaValidator interface
type DefaultSchemaValidator struct {
	client cc.ClusterClient
	logger logging.Logger
	crds   []*extv1.CustomResourceDefinition
}

// NewSchemaValidator creates a new DefaultSchemaValidator
func NewSchemaValidator(client cc.ClusterClient, logger logging.Logger) SchemaValidator {
	return &DefaultSchemaValidator{
		client: client,
		logger: logger,
		crds:   []*extv1.CustomResourceDefinition{},
	}
}

// LoadCRDs loads CRDs from the cluster
func (v *DefaultSchemaValidator) LoadCRDs(ctx context.Context) error {
	v.logger.Debug("Loading CRDs from cluster")

	// Get XRDs from the client (which will use its cache when available)
	xrds, err := v.client.GetXRDs(ctx)
	if err != nil {
		v.logger.Debug("Failed to get XRDs", "error", err)
		return errors.Wrap(err, "cannot get XRDs")
	}

	// Convert XRDs to CRDs
	crds, err := internal.ConvertToCRDs(xrds)
	if err != nil {
		v.logger.Debug("Failed to convert XRDs to CRDs", "error", err)
		return errors.Wrap(err, "cannot convert XRDs to CRDs")
	}

	v.crds = crds
	v.logger.Debug("Loaded CRDs", "count", len(crds))
	return nil
}

// SetCRDs sets the CRDs directly, useful for testing or when CRDs are pre-loaded
func (v *DefaultSchemaValidator) SetCRDs(crds []*extv1.CustomResourceDefinition) {
	v.crds = crds
	v.logger.Debug("Set CRDs directly", "count", len(crds))
}

// GetCRDs returns the current CRDs
func (v *DefaultSchemaValidator) GetCRDs() []*extv1.CustomResourceDefinition {
	return v.crds
}

// ValidateResources validates resources using schema validation
func (v *DefaultSchemaValidator) ValidateResources(ctx context.Context, xr *unstructured.Unstructured, composed []composed.Unstructured) error {
	v.logger.Debug("Validating resources",
		"xr", fmt.Sprintf("%s/%s", xr.GetKind(), xr.GetName()),
		"composedCount", len(composed))

	// Collect all resources that need to be validated
	resources := make([]*unstructured.Unstructured, 0, len(composed)+1)

	// Add the XR to the validation list
	resources = append(resources, xr)

	// Add composed resources to validation list
	for i := range composed {
		resources = append(resources, &unstructured.Unstructured{Object: composed[i].UnstructuredContent()})
	}

	// Ensure we have all the required CRDs
	v.logger.Debug("Ensuring required CRDs for validation",
		"cachedCRDs", len(v.crds),
		"resourceCount", len(resources))

	if err := v.EnsureComposedResourceCRDs(ctx, resources); err != nil {
		return errors.Wrap(err, "unable to ensure CRDs")
	}

	// Create a logger writer to capture output
	loggerWriter := internal.NewLoggerWriter(v.logger)

	// Validate using the CRD schemas
	// Use skipSuccessLogs=true to avoid cluttering the output with success messages
	v.logger.Debug("Performing schema validation", "resourceCount", len(resources))
	if err := validate.SchemaValidation(resources, v.crds, true, true, loggerWriter); err != nil {
		return errors.Wrap(err, "schema validation failed")
	}

	v.logger.Debug("Resources validated successfully")
	return nil
}

// EnsureComposedResourceCRDs checks if we have all the CRDs needed for the composed resources
// and fetches any missing ones from the cluster
func (v *DefaultSchemaValidator) EnsureComposedResourceCRDs(ctx context.Context, resources []*unstructured.Unstructured) error {
	// Create a map of existing CRDs by GVK for quick lookup
	existingCRDs := make(map[schema.GroupVersionKind]bool)
	for _, crd := range v.crds {
		for _, version := range crd.Spec.Versions {
			gvk := schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: version.Name,
				Kind:    crd.Spec.Names.Kind,
			}
			existingCRDs[gvk] = true
		}
	}

	// Collect GVKs from resources that aren't already covered
	missingGVKs := make(map[schema.GroupVersionKind]bool)
	for _, res := range resources {
		gvk := res.GroupVersionKind()
		if !existingCRDs[gvk] {
			missingGVKs[gvk] = true
		}
	}

	// If we have all the CRDs already, we're done
	if len(missingGVKs) == 0 {
		v.logger.Debug("All required CRDs are already cached")
		return nil
	}

	v.logger.Debug("Fetching additional CRDs", "missingCount", len(missingGVKs))

	// Fetch missing CRDs
	for gvk := range missingGVKs {
		// Skip resources that don't require CRDs
		if !v.client.IsCRDRequired(ctx, gvk) {
			v.logger.Debug("Skipping built-in resource type, no CRD required",
				"gvk", gvk.String())
			continue
		}

		// Try to get the CRD using the client's GetCRD method
		crdObj, err := v.client.GetCRD(ctx, gvk)
		if err != nil {
			v.logger.Debug("CRD not found (continuing)",
				"gvk", gvk.String(),
				"error", err)
			return errors.New("unable to find CRD for " + gvk.String())
		}

		// Convert to CRD
		crd := &extv1.CustomResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(crdObj.Object, crd); err != nil {
			v.logger.Debug("Error converting CRD (continuing)",
				"gvk", gvk.String(),
				"error", err)
			continue
		}

		// Add to our cache
		v.crds = append(v.crds, crd)
		v.logger.Debug("Added CRD to cache", "crdName", crd.Name)
	}

	v.logger.Debug("Finished ensuring CRDs", "totalCRDs", len(v.crds))
	return nil
}
