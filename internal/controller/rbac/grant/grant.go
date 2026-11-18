/*
Copyright 2026 The Crossplane Authors.

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

// Package grant discovers and validates CRDs used to render RBAC rules.
package grant

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"

	apis "github.com/crossplane/crossplane/apis/v2"
)

//nolint:gochecknoglobals // Derived once from Crossplane's API registrations.
var crossplaneGroups = crossplaneAPIGroups()

func crossplaneAPIGroups() map[string]bool {
	s := runtime.NewScheme()
	utilruntime.Must(apis.AddToScheme(s))

	groups := make(map[string]bool)
	for gvk := range s.AllKnownTypes() {
		groups[gvk.Group] = true
	}

	return groups
}

// Permitted reports whether a group and plural are valid for a generated RBAC
// rule. Kubernetes and Crossplane API groups are excluded. It doesn't check
// that a custom resource exists or who owns it.
func Permitted(group, plural string) bool {
	// Apply the API server's CRD naming rules. CRD groups must contain a dot,
	// which also excludes built-in groups such as apps, batch, and policy.
	if len(validation.IsDNS1035Label(plural)) > 0 ||
		len(validation.IsDNS1123Subdomain(group)) > 0 || !strings.Contains(group, ".") {
		return false
	}

	// Groups under k8s.io are reserved for Kubernetes APIs.
	if group == "k8s.io" || strings.HasSuffix(group, ".k8s.io") {
		return false
	}

	// Match Crossplane's groups exactly: providers define APIs
	// in other groups under crossplane.io, such as helm.crossplane.io.
	return !crossplaneGroups[group]
}
