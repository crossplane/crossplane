package composition

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Handler implements the admission handler for Composition.
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
	case admissionv1.Delete, admissionv1.Connect:
		return admission.Allowed("")
	}

	warns, err := h.validator.Validate(ctx, c)
	if err != nil {
		return admission.Denied(err.Error()).WithWarnings(warns...)
	}
	return admission.Allowed("").WithWarnings(warns...)
}
