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

package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	xpmeta "github.com/crossplane/crossplane-runtime/v2/pkg/meta"

	"github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

// Error strings.
const (
	errFmtUnexpectedScopeOp = "unexpected operation %q, expected \"CREATE\" or \"UPDATE\""
	errFmtClusterScopedOf   = "a namespaced Usage cannot protect the cluster-scoped resource kind %s - use a ClusterUsage instead"
)

// A ScopeHandler validates that a namespaced Usage only uses namespaced
// resources. A namespaced Usage can't block deletion of a cluster-scoped
// resource, so we reject it at admission time. This gives faster feedback
// than the error condition the Usage controller would set.
type ScopeHandler struct {
	mapper meta.RESTMapper
	log    logging.Logger
}

// ScopeHandlerOption is used to configure the ScopeHandler.
type ScopeHandlerOption func(*ScopeHandler)

// WithScopeLogger configures the logger for the ScopeHandler.
func WithScopeLogger(l logging.Logger) ScopeHandlerOption {
	return func(h *ScopeHandler) {
		h.log = l
	}
}

// NewScopeHandler returns a new ScopeHandler.
func NewScopeHandler(mapper meta.RESTMapper, opts ...ScopeHandlerOption) *ScopeHandler {
	h := &ScopeHandler{
		mapper: mapper,
		log:    logging.NewNopLogger(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Handle handles the admission request, validating that a namespaced Usage
// doesn't use a cluster-scoped resource.
func (h *ScopeHandler) Handle(_ context.Context, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
		// The only operations we validate.
	case admissionv1.Delete, admissionv1.Connect:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedScopeOp, request.Operation))
	default:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedScopeOp, request.Operation))
	}

	u := &v1beta1.Usage{}
	if err := json.Unmarshal(request.Object.Raw, u); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Never deny an update to a Usage that's being deleted. Denying it would
	// block removing the Usage's finalizers - notably the one the Usage
	// controller adds - leaving the Usage stuck terminating. A Usage that's
	// being deleted can't begin to protect anything, so there's nothing to
	// validate.
	if xpmeta.WasDeleted(u) {
		return admission.Allowed("")
	}

	// Only deny an update that changes what resource the Usage uses. A Usage
	// that uses a cluster-scoped resource might predate this webhook. Denying
	// every update would make such a Usage impossible to mutate or delete -
	// even removing a finalizer is an update. The Usage controller rejects
	// what we allow here; this webhook only prevents introducing new usages
	// of cluster-scoped resources.
	if request.Operation == admissionv1.Update {
		old := &v1beta1.Usage{}
		if err := json.Unmarshal(request.OldObject.Raw, old); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if u.Spec.Of.APIVersion == old.Spec.Of.APIVersion && u.Spec.Of.Kind == old.Spec.Of.Kind {
			return admission.Allowed("")
		}
	}

	gv, err := schema.ParseGroupVersion(u.Spec.Of.APIVersion)
	if err != nil {
		// The Usage controller returns an error for an APIVersion it can't
		// parse. Allow the Usage so the controller can surface it.
		return admission.Allowed("")
	}

	gvk := gv.WithKind(u.Spec.Of.Kind)

	m, err := h.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		// We can't determine the scope of a kind we don't know about, for
		// example a kind whose CRD isn't installed yet. Allow the Usage - the
		// Usage controller will reject it if its used resource turns out to
		// be cluster-scoped.
		h.log.Debug("Cannot determine scope of the used resource. Allowing the request.", "gvk", gvk, "error", err)
		return admission.Allowed("")
	}

	if m.Scope.Name() == meta.RESTScopeNameRoot {
		return admission.Denied(fmt.Sprintf(errFmtClusterScopedOf, gvk))
	}

	return admission.Allowed("")
}
