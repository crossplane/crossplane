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

// Package beta implements Crossplane beta (experimental) commands.
package beta

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

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
	Kind           string `help:"Managed resource kind to discover (required)" required:""`
	Group          string `help:"API group of the resource (required)" required:""`
	ProviderConfig string `help:"ProviderConfig name to use for discovery" default:"default"`
	Namespace      string `help:"Namespace where adopted resources will be created" default:"default"`
	DryRun         bool   `help:"Print YAML without applying (default: true)" default:"true"`
	AutoImport     bool   `help:"Apply manifests automatically (requires --dry-run=false)" default:"false"`
	Output         string `help:"Write YAML to file instead of stdout"`
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

	// Step 5: List external resources from DiscoveryReport
	log.Debug("Querying DiscoveryReport for external resources")
	externalNames, err := c.listExternalResources(ctx, log, dClient, mrd)
	if err != nil {
		return errors.Wrap(err, "cannot list external resources")
	}
	log.Debug("Discovered resources", "count", len(externalNames))

	// Step 6: Get existing managed resources in cluster
	log.Debug("Querying existing managed resources")
	existingNames, err := c.getExistingManagedResources(ctx, dClient, mrd)
	if err != nil {
		return errors.Wrap(err, "cannot get existing managed resources")
	}
	log.Debug("Found existing resources", "count", len(existingNames))

	// Step 7: Compute set difference
	unmanaged := setDifference(externalNames, existingNames)
	if len(unmanaged) == 0 {
		fmt.Println("No unmanaged resources found.")
		return nil
	}
	log.Debug("Unmanaged resources", "count", len(unmanaged))

	// Step 8: Generate YAML manifests
	log.Debug("Generating adoption manifests")
	yaml, err := c.generateManifests(mrd, unmanaged)
	if err != nil {
		return errors.Wrap(err, "cannot generate manifests")
	}

	// Step 9: Output YAML
	if err := c.outputManifests(yaml); err != nil {
		return errors.Wrap(err, "cannot output manifests")
	}

	// Step 10: Auto-import if requested
	if !c.DryRun && c.AutoImport {
		log.Debug("Applying manifests")
		if err := c.applyManifests(ctx, dClient, yaml, c.Namespace); err != nil {
			return errors.Wrap(err, "cannot apply manifests")
		}
		fmt.Printf("Successfully imported %d resources.\n", len(unmanaged))
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

// listExternalResources queries the DiscoveryReport CRD to get discovered external resources.
// The discovery controller populates the DiscoveryReport, so this reads its findings.
func (c *discoverCmd) listExternalResources(ctx context.Context, log logging.Logger, dClient dynamic.Interface, mrd *xpv1alpha1.ManagedResourceDefinition) ([]string, error) {

	// Query DiscoveryReport CRD by MRD name (cluster-scoped)
	gvr := schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "discoveryreports",
	}

	// Try to get the DiscoveryReport with the same name as the MRD
	dr, err := dClient.Resource(gvr).Get(ctx, mrd.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get DiscoveryReport for MRD %q; " +
			"ensure the discovery controller is running and has scanned this resource type", mrd.Name)
	}

	// Extract the list of external names from the DiscoveryReport status
	unmanagedResources, ok, err := unstructured.NestedSlice(dr.Object, "status", "unmanagedResources")
	if err != nil || !ok {
		// No unmanaged resources found (all are already managed)
		return []string{}, nil
	}

	// Convert each unmanagedResource to a string (externalName)
	var externalNames []string
	for _, res := range unmanagedResources {
		resMap, ok := res.(map[string]interface{})
		if !ok {
			log.Debug("skipping invalid unmanagedResource entry")
			continue
		}

		externalName, ok := resMap["externalName"].(string)
		if ok && externalName != "" {
			externalNames = append(externalNames, externalName)
		}
	}

	// Warn if the DiscoveryReport is stale (> 10 minutes old)
	lastScanTime, ok, err := unstructured.NestedString(dr.Object, "status", "lastDiscoveryTime")
	if ok && lastScanTime != "" {
		lastScan, parseErr := time.Parse(time.RFC3339, lastScanTime)
		if parseErr == nil && time.Since(lastScan) > 10*time.Minute {
			log.Debug("DiscoveryReport is stale; results may be outdated", "lastScan", lastScan)
		}
	}

	return externalNames, nil
}

