package diffprocessor

import (
	"context"
	"fmt"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1"
	"github.com/crossplane/crossplane/cmd/crank/beta/diff/resourceutils"
	"strings"
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	cc "github.com/crossplane/crossplane/cmd/crank/beta/diff/clusterclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RequirementsProvider consolidates requirement processing with caching
type RequirementsProvider struct {
	client   cc.ClusterClient
	renderFn RenderFunc
	logger   logging.Logger

	// Resource cache by resource key (apiVersion+kind+name)
	resourceCache map[string]*unstructured.Unstructured
	cacheMutex    sync.RWMutex
}

// NewRequirementsProvider creates a new provider with caching
func NewRequirementsProvider(
	client cc.ClusterClient,
	renderFn RenderFunc,
	logger logging.Logger,
) *RequirementsProvider {
	return &RequirementsProvider{
		client:        client,
		renderFn:      renderFn,
		logger:        logger,
		resourceCache: make(map[string]*unstructured.Unstructured),
	}
}

// Initialize pre-fetches resources like environment configs
func (p *RequirementsProvider) Initialize(ctx context.Context) error {
	p.logger.Debug("Initializing extra resource provider")

	// Pre-fetch environment configs
	envConfigs, err := p.client.GetEnvironmentConfigs(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot get environment configs")
	}

	// Add to cache
	p.cacheResources(envConfigs)

	p.logger.Debug("Extra resource provider initialized",
		"envConfigCount", len(envConfigs),
		"cacheSize", len(p.resourceCache))

	return nil
}

// cacheResources adds resources to the cache
func (p *RequirementsProvider) cacheResources(resources []*unstructured.Unstructured) {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	for _, res := range resources {
		key := fmt.Sprintf("%s/%s/%s", res.GetAPIVersion(), res.GetKind(), res.GetName())
		p.resourceCache[key] = res
	}
}

// getCachedResource retrieves a resource from cache if available
func (p *RequirementsProvider) getCachedResource(apiVersion, kind, name string) *unstructured.Unstructured {
	p.cacheMutex.RLock()
	defer p.cacheMutex.RUnlock()

	key := fmt.Sprintf("%s/%s/%s", apiVersion, kind, name)
	return p.resourceCache[key]
}

// ProvideRequirements provides requirements, checking cache first
func (p *RequirementsProvider) ProvideRequirements(
	ctx context.Context,
	requirements map[string]v1.Requirements,
) ([]*unstructured.Unstructured, error) {
	if len(requirements) == 0 {
		return nil, nil
	}

	var allResources []*unstructured.Unstructured
	var newlyFetchedResources []*unstructured.Unstructured

	// Process each step's requirements
	for stepName := range requirements {
		// Process resource selectors directly using the map key
		for resourceKey, selector := range requirements[stepName].ExtraResources {
			if selector == nil {
				p.logger.Debug("Nil selector in requirements",
					"step", stepName,
					"resourceKey", resourceKey)
				continue
			}

			// Parse apiVersion into group/version
			group, version := parseAPIVersion(selector.ApiVersion)

			gvk := schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    selector.Kind,
			}

			// Process by selector type
			switch {
			case selector.GetMatchName() != "":
				// Try to get from cache first
				name := selector.GetMatchName()
				cached := p.getCachedResource(selector.ApiVersion, selector.Kind, name)

				if cached != nil {
					p.logger.Debug("Found resource in cache",
						"apiVersion", selector.ApiVersion,
						"kind", selector.Kind,
						"name", name)
					allResources = append(allResources, cached)
					continue
				}

				// Not in cache, fetch from cluster
				ns := "" // TODO: handle namespaced resources

				p.logger.Debug("Fetching reference by name",
					"gvk", gvk.String(),
					"name", name)

				resource, err := p.client.GetResource(ctx, gvk, ns, name)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get referenced resource %s/%s", ns, name)
				}

				allResources = append(allResources, resource)
				newlyFetchedResources = append(newlyFetchedResources, resource)

			case selector.GetMatchLabels() != nil:
				// Label selectors always go to the cluster
				// (Can't efficiently check cache for label matches)
				gvr := resourceutils.KindToResource(gvk)

				// Convert MatchLabels to LabelSelector
				labelSelector := metav1.LabelSelector{
					MatchLabels: selector.GetMatchLabels().GetLabels(),
				}

				p.logger.Debug("Fetching resources by label",
					"gvr", gvr.String(),
					"labels", labelSelector.MatchLabels)

				resources, err := p.client.GetResourcesByLabel(ctx, "", gvr, labelSelector)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot get resources by label")
				}

				allResources = append(allResources, resources...)
				newlyFetchedResources = append(newlyFetchedResources, resources...)

			default:
				p.logger.Debug("Unsupported selector type",
					"step", stepName,
					"resourceKey", resourceKey)
			}
		}
	}

	// Cache any newly fetched resources
	if len(newlyFetchedResources) > 0 {
		p.cacheResources(newlyFetchedResources)
	}

	p.logger.Debug("Processed all requirements",
		"resourceCount", len(allResources),
		"newlyFetchedCount", len(newlyFetchedResources),
		"cacheSize", len(p.resourceCache))

	return allResources, nil
}

// ClearCache clears all cached resources
func (p *RequirementsProvider) ClearCache() {
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	p.resourceCache = make(map[string]*unstructured.Unstructured)
	p.logger.Debug("Resource cache cleared")
}

// Helper to parse apiVersion into group and version
func parseAPIVersion(apiVersion string) (string, string) {
	group, version := "", ""
	if parts := strings.SplitN(apiVersion, "/", 2); len(parts) == 2 {
		// Normal case: group/version (e.g., "apps/v1")
		group, version = parts[0], parts[1]
	} else {
		// Core case: version only (e.g., "v1")
		version = apiVersion
	}
	return group, version
}
