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

package xresource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// TODO(negz): We can't use crossplane-runtime's connection secret types because
// they require an interface with support for external secret stores. We drop
// that feature in Crossplane v2. We should switch back to runtime's types and
// functions once it no longer requires secret store support.

// A LocalConnectionSecretOwner owns and writes a connection secret to its own
// namespace.
type LocalConnectionSecretOwner interface {
	resource.Object

	resource.LocalConnectionSecretWriterTo
}

// A ConnectionSecretOwner owns and writes a connection secret to a specified
// namespace.
type ConnectionSecretOwner interface {
	resource.Object

	resource.ConnectionSecretWriterTo
}

// LocalConnectionSecretFor creates a connection secret in the namespace of the
// supplied LocalConnectionSecretOwner, assumed to be of the supplied kind.
func LocalConnectionSecretFor(o LocalConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetNamespace(),
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(o, kind))},
		},
		Type: resource.SecretTypeConnection,
		Data: make(map[string][]byte),
	}
}

// ConnectionSecretFor creates a connection for the supplied
// ConnectionSecretOwner, assumed to be of the supplied kind.
func ConnectionSecretFor(o ConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetWriteConnectionSecretToReference().Namespace,
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(o, kind))},
		},
		Type: resource.SecretTypeConnection,
		Data: make(map[string][]byte),
	}
}
