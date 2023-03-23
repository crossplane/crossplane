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

package composition

import (
	"context"
	"errors"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Handler implements the admission handler for Composition.
// TODO(lsviben): switch to using CustomValidatior when https://github.com/kubernetes-sigs/controller-runtime/issues/1896 is resolved.
type Handler struct {
	validator *CustomValidator
	decoder   *admission.Decoder
}

// InjectDecoder injects the decoder.
func (h *Handler) InjectDecoder(decoder *admission.Decoder) error {
	h.decoder = decoder
	return nil
}

// NewHandler returns a new handler using the given validator.
func NewHandler(v *CustomValidator) *Handler {
	return &Handler{
		validator: v,
	}
}

// Handle handles the admission request, validating the Composition.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	c := &v1.Composition{}

	switch request.Operation {
	case admissionv1.Create, admissionv1.Update:
		if err := h.decoder.Decode(request, c); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	case admissionv1.Delete:
		return admission.Allowed("")
	case admissionv1.Connect:
		return admission.Errored(http.StatusBadRequest, errors.New("unexpected operation"))
	default:
		return admission.Errored(http.StatusBadRequest, errors.New("unexpected operation"))
	}

	warns, err := h.validator.Validate(ctx, c)
	if err != nil {
		var apiStatus apierrors.APIStatus
		if errors.As(err, &apiStatus) {
			return validationResponseFromStatus(false, apiStatus.Status(), warns)
		}
		return admission.Denied(err.Error()).WithWarnings(warns...)
	}
	return admission.Allowed("").WithWarnings(warns...)
}

func validationResponseFromStatus(allowed bool, status metav1.Status, warns []string) admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: allowed,
			Result:  &status,
		},
	}.WithWarnings(warns...)
	return resp
}
