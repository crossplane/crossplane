/*
Copyright 2024 The Crossplane Authors.

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

// Package provider contains the Handler for the provider deletion webhook.
package provider

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"

	v1alpha1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
)

// Error strings.
const (
	errFmtUnexpectedOp        = "unexpected operation %q, expected \"DELETE\""
	errFmtGetProviderRevision = "cannot get provider revision %q"
	errFmtGetMRD              = "cannot get ManagedResourceDefinition %q"
	errFmtGetCRD              = "cannot get CustomResourceDefinition %q"
	errFmtListCRs             = "cannot list custom resources for CRD %q"
	errCRsExist               = "Cannot delete provider: custom resources still exist. Please delete all custom resources before deleting the provider."
	errUnmarshalProvider      = "cannot unmarshal provider from admission request"
	errExtractMRDNames        = "cannot extract CRD names from MRDs"
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, options controller.Options) {
	h := NewHandler(mgr.GetClient(), WithLogger(options.Logger.WithValues("webhook", "provider-deletion")))
	mgr.GetWebhookServer().Register("/validate-provider-deletion", &webhook.Admission{Handler: h})
}

// Handler implements the admission Handler for Provider deletion.
type Handler struct {
	client client.Client
	log    logging.Logger
}

// HandlerOption is used to configure the Handler.
type HandlerOption func(*Handler)

// WithLogger configures the logger for the Handler.
func WithLogger(l logging.Logger) HandlerOption {
	return func(h *Handler) {
		h.log = l
	}
}

// NewHandler returns a new Handler.
func NewHandler(client client.Client, opts ...HandlerOption) *Handler {
	h := &Handler{
		client: client,
		log:    logging.NewNopLogger(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Handle handles the admission request, validating that no custom resources
// exist for the provider's CRDs before allowing deletion.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update, admissionv1.Connect:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, request.Operation))
	case admissionv1.Delete:
		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(request.OldObject.Raw); err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrap(err, errUnmarshalProvider))
		}

		provider := &v1.Provider{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, provider); err != nil {
			return admission.Errored(http.StatusBadRequest, errors.Wrap(err, errUnmarshalProvider))
		}

		log := h.log.WithValues(
			"provider", provider.GetName(),
			"currentRevision", provider.Status.CurrentRevision,
		)

		log.Debug("Validating provider deletion")

		// Check if provider has a current revision
		if provider.Status.CurrentRevision == "" {
			log.Debug("Provider has no current revision, allowing deletion")
			return admission.Allowed("")
		}

		// Get the current provider revision
		revision := &v1.ProviderRevision{}
		if err := h.client.Get(ctx, types.NamespacedName{Name: provider.Status.CurrentRevision}, revision); err != nil {
			if kerrors.IsNotFound(err) {
				log.Debug("Provider revision not found, allowing deletion", "err", err)
				// If revision is gone, allow provider deletion
				return admission.Allowed("")
			}
			log.Debug("Error getting provider revision", "err", err)
			return admission.Errored(http.StatusInternalServerError, errors.Wrapf(err, errFmtGetProviderRevision, provider.Status.CurrentRevision))
		}

		// Extract CRD names from MRDs in the revision's ObjectRefs
		// This checks if MRDs are Active and only returns CRD names for Active MRDs
		crdNames, err := h.extractActiveMRDNames(ctx, revision.Status.ObjectRefs)
		if err != nil {
			log.Debug("Error extracting active MRD names", "err", err)
			return admission.Errored(http.StatusInternalServerError, errors.Wrap(err, errExtractMRDNames))
		}
		if len(crdNames) == 0 {
			log.Debug("No active MRDs found in provider revision, allowing deletion")
			return admission.Allowed("")
		}

		log.Debug("Checking for existing custom resources", "crdCount", len(crdNames))

		// Check each CRD for existing custom resources
		// Return early as soon as we find any instance to minimize API calls
		for _, crdName := range crdNames {
			hasInstances, err := h.hasCustomResources(ctx, crdName)
			if err != nil {
				log.Debug("Error checking for custom resources", "crd", crdName, "err", err)
				return admission.Errored(http.StatusInternalServerError, errors.Wrapf(err, errFmtListCRs, crdName))
			}
			if hasInstances {
				log.Debug("Custom resources found, blocking deletion", "crd", crdName)
				return admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:   int32(http.StatusConflict),
							Reason: metav1.StatusReason(errCRsExist),
						},
					},
				}
			}
		}

		log.Debug("No custom resources found, allowing deletion")
		return admission.Allowed("")
	default:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, request.Operation))
	}
}

// extractActiveMRDNames extracts CRD names from Active MRDs in ObjectRefs.
// Providers track their CRDs via ManagedResourceDefinitions (MRDs).
// This function:
// 1. Finds MRD references in ObjectRefs
// 2. Fetches each MRD resource
// 3. Checks if the MRD is Active (spec.state == "Active")
// 4. Only returns CRD names for Active MRDs (inactive MRDs don't have CRDs).
func (h *Handler) extractActiveMRDNames(ctx context.Context, refs []xpv1.TypedReference) ([]string, error) {
	crdNames := []string{}
	for _, ref := range refs {
		if ref.Kind != v1alpha1.ManagedResourceDefinitionKind {
			continue
		}

		// Fetch the MRD to check its state
		mrd := &v1alpha1.ManagedResourceDefinition{}
		if err := h.client.Get(ctx, types.NamespacedName{Name: ref.Name}, mrd); err != nil {
			if kerrors.IsNotFound(err) {
				// MRD doesn't exist anymore, skip it (no CRD to check)
				continue
			}
			return nil, errors.Wrapf(err, errFmtGetMRD, ref.Name)
		}

		// Only include Active MRDs - Inactive MRDs don't create CRDs
		if mrd.Spec.State.IsActive() {
			crdNames = append(crdNames, mrd.Name)
		}
	}
	return crdNames, nil
}

// hasCustomResources checks if any custom resource instances exist for a given CRD.
// It returns early after finding the first instance to minimize API calls.
func (h *Handler) hasCustomResources(ctx context.Context, crdName string) (bool, error) {
	// Get the CRD to extract GVK information
	crd := &extv1.CustomResourceDefinition{}
	if err := h.client.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
		if kerrors.IsNotFound(err) {
			// CRD doesn't exist, so no instances can exist
			return false, nil
		}
		return false, errors.Wrapf(err, errFmtGetCRD, crdName)
	}

	// Find the storage version
	var storageVersion string
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			storageVersion = version.Name
			break
		}
	}

	// Build GVK for listing
	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: storageVersion,
		Kind:    crd.Spec.Names.ListKind,
	}

	// List custom resources using unstructured with Limit(1) to check existence
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := h.client.List(ctx, list, client.Limit(1)); err != nil {
		return false, errors.Wrapf(err, errFmtListCRs, crdName)
	}

	return len(list.Items) > 0, nil
}
