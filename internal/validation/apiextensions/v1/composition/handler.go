/*
Copyright 2023 The Crossplane Authors.

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

// Package composition contains internal logic linked to the validation of the v1.Composition type.
package composition

import (
	"context"
	"errors"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/pkg/validation/apiextensions/v1/composition"
)

// handler implements the admission handler for Composition.
type handler struct {
	validator *composition.Validator
	decoder   *admission.Decoder
}

// InjectDecoder injects the decoder.
func (h *handler) InjectDecoder(decoder *admission.Decoder) error {
	h.decoder = decoder
	return nil
}

// newHandler returns a new handler using the given validator.
func newHandler() admission.Handler {
	return &handler{
		validator: &composition.Validator{},
	}
}

// SetupWebhookWithManager sets up the webhook with the manager.
//
// TODO(lsviben): switch to using admission.CustomValidator when https://github.com/kubernetes-sigs/controller-runtime/issues/1896 is resolved.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	mgr.GetWebhookServer().Register(v1.CompositionValidatingWebhookPath,
		&webhook.Admission{Handler: newHandler()})
	return nil
}

// Handle handles the admission request, validating the Composition.
func (h *handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
		c := &v1.Composition{}
		if err := h.decoder.Decode(request, c); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		warns, err := h.validator.Validate(ctx, c)
		if err == nil {
			return admission.Allowed("").WithWarnings(warns...)
		}
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			return validationResponseFromStatus(false, apiStatus.Status()).WithWarnings(warns...)
		}
		return admission.Denied(err.Error()).WithWarnings(warns...)
	case admissionv1.Delete:
		return admission.Allowed("")
	case admissionv1.Connect:
		return admission.Errored(http.StatusBadRequest, errors.New("unexpected operation"))
	default:
		return admission.Errored(http.StatusBadRequest, errors.New("unexpected operation"))
	}
}

func validationResponseFromStatus(allowed bool, status metav1.Status) admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result:  &status,
		},
	}
	return resp
}
