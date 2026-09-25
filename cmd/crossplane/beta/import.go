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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

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

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "cannot create discovery client")
	}

	// Step 3: Look up MRD for the given kind/group
	log.Debug("Looking up ManagedResourceDefinition", "kind", c.Kind, "group", c.Group)
	mrd, err := c.getMRD(ctx, dClient, discoveryClient)
	if err != nil {
		return errors.Wrap(err, "cannot get ManagedResourceDefinition")
	}

	log.Debug("Found MRD", "name", mrd.Name)

	// Step 4: Fetch ProviderConfig
	log.Debug("Fetching ProviderConfig", "name", c.ProviderConfig, "group", mrd.Spec.Group)
	pc, err := c.getProviderConfig(ctx, dClient, mrd, c.ProviderConfig)
	if err != nil {
		return errors.Wrap(err, "cannot get ProviderConfig")
	}

	// Step 5: List external resources (would call provider's ExternalLister)
	log.Debug("Discovering external resources")
	externalNames, err := c.listExternalResources(ctx, log, mrd, pc)
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
	yaml, err := c.generateManifests(mrd, pc, unmanaged)
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
func (c *discoverCmd) getMRD(ctx context.Context, dClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) (*xpv1alpha1.ManagedResourceDefinition, error) {
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

// getProviderConfig fetches the named ProviderConfig from the cluster.
// TODO: Implement actual ProviderConfig fetching via dynamic client.
// This is a design placeholder - providers have different ProviderConfig types.
func (c *discoverCmd) getProviderConfig(ctx context.Context, dClient dynamic.Interface, mrd *xpv1alpha1.ManagedResourceDefinition, pcName string) (resource.ProviderConfig, error) {
	_ = ctx
	_ = dClient
	_ = mrd
	_ = pcName

	// Placeholder: In real implementation, would:
	// 1. Determine ProviderConfig GVR from MRD
	// 2. Query by name in default/specified namespace
	// 3. Return the actual ProviderConfig object
	return nil, errors.New("ProviderConfig fetching not yet implemented - design decision needed on provider-specific types")
}

// listExternalResources calls the provider's ExternalLister (if implemented).
// TODO: Implement provider instantiation and ExternalLister integration.
// This is the key integration point with the ExternalLister interface.
func (c *discoverCmd) listExternalResources(ctx context.Context, log logging.Logger, mrd *xpv1alpha1.ManagedResourceDefinition, pc resource.ProviderConfig) ([]string, error) {
	_ = ctx
	_ = log
	_ = mrd
	_ = pc

	// In the real implementation:
	// 1. Instantiate the provider's ExternalClient
	// 2. Type-assert to check if it implements ExternalLister
	// 3. Call List() repeatedly until NextPageToken is empty
	// 4. Collect and return all external names

	return []string{}, errors.New("ExternalLister integration not yet implemented - awaiting design decision on provider instantiation")
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
func (c *discoverCmd) generateManifests(mrd *xpv1alpha1.ManagedResourceDefinition, pc resource.ProviderConfig, unmanagedNames []string) (string, error) {
	_ = pc // Would be used to get ProviderConfig name

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
// TODO: Implement manifest application using dynamic client.
func (c *discoverCmd) applyManifests(ctx context.Context, dClient dynamic.Interface, yaml string, namespace string) error {
	_ = ctx
	_ = dClient
	_ = yaml
	_ = namespace

	// TODO: Parse YAML documents and apply each using dynamic client
	// This would typically:
	// 1. Parse YAML string into unstructured objects
	// 2. For each object, call dClient.Resource(gvr).Namespace(...).Create(...)
	// 3. Handle conflicts/existing resources gracefully
	return errors.New("applyManifests not yet implemented - awaiting design decision on YAML parsing library")
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
