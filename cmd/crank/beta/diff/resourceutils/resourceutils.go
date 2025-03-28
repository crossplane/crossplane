// Package resourceutils provides utility functions for working with Kubernetes resources.
package resourceutils

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PluralizeResourceName converts a singular resource kind to its plural resource name form.
// It handles common irregular pluralizations in Kubernetes resource types.
func PluralizeResourceName(kind string) string {
	// Convert to lowercase for consistent handling
	lowerKind := strings.ToLower(kind)

	// Handle irregular plurals
	switch lowerKind {
	case "ingress":
		return "ingresses"
	case "endpoints":
		return "endpoints" // Already plural, no change
	case "configmap":
		return "configmaps"
	case "policy":
		return "policies"
	case "gateway":
		return "gateways"
	case "proxy":
		return "proxies"
	case "index":
		return "indices"
	case "matrix":
		return "matrices"
	case "status":
		return "statuses"
	case "patch":
		return "patches"
	case "address":
		return "addresses"
	case "discovery":
		return "discoveries"
	}

	// Default pluralization by adding 's'
	return lowerKind + "s"
}

// KindToResource converts a GVK to a GVR by properly pluralizing the kind
func KindToResource(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: PluralizeResourceName(gvk.Kind),
	}
}

// GuessCRDName creates a CRD name from a GVK using the conventional pattern (plural.group)
func GuessCRDName(gvk schema.GroupVersionKind) string {
	plural := PluralizeResourceName(gvk.Kind)
	return fmt.Sprintf("%s.%s", plural, gvk.Group)
}
