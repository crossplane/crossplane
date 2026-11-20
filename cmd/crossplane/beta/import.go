/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package beta

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	xpv1alpha1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
)

// ImportCommand defines the `crossplane beta import` subcommand.
type ImportCommand struct {
	Discover discoverCmd `cmd:"" help:"Discover external resources and generate adoption manifests."`
}

// Run is a no-op for kong command tree.
func (c *ImportCommand) Run() error {
	return nil
}

// discoverCmd implements the actual discovery and import workflow.
type discoverCmd struct {
	Kind           string   `help:"Managed resource kind to discover (required)"                      required:""`
	Group          string   `help:"API group of the resource (required)"                              required:""`
	ProviderConfig string   `default:"default"                                                        help:"ProviderConfig name to use for discovery"`
	Namespace      string   `default:"default"                                                        help:"Namespace where adopted resources will be created"`
	DryRun         bool     `default:"true"                                                           help:"Print YAML without applying (default: true)"`
	AutoImport     bool     `default:"false"                                                          help:"Apply manifests automatically (requires --dry-run=false)"`
	Scope          []string `help:"Only import resources whose scope has this key=value (repeatable)"`
	Output         string   `help:"Write YAML to file instead of stdout"`
}

// Run executes the discovery workflow.
func (c *discoverCmd) Run(log logging.Logger) error {
	// Step 1: Validate flags
	if !c.DryRun && !c.AutoImport {
		return errors.New("--auto-import must be true when --dry-run=false")
	}

	ctx := context.Background()

	// Step 2: Initialize Kubernetes clients
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to out-of-cluster config (kubeconfig)
		cfg, err = ctrl.GetConfig()
		if err != nil {
			return errors.Wrap(err, "cannot load Kubernetes config")
		}
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes client")
	}
	_ = k8sClient // May be used in future implementation

	dClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create dynamic client")
	}

	// Step 3: Look up MRD for the given kind/group
	log.Debug("Looking up ManagedResourceDefinition", "kind", c.Kind, "group", c.Group)
	mrd, err := c.getMRD(ctx, dClient)
	if err != nil {
		return errors.Wrap(err, "cannot get ManagedResourceDefinition")
	}

	log.Debug("Found MRD", "name", mrd.Name)

	// Step 5: Validate the scope filter
	filter, err := parseScopeFilter(c.Scope)
	if err != nil {
		return errors.Wrap(err, "invalid --scope")
	}

	// Step 6: List external resources from DiscoveryReport
	log.Debug("Querying DiscoveryReport for external resources")
	external, err := c.listExternalResources(ctx, log, dClient, mrd)
	if err != nil {
		return errors.Wrap(err, "cannot list external resources")
	}
	log.Debug("Discovered resources", "count", len(external))

	// Step 7: Get existing managed resources in cluster
	log.Debug("Querying existing managed resources")
	existing, err := c.getExistingManagedResources(ctx, dClient, mrd)
	if err != nil {
		return errors.Wrap(err, "cannot get existing managed resources")
	}
	log.Debug("Found existing resources", "count", len(existing))

	// Step 8: Compute the unmanaged resources within the requested scope
	unmanaged := unmanagedResources(inScope(external, filter), existing)
	if len(unmanaged) == 0 {
		if _, err := fmt.Fprintln(os.Stdout, "No unmanaged resources found."); err != nil {
			return errors.Wrap(err, "cannot write output")
		}
		return nil
	}
	log.Debug("Unmanaged resources", "count", len(unmanaged))

	// Step 8: Generate adoption manifests
	log.Debug("Generating adoption manifests")
	objs, err := c.generateManifests(mrd, unmanaged)
	if err != nil {
		return errors.Wrap(err, "cannot generate manifests")
	}

	// Step 9: Render and output YAML
	rendered, err := renderManifests(objs)
	if err != nil {
		return errors.Wrap(err, "cannot render manifests")
	}

	if err := c.outputManifests(rendered); err != nil {
		return errors.Wrap(err, "cannot output manifests")
	}

	// Step 10: Auto-import if requested
	if !c.DryRun && c.AutoImport {
		log.Debug("Applying manifests")
		if err := c.applyManifests(ctx, dClient, objs, mrd); err != nil {
			return errors.Wrap(err, "cannot apply manifests")
		}
		if _, err := fmt.Fprintf(os.Stdout, "Successfully imported %d resources.\n", len(unmanaged)); err != nil {
			return errors.Wrap(err, "cannot write output")
		}
	}

	return nil
}