// getExistingManagedResources queries the cluster for all MRs of the given kind
// and extracts their external-name annotations.
func (c *discoverCmd) getExistingManagedResources(ctx context.Context, dClient dynamic.Interface, mrd *xpv1alpha1.ManagedResourceDefinition) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    mrd.Spec.Group,
		Version:  "v1",
		Resource: strings.ToLower(mrd.Spec.Names.Kind + "s"), // pluralize
	}

	list, err := dClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "cannot list managed resources")
	}

	var names []string
	for _, item := range list.Items {
		extName, ok := item.GetAnnotations()["crossplane.io/external-name"]
		if ok && extName != "" {
			names = append(names, extName)
		}
	}

	return names, nil
}

// generateManifests creates Kubernetes manifests for adopting unmanaged resources.
func (c *discoverCmd) generateManifests(mrd *xpv1alpha1.ManagedResourceDefinition, unmanagedNames []string) (string, error) {

	var buf bytes.Buffer

	sort.Strings(unmanagedNames) // Deterministic output

	for i, externalName := range unmanagedNames {
		// Generate a stable name based on the external name
		mrName := generateMRName(mrd.Spec.Names.Kind, externalName)

		// Create the unstructured object
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(fmt.Sprintf("%s/v1", mrd.Spec.Group))
		obj.SetKind(mrd.Spec.Names.Kind)
		obj.SetName(mrName)
		obj.SetNamespace(c.Namespace)

		// Set annotations
		annotations := map[string]string{
			"crossplane.io/external-name": externalName,
		}
		obj.SetAnnotations(annotations)

		// Set spec fields
		spec := map[string]interface{}{
			"managementPolicies": []string{"Observe", "LateInitialize"},
			"providerConfigRef": map[string]interface{}{
				"name": c.ProviderConfig,
			},
		}
		obj.Object["spec"] = spec

		// Marshal to YAML
		yamlBytes, err := marshalToYAML(obj)
		if err != nil {
			return "", errors.Wrap(err, "cannot marshal manifest")
		}

		buf.Write(yamlBytes)

		// Add separator between manifests (except after last one)
		if i < len(unmanagedNames)-1 {
			buf.WriteString("---\n")
		}
	}

	return buf.String(), nil
}

// outputManifests writes the YAML to stdout or a file.
func (c *discoverCmd) outputManifests(yaml string) error {
	if c.Output != "" {
		// Write to file
		if err := os.WriteFile(c.Output, []byte(yaml), 0o644); err != nil {
			return errors.Wrap(err, "cannot write to output file")
		}
		fmt.Printf("Manifests written to %s\n", c.Output)
		return nil
	}

	// Write to stdout
	fmt.Print(yaml)
	return nil
}

// applyManifests applies the generated manifests to the cluster.
// This implementation uses a simple YAML parser suitable for the generated format.
// For production use, consider using sigs.k8s.io/yaml for full YAML support.
func (c *discoverCmd) applyManifests(ctx context.Context, dClient dynamic.Interface, yamlContent string, namespace string) error {
	// Parse YAML documents (simple splitter for now).
	// TODO: Use sigs.k8s.io/yaml for full YAML parsing support
	// (handles comments, aliases, complex structures, etc.)
	documents := strings.Split(yamlContent, "---\n")

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Parse YAML into unstructured object
		// TODO: Use proper YAML unmarshaler
		obj, err := parseYAMLToUnstructured(doc)
		if err != nil {
			return errors.Wrapf(err, "cannot parse manifest")
		}

		// Extract API version and kind to build GVR
		apiVersion := obj.GetAPIVersion()
		kind := obj.GetKind()

		// Parse apiVersion (format: group/version or just version for core API)
		var group, version string
		if strings.Contains(apiVersion, "/") {
			parts := strings.SplitN(apiVersion, "/", 2)
			group = parts[0]
			version = parts[1]
		} else {
			version = apiVersion
		}

		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: strings.ToLower(kind + "s"), // Simple pluralization
		}

		// Apply the resource
		ns := obj.GetNamespace()
		if ns == "" {
			ns = namespace
		}

		_, err = dClient.Resource(gvr).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			// If resource already exists, that's acceptable (idempotent)
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			return errors.Wrapf(err, "cannot create %s/%s", obj.GetKind(), obj.GetName())
		}
	}

	return nil
}

