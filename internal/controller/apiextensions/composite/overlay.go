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

package composite

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

var _ resource.Composite = &overlayComposite{}
var _ runtime.Unstructured = &overlayComposite{}

type overlayComposite struct {
	*composite.Unstructured
	patch *composite.Unstructured
}

func newOverlayComposite(base *composite.Unstructured) *overlayComposite {
	p := &composite.Unstructured{}
	p.SetKind(base.GetKind())
	p.SetAPIVersion(base.GetAPIVersion())
	p.SetName(base.GetName())

	return &overlayComposite{Unstructured: base, patch: p}
}

func (r *overlayComposite) GetCompositionReference() *corev1.ObjectReference {
	if ref := r.patch.GetCompositionReference(); ref != nil {
		return ref
	}
	return r.Unstructured.GetCompositionReference()
}

func (r *overlayComposite) SetCompositionReference(ref *corev1.ObjectReference) {
	r.patch.SetCompositionReference(ref)
	r.Unstructured.SetCompositionReference(ref)
}

func (r *overlayComposite) GetCompositionRevisionReference() *corev1.ObjectReference {
	if ref := r.patch.GetCompositionRevisionReference(); ref != nil {
		return ref
	}
	return r.Unstructured.GetCompositionRevisionReference()
}

func (r *overlayComposite) SetCompositionRevisionReference(ref *corev1.ObjectReference) {
	r.patch.SetCompositionRevisionReference(ref)
	r.Unstructured.SetCompositionRevisionReference(ref)
}

func (r *overlayComposite) GetEnvironmentConfigReferences() []corev1.ObjectReference {
	if refs := r.patch.GetEnvironmentConfigReferences(); len(refs) > 0 {
		return refs
	}
	return r.Unstructured.GetEnvironmentConfigReferences()
}

func (r *overlayComposite) SetEnvironmentConfigReferences(refs []corev1.ObjectReference) {
	r.patch.SetEnvironmentConfigReferences(refs)
	r.Unstructured.SetEnvironmentConfigReferences(refs)
}

func (r *overlayComposite) SetResourceReferences(refs []corev1.ObjectReference) {
	r.patch.SetResourceReferences(refs)
	r.Unstructured.SetResourceReferences(refs)
}

func (r *overlayComposite) SetWriteConnectionSecretToReference(sr *xpv1.SecretReference) {
	r.patch.SetWriteConnectionSecretToReference(sr)
	r.Unstructured.SetWriteConnectionSecretToReference(sr)
}

func (r *overlayComposite) GetPatch() *composite.Unstructured {
	r.patch.SetName(r.Unstructured.GetName())
	r.patch.SetKind(r.Unstructured.GetKind())
	r.patch.SetAPIVersion(r.Unstructured.GetAPIVersion())
	return r.patch
}

func (r *overlayComposite) GetBase() *composite.Unstructured {
	return r.Unstructured
}

func (r *overlayComposite) GetUnstructured() *unstructured.Unstructured {
	return r.Unstructured.GetUnstructured()
}

func (r *overlayComposite) SetFinalizers(finalizers []string) {
	r.patch.SetFinalizers(finalizers)
	r.Unstructured.SetFinalizers(finalizers)
}

func (r *overlayComposite) SetLabels(labels map[string]string) {
	r.patch.SetLabels(labels)
	meta.AddLabels(r.Unstructured, labels)
}

func (r *overlayComposite) SetPublishConnectionDetailsTo(c *xpv1.PublishConnectionDetailsTo) {
	r.patch.SetPublishConnectionDetailsTo(c)
	r.Unstructured.SetPublishConnectionDetailsTo(c)
}

type overlayAwareClient struct {
	client.Client
}

func (c *overlayAwareClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if pc, ok := obj.(*overlayComposite); ok {
		pobj := pc.GetPatch()
		if err := c.Client.Patch(ctx, pobj, patch, opts...); err != nil {
			return err
		}
		pc.GetBase().SetUnstructuredContent(pobj.UnstructuredContent())
		pobj.SetUnstructuredContent(map[string]any{})
		return nil
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}