// getMRD looks up the ManagedResourceDefinition by kind/group.
func (c *discoverCmd) getMRD(ctx context.Context, dClient dynamic.Interface) (*xpv1alpha1.ManagedResourceDefinition, error) {
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "managedresourcedefinitions",
	}

	// Query the cluster for MRDs matching our kind and group
	list, err := dClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list MRDs")
	}

	var foundMRD *xpv1alpha1.ManagedResourceDefinition
	for _, item := range list.Items {
		mrd := &xpv1alpha1.ManagedResourceDefinition{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, mrd); err != nil {
			continue
		}
		if mrd.Spec.Names.Kind == c.Kind && mrd.Spec.Group == c.Group {
			foundMRD = mrd
			break
		}
	}

	if foundMRD == nil {
		return nil, errors.Errorf("ManagedResourceDefinition not found: %s/%s", c.Group, c.Kind)
	}

	return foundMRD, nil
}

// renderManifests renders adoption objects as a multi-document YAML manifest.
func renderManifests(objs []*unstructured.Unstructured) (string, error) {
	docs := make([]string, 0, len(objs))

	for _, obj := range objs {
		b, err := yaml.Marshal(obj.Object)
		if err != nil {
			return "", errors.Wrap(err, "cannot marshal manifest")
		}
		docs = append(docs, string(b))
	}

	return strings.Join(docs, "---\n"), nil
}

// outputManifests writes the YAML to stdout or a file.
func (c *discoverCmd) outputManifests(yaml string) error {
	if c.Output != "" {
		// Write to file
		if err := os.WriteFile(c.Output, []byte(yaml), 0o600); err != nil {
			return errors.Wrap(err, "cannot write to output file")
		}
		if _, err := fmt.Fprintf(os.Stdout, "Manifests written to %s\n", c.Output); err != nil {
			return errors.Wrap(err, "cannot write output")
		}
		return nil
	}

	// Write to stdout
	if _, err := fmt.Fprint(os.Stdout, yaml); err != nil {
		return errors.Wrap(err, "cannot write output")
	}
	return nil
}

// managedVersion returns the name of the MRD's storage version, which is the
// version Crossplane serves managed resources at for the MRD's group and kind.
func managedVersion(mrd *xpv1alpha1.ManagedResourceDefinition) (string, error) {
	for _, v := range mrd.Spec.Versions {
		if v.Storage {
			return v.Name, nil
		}
	}

	return "", errors.Errorf("ManagedResourceDefinition %s has no storage version", mrd.GetName())
}

// applyManifests creates the given adoption objects in the cluster.
func (c *discoverCmd) applyManifests(ctx context.Context, dClient dynamic.Interface, objs []*unstructured.Unstructured, mrd *xpv1alpha1.ManagedResourceDefinition) error {
	// The MRD carries the authoritative plural and API version. Deriving the
	// plural from the kind gets it wrong for irregular kinds, e.g. an
	// IPAddress kind is served as "ipaddresses", not "ipaddresss".
	version, err := managedVersion(mrd)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    mrd.Spec.Group,
		Version:  version,
		Resource: mrd.Spec.Names.Plural,
	}

	for _, obj := range objs {
		ns := obj.GetNamespace()
		if ns == "" {
			ns = c.Namespace
			obj.SetNamespace(ns)
		}

		_, err := dClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			// If resource already exists, that's acceptable (idempotent)
			if kerrors.IsAlreadyExists(err) {
				continue
			}
			return errors.Wrapf(err, "cannot create %s/%s", obj.GetKind(), obj.GetName())
		}
	}

	return nil
}

// Helper functions