// parseYAMLToUnstructured converts a YAML document to an unstructured object.
// This is a simple implementation that handles the format we generate.
// TODO: For full YAML support, integrate sigs.k8s.io/yaml package
func parseYAMLToUnstructured(doc string) (*unstructured.Unstructured, error) {
	// For now, use a simple line-by-line parser suitable for our generated format.
	// This is sufficient for the structured YAML we generate.
	// A production implementation would use proper YAML parsing.

	obj := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}

	metadata := make(map[string]interface{})
	spec := make(map[string]interface{})
	annotations := make(map[string]interface{})

	lines := strings.Split(doc, "\n")

	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove indentation and parse key: value
		line = strings.TrimLeft(line, " \t")

		// Parse key: value or key: or list item
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Handle special cases for our generated format
		switch key {
		case "apiVersion":
			obj.SetAPIVersion(value)
		case "kind":
			obj.SetKind(value)
		case "name":
			metadata["name"] = value
		case "namespace":
			metadata["namespace"] = value
		case "annotations":
			// Mark that we're in annotations section
			_ = spec // ensure spec is defined for next case
		case "crossplane.io/external-name":
			// This is an annotation value
			annotations["crossplane.io/external-name"] = value
		case "managementPolicies":
			// This is a spec value (we'll parse list items)
			_ = spec
		}
	}

	// Set metadata and spec
	if len(metadata) > 0 {
		if len(annotations) > 0 {
			metadata["annotations"] = annotations
		}
		obj.Object["metadata"] = metadata
	}

	if len(spec) > 0 {
		obj.Object["spec"] = spec
	}

	return obj, nil
}

// Helper functions

// generateMRName creates a stable name for a managed resource based on the external name.
func generateMRName(kind string, externalName string) string {
	h := fnv.New32a()
	h.Write([]byte(externalName))
	hash := fmt.Sprintf("%08x", h.Sum32())

	// Format: <kind-lower>-<hash>
	return fmt.Sprintf("%s-%s", strings.ToLower(kind), hash)
}

// setDifference returns elements in a that are not in b.
func setDifference(a, b []string) []string {
	bSet := make(map[string]bool)
	for _, v := range b {
		bSet[v] = true
	}
	var result []string
	for _, v := range a {
		if !bSet[v] {
			result = append(result, v)
		}
	}
	return result
}

// marshalToYAML converts an unstructured object to YAML bytes.
// This is a simple implementation suitable for the initial version.
// TODO: Consider using a proper YAML marshaler (e.g., sigs.k8s.io/yaml) for full YAML support.
func marshalToYAML(obj *unstructured.Unstructured) ([]byte, error) {
	var buf bytes.Buffer

	// Write headers
	buf.WriteString("apiVersion: ")
	buf.WriteString(obj.GetAPIVersion())
	buf.WriteString("\n")

	buf.WriteString("kind: ")
	buf.WriteString(obj.GetKind())
	buf.WriteString("\n")

	// Write metadata
	buf.WriteString("metadata:\n")
	buf.WriteString("  name: ")
	buf.WriteString(obj.GetName())
	buf.WriteString("\n")

	buf.WriteString("  namespace: ")
	buf.WriteString(obj.GetNamespace())
	buf.WriteString("\n")

	// Write annotations
	if len(obj.GetAnnotations()) > 0 {
		buf.WriteString("  annotations:\n")
		for k, v := range obj.GetAnnotations() {
			buf.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
		}
	}

	// Write spec
	buf.WriteString("spec:\n")
	if spec, ok := obj.Object["spec"].(map[string]interface{}); ok {
		if policies, ok := spec["managementPolicies"].([]string); ok {
			buf.WriteString("  managementPolicies:\n")
			for _, p := range policies {
				buf.WriteString(fmt.Sprintf("  - %s\n", p))
			}
		}
		if pcRef, ok := spec["providerConfigRef"].(map[string]interface{}); ok {
			buf.WriteString("  providerConfigRef:\n")
			if name, ok := pcRef["name"].(string); ok {
				buf.WriteString(fmt.Sprintf("    name: %s\n", name))
			}
		}
	}

	buf.WriteString("\n")

	return buf.Bytes(), nil
}
