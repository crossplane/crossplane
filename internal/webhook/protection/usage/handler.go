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

// Package usage contains the Handler for the usage webhook.
package usage

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	xpmeta "github.com/crossplane/crossplane-runtime/pkg/meta"

	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
	"github.com/crossplane/crossplane/internal/protection"
	"github.com/crossplane/crossplane/internal/protection/usage"
)

// Error strings.
const (
	errFmtUnexpectedOp = "unexpected operation %q, expected \"DELETE\""
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, f Finder, options controller.Options) {
	h := NewHandler(xpunstructured.NewClient(mgr.GetClient()), f, WithLogger(options.Logger.WithValues("webhook", "no-usages")))
	mgr.GetWebhookServer().Register("/validate-no-usages", &webhook.Admission{Handler: h})
}

// A Finder finds usages.
type Finder interface {
	FindUsageOf(ctx context.Context, o usage.Object) ([]protection.Usage, error)
}

// Handler implements the admission Handler for Composition.
type Handler struct {
	client   client.Client
	resource Finder
	log      logging.Logger
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
func NewHandler(client client.Client, f Finder, opts ...HandlerOption) *Handler {
	h := &Handler{
		client:   client,
		resource: f,
		log:      logging.NewNopLogger(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Handle handles the admission request, validating there is no usage for the
// resource being deleted.
func (h *Handler) Handle(ctx context.Context, request admission.Request) admission.Response {
	switch request.Operation {
	case admissionv1.Create, admissionv1.Update, admissionv1.Connect:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, request.Operation))
	case admissionv1.Delete:
		u := &unstructured.Unstructured{}
		if err := u.UnmarshalJSON(request.OldObject.Raw); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		opts := &metav1.DeleteOptions{}
		if err := yaml.Unmarshal(request.Options.Raw, opts); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		log := h.log.WithValues(
			"apiVersion", u.GetAPIVersion(),
			"kind", u.GetKind(),
			"name", u.GetName(),
			"policy", opts.PropagationPolicy,
		)
		if u.GetNamespace() != "" {
			log = log.WithValues("namespace", u.GetNamespace())
		}

		log.Debug("Validating no usages")

		usages, err := h.resource.FindUsageOf(ctx, u)
		if err != nil {
			log.Debug("Error when getting usages", "err", err)
			return admission.Errored(http.StatusInternalServerError, err)
		}

		if len(usages) == 0 {
			log.Debug("No usages found, deletion allowed")
			return admission.Allowed("")
		}

		msg := inUseMessage(usages)
		log.Debug("Usages found, deletion not allowed", "msg", msg)

		// If the resource is being deleted, we want to record the first deletion attempt
		// so that we can track whether a deletion was attempted at least once.
		policy := string(ptr.Deref(opts.PropagationPolicy, metav1.DeletePropagationBackground))
		if u.GetAnnotations() == nil || u.GetAnnotations()[protection.AnnotationKeyDeletionAttempt] != policy {
			orig := u.DeepCopy()
			xpmeta.AddAnnotations(u, map[string]string{protection.AnnotationKeyDeletionAttempt: policy})
			// Patch the resource to add the deletion attempt annotation
			if err := h.client.Patch(ctx, u, client.MergeFrom(orig)); err != nil {
				log.Debug("Error when patching the resource to add the deletion attempt annotation", "err", err)
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}

		return admission.Response{
			AdmissionResponse: admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:   int32(http.StatusConflict),
					Reason: metav1.StatusReason(msg),
				},
			},
		}
	default:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, request.Operation))
	}
}

func inUseMessage(u []protection.Usage) string {
	first := u[0]
	by := first.GetUsedBy()
	id := fmt.Sprintf("%q", first.GetName())
	if first.GetNamespace() != "" {
		id = fmt.Sprintf("%q (in namespace %q)", first.GetName(), first.GetNamespace())
	}
	if by != nil {
		return fmt.Sprintf("This resource is in-use by %d usage(s), including the %T %s by resource %s/%s.", len(u), first, id, by.Kind, by.ResourceRef.Name)
	}
	if r := ptr.Deref(first.GetReason(), ""); r != "" {
		return fmt.Sprintf("This resource is in-use by %d usage(s), including the %T %s with reason: %q.", len(u), first, id, r)
	}
	// Either spec.by or spec.reason should be set, which we enforce with a CEL
	// rule. This is just a fallback.
	return fmt.Sprintf("This resource is in-use by %d usage(s), including the %T %s.", len(u), first, id)
}