// externalResource is a resource the provider listed, with the scope fields that
// locate it within its parent set.
type externalResource struct {
	Name  string
	Scope map[string]string
}

// existingResource is a managed resource of the kind being imported: its external
// name and the spec.forProvider fields that scope it.
type existingResource struct {
	Name        string
	ForProvider map[string]any
}

// parseScopeFilter parses --scope arguments of the form key=value.
func parseScopeFilter(args []string) (map[string]string, error) {
	filter := make(map[string]string, len(args))
	for _, arg := range args {
		k, v, ok := strings.Cut(arg, "=")
		if !ok || k == "" || v == "" {
			return nil, errors.Errorf("%q is not in key=value form", arg)
		}

		filter[k] = v
	}

	return filter, nil
}

// inScope returns the resources whose scope has every filter value.
func inScope(resources []externalResource, filter map[string]string) []externalResource {
	var out []externalResource
	for _, res := range resources {
		if matchesFilter(res.Scope, filter) {
			out = append(out, res)
		}
	}

	return out
}

func matchesFilter(scope, filter map[string]string) bool {
	for k, want := range filter {
		if got, ok := scope[k]; !ok || got != want {
			return false
		}
	}

	return true
}

// listExternalResources reads the unmanaged resources, with their scopes, from the
// DiscoveryReport. The discovery controller populates it from the provider's
// ExternalInventory.
func (c *discoverCmd) listExternalResources(ctx context.Context, log logging.Logger, dClient dynamic.Interface, mrd *xpv1alpha1.ManagedResourceDefinition) ([]externalResource, error) {
	// Query DiscoveryReport CRD by MRD name (cluster-scoped)
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "discoveryreports",
	}

	// Try to get the DiscoveryReport with the same name as the MRD
	dr, err := dClient.Resource(gvr).Get(ctx, mrd.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get DiscoveryReport for MRD %q; "+
			"ensure the discovery controller is running and has scanned this resource type", mrd.Name)
	}

	entries, ok, err := unstructured.NestedSlice(dr.Object, "status", "unmanagedResources")
	if err != nil {
		return nil, errors.Wrap(err, "cannot read unmanaged resources from DiscoveryReport status")
	}
	if !ok {
		// No unmanaged resources found (all are already managed)
		return []externalResource{}, nil
	}

	var out []externalResource
	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			log.Debug("skipping invalid unmanagedResource entry")
			continue
		}

		name, ok := entryMap["externalName"].(string)
		if !ok || name == "" {
			continue
		}

		out = append(out, externalResource{Name: name, Scope: scopeFromValue(entryMap["scope"])})
	}

	// Warn if the DiscoveryReport is stale (> 10 minutes old)
	lastScanTime, found, err := unstructured.NestedString(dr.Object, "status", "lastDiscoveryTime")
	if err != nil {
		return nil, errors.Wrap(err, "cannot read last discovery time from DiscoveryReport status")
	}
	if found && lastScanTime != "" {
		lastScan, parseErr := time.Parse(time.RFC3339, lastScanTime)
		if parseErr == nil && time.Since(lastScan) > 10*time.Minute {
			log.Debug("DiscoveryReport is stale; results may be outdated", "lastScan", lastScan)
		}
	}

	return out, nil
}

// scopeFromValue converts the unstructured scope of a DiscoveryReport entry.
func scopeFromValue(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}

	scope := make(map[string]string, len(m))
	for k, val := range m {
		scope[k] = fmt.Sprint(val)
	}

	return scope
}

// getExistingManagedResources queries the cluster for all MRs of the given kind
// and extracts their external names and scope fields.
func (c *discoverCmd) getExistingManagedResources(ctx context.Context, dClient dynamic.Interface, mrd *xpv1alpha1.ManagedResourceDefinition) ([]existingResource, error) {
	version, err := managedVersion(mrd)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:   mrd.Spec.Group,
		Version: version,
		// Use the plural from the MRD rather than guessing it from the kind.
		Resource: mrd.Spec.Names.Plural,
	}

	list, err := dClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list managed resources")
	}

	var existing []existingResource
	for _, item := range list.Items {
		extName, ok := item.GetAnnotations()["crossplane.io/external-name"]
		if !ok || extName == "" {
			continue
		}

		forProvider, _, _ := unstructured.NestedMap(item.Object, "spec", "forProvider")
		existing = append(existing, existingResource{Name: extName, ForProvider: forProvider})
	}

	return existing, nil
}

