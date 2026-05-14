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

package usage

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// Error strings for the validating handler.
const (
	errDecodeUsage                  = "cannot decode Usage object"
	errFmtNamespacedUsageOfCluster  = "namespaced Usage %q in namespace %q references cluster-scoped %s/%s; use a ClusterUsage instead"
	errFmtResolveResourceScope      = "cannot resolve scope of referenced resource %s/%s"
	errFmtNamespacedUsageOfClusterN = "namespaced Usage %q in namespace %q references cluster-scoped %s/%s %q; use a ClusterUsage instead"
)

// A ValidatingHandler validates Usage objects on create and update, rejecting
// namespaced Usages that reference cluster-scoped resources.
type ValidatingHandler struct {
	mapper meta.RESTMapper
	log    logging.Logger
}

// ValidatingHandlerOption configures a ValidatingHandler.
type ValidatingHandlerOption func(*ValidatingHandler)

// WithValidatingLogger configures the logger for the ValidatingHandler.
func WithValidatingLogger(l logging.Logger) ValidatingHandlerOption {
	return func(h *ValidatingHandler) {
		h.log = l
	}
}

// NewValidatingHandler returns a new ValidatingHandler.
func NewValidatingHandler(mapper meta.RESTMapper, opts ...ValidatingHandlerOption) *ValidatingHandler {
	h := &ValidatingHandler{
		mapper: mapper,
		log:    logging.NewNopLogger(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Handle validates the admission request, rejecting namespaced Usages that
// reference cluster-scoped resources.
func (h *ValidatingHandler) Handle(_ context.Context, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
		// Validate create and update operations.
	case admissionv1.Delete, admissionv1.Connect:
		return admission.Allowed("")
	default:
		return admission.Allowed("")
	}

	u := &unstructured.Unstructured{}
	if err := u.UnmarshalJSON(request.Object.Raw); err != nil {
		return admission.Errored(http.StatusBadRequest, errors.Wrap(err, errDecodeUsage))
	}

	// Only namespaced Usages can have this problem.
	if u.GetNamespace() == "" {
		return admission.Allowed("")
	}

	apiVersion, _, _ := unstructured.NestedString(u.Object, "spec", "of", "apiVersion")
	kind, _, _ := unstructured.NestedString(u.Object, "spec", "of", "kind")

	if apiVersion == "" || kind == "" {
		return admission.Allowed("")
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		// Let the reconciler handle unparseable API versions.
		return admission.Allowed("")
	}

	mapping, err := h.mapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
	if err != nil {
		// If the GVK cannot be resolved (e.g. CRD not yet installed), let the
		// reconciler handle it.
		h.log.Debug(fmt.Sprintf(errFmtResolveResourceScope, apiVersion, kind), "error", err)
		return admission.Allowed("")
	}

	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		return admission.Allowed("")
	}

	// The referenced resource is cluster-scoped but the Usage is namespaced.
	name, _, _ := unstructured.NestedString(u.Object, "spec", "of", "resourceRef", "name")
	if name != "" {
		return admission.Denied(fmt.Sprintf(errFmtNamespacedUsageOfClusterN, u.GetName(), u.GetNamespace(), apiVersion, kind, name))
	}

	return admission.Denied(fmt.Sprintf(errFmtNamespacedUsageOfCluster, u.GetName(), u.GetNamespace(), apiVersion, kind))
}
