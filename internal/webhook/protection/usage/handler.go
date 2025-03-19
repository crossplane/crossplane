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

	legacy "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/apis/protection/v1beta1"
	"github.com/crossplane/crossplane/internal/protection"
	xpunstructured "github.com/crossplane/crossplane/internal/xresource/unstructured"
)

// Error strings.
const (
	errFmtUnexpectedOp = "unexpected operation %q, expected \"DELETE\""
)

// SetupWebhookWithManager sets up the webhook with the manager.
func SetupWebhookWithManager(mgr ctrl.Manager, options controller.Options) error {
	fi := mgr.GetFieldIndexer()

	if err := fi.IndexField(context.Background(), &v1beta1.Usage{}, protection.InUseIndexKey, func(obj client.Object) []string {
		u := obj.(*v1beta1.Usage) //nolint:forcetypeassert // Will always be a Usage.
		if u.Spec.Of.ResourceRef == nil || len(u.Spec.Of.ResourceRef.Name) == 0 {
			return []string{}
		}
		return []string{protection.InUseIndexValue(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name)}
	}); err != nil {
		return err
	}

	if err := fi.IndexField(context.Background(), &v1beta1.ClusterUsage{}, protection.InUseIndexKey, func(obj client.Object) []string {
		u := obj.(*v1beta1.ClusterUsage) //nolint:forcetypeassert // Will always be a ClusterUsage.
		if u.Spec.Of.ResourceRef == nil || len(u.Spec.Of.ResourceRef.Name) == 0 {
			return []string{}
		}
		return []string{protection.InUseIndexValue(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name)}
	}); err != nil {
		return err
	}

	//nolint:staticcheck // Usage is deprecated but we still need to support it.
	if err := fi.IndexField(context.Background(), &legacy.Usage{}, protection.InUseIndexKey, func(obj client.Object) []string {
		u := obj.(*legacy.Usage) //nolint:forcetypeassert,staticcheck // This'll always be a Usage. Which, as above, is deprecated.
		if u.Spec.Of.ResourceRef == nil || len(u.Spec.Of.ResourceRef.Name) == 0 {
			return []string{}
		}
		return []string{protection.InUseIndexValue(u.Spec.Of.APIVersion, u.Spec.Of.Kind, u.Spec.Of.ResourceRef.Name)}
	}); err != nil {
		return err
	}

	mgr.GetWebhookServer().Register("/validate-no-usages",
		&webhook.Admission{Handler: NewHandler(
			xpunstructured.NewClient(mgr.GetClient()),
			WithLogger(options.Logger.WithValues("webhook", "no-usages")),
		)})
	return nil
}

// Handler implements the admission Handler for Composition.
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
		return h.validateNoUsages(ctx, u, opts)
	default:
		return admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, request.Operation))
	}
}

func (h *Handler) validateNoUsages(ctx context.Context, u *unstructured.Unstructured, opts *metav1.DeleteOptions) admission.Response {
	h.log.Debug("Validating no usages", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "policy", opts.PropagationPolicy)

	usages := make([]protection.Usage, 0)

	ul := &v1beta1.UsageList{}
	if err := h.client.List(ctx, ul, client.MatchingFields{protection.InUseIndexKey: protection.InUseIndexValue(u.GetAPIVersion(), u.GetKind(), u.GetName())}); err != nil {
		h.log.Debug("Error when getting usages", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "type", v1beta1.UsageGroupVersionKind, "err", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	for _, u := range ul.Items {
		usages = append(usages, &u)
	}

	cul := &v1beta1.ClusterUsageList{}
	if err := h.client.List(ctx, cul, client.MatchingFields{protection.InUseIndexKey: protection.InUseIndexValue(u.GetAPIVersion(), u.GetKind(), u.GetName())}); err != nil {
		h.log.Debug("Error when getting usages", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "type", v1beta1.ClusterUsageGroupVersionKind, "err", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	for _, u := range cul.Items {
		usages = append(usages, &u)
	}

	lul := &legacy.UsageList{} //nolint:staticcheck // It's deprecated but we still need to support it.
	if err := h.client.List(ctx, lul, client.MatchingFields{protection.InUseIndexKey: protection.InUseIndexValue(u.GetAPIVersion(), u.GetKind(), u.GetName())}); err != nil {
		h.log.Debug("Error when getting usages", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "type", legacy.UsageGroupVersionKind, "err", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	for _, u := range lul.Items {
		usages = append(usages, &u)
	}

	if len(usages) == 0 {
		h.log.Debug("No usage found, deletion allowed", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName())
		return admission.Allowed("")
	}

	msg := inUseMessage(usages)
	h.log.Debug("Usage found, deletion not allowed", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "msg", msg)

	// Use the default propagation policy if not provided
	policy := metav1.DeletePropagationBackground
	if opts.PropagationPolicy != nil {
		policy = *opts.PropagationPolicy
	}
	// If the resource is being deleted, we want to record the first deletion attempt
	// so that we can track whether a deletion was attempted at least once.
	if u.GetAnnotations() == nil || u.GetAnnotations()[protection.AnnotationKeyDeletionAttempt] != string(policy) {
		orig := u.DeepCopy()
		xpmeta.AddAnnotations(u, map[string]string{protection.AnnotationKeyDeletionAttempt: string(policy)})
		// Patch the resource to add the deletion attempt annotation
		if err := h.client.Patch(ctx, u, client.MergeFrom(orig)); err != nil {
			h.log.Debug("Error when patching the resource to add the deletion attempt annotation", "apiVersion", u.GetAPIVersion(), "kind", u.GetKind(), "name", u.GetName(), "err", err)
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
}

func inUseMessage(u []protection.Usage) string {
	first := u[0]
	by := first.GetUsedBy()
	if by != nil {
		return fmt.Sprintf("This resource is in-use by %d Usage(s), including the Usage %q by resource %s/%s.", len(u), first.GetName(), by.Kind, by.ResourceRef.Name)
	}
	if r := ptr.Deref(first.GetReason(), ""); r != "" {
		return fmt.Sprintf("This resource is in-use by %d Usage(s), including the Usage %q with reason: %q.", len(u), first.GetName(), r)
	}
	// Either spec.by or spec.reason should be set, which we enforce with a CEL
	// rule. This is just a fallback.
	return fmt.Sprintf("This resource is in-use by %d Usage(s), including the Usage %q.", len(u), first.GetName())
}