// unmanagedResources returns the external resources with no matching managed
// resource. A resource matches by external name and by every scope field.
func unmanagedResources(external []externalResource, existing []existingResource) []externalResource {
	var out []externalResource
	for _, res := range external {
		if !isManagedResource(res, existing) {
			out = append(out, res)
		}
	}

	return out
}

func isManagedResource(res externalResource, existing []existingResource) bool {
	for _, e := range existing {
		if e.Name == res.Name && forProviderHasScope(e.ForProvider, res.Scope) {
			return true
		}
	}

	return false
}

// forProviderHasScope reports whether forProvider has the scope's value for every
// scope field.
func forProviderHasScope(forProvider map[string]any, scope map[string]string) bool {
	for k, want := range scope {
		got, ok := forProvider[k]
		if !ok || fmt.Sprint(got) != want {
			return false
		}
	}

	return true
}

// generateManifests builds the managed resource objects that adopt unmanaged
// resources. Objects are returned unstructured so the same data can be rendered
// as YAML and applied to the cluster without a lossy parse-back round-trip.
func (c *discoverCmd) generateManifests(mrd *xpv1alpha1.ManagedResourceDefinition, unmanaged []externalResource) ([]*unstructured.Unstructured, error) {
	version, err := managedVersion(mrd)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(unmanaged, func(i, j int) bool {
		if unmanaged[i].Name != unmanaged[j].Name {
			return unmanaged[i].Name < unmanaged[j].Name
		}

		return scopeKey(unmanaged[i].Scope) < scopeKey(unmanaged[j].Scope)
	})

	objs := make([]*unstructured.Unstructured, 0, len(unmanaged))

	for _, res := range unmanaged {
		// Generate a stable name based on the external name and scope
		mrName := generateMRName(mrd.Spec.Names.Kind, res.Name, res.Scope)

		// Create the unstructured object
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(fmt.Sprintf("%s/%s", mrd.Spec.Group, version))
		obj.SetKind(mrd.Spec.Names.Kind)
		obj.SetName(mrName)
		obj.SetNamespace(c.Namespace)

		// Set annotations
		obj.SetAnnotations(map[string]string{
			"crossplane.io/external-name": res.Name,
		})

		// Set spec fields. The management policies must be set explicitly:
		// when they are absent the CRD defaults them to "*", which grants full
		// management and means deleting the managed resource deletes the
		// external one.
		spec := map[string]any{
			"managementPolicies": []any{"Observe", "LateInitialize"},
			"providerConfigRef": map[string]any{
				"name": c.ProviderConfig,
			},
		}

		// Scope fields locate the resource within its parent set.
		if len(res.Scope) > 0 {
			forProvider := make(map[string]any, len(res.Scope))
			for k, v := range res.Scope {
				forProvider[k] = v
			}

			spec["forProvider"] = forProvider
		}

		obj.Object["spec"] = spec

		objs = append(objs, obj)
	}

	return objs, nil
}

// generateMRName creates a stable name for a managed resource based on the external
// name and, for scoped resources, the scope. The same external name in two scopes
// yields two names.
func generateMRName(kind string, externalName string, scope map[string]string) string {
	h := fnv.New32a()
	h.Write([]byte(externalName))
	if len(scope) > 0 {
		h.Write([]byte("\x00" + scopeKey(scope)))
	}

	hash := fmt.Sprintf("%08x", h.Sum32())

	// Format: <kind-lower>-<hash>
	return fmt.Sprintf("%s-%s", strings.ToLower(kind), hash)
}

// scopeKey renders a scope as a sorted, comma-separated list of key=value pairs.
func scopeKey(scope map[string]string) string {
	keys := make([]string, 0, len(scope))
	for k := range scope {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+scope[k])
	}

	return strings.Join(parts, ",")
}
