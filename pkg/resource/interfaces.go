/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// A Bindable resource may be bound to another resource. Resources are bindable
// when they available for use.
type Bindable interface {
	SetBindingPhase(p v1alpha1.BindingPhase)
	GetBindingPhase() v1alpha1.BindingPhase
}

// A ConditionSetter may have conditions set. Conditions are informational, and
// typically indicate the status of both a resource and its reconciliation
// process.
type ConditionSetter interface {
	SetConditions(c ...v1alpha1.Condition)
}

// A ClaimReferencer may reference a resource claim.
type ClaimReferencer interface {
	SetClaimReference(r *corev1.ObjectReference)
	GetClaimReference() *corev1.ObjectReference
}

// A ClassReferencer may reference a resource class.
type ClassReferencer interface {
	SetClassReference(r *corev1.ObjectReference)
	GetClassReference() *corev1.ObjectReference
}

// A ManagedResourceReferencer may reference a concrete managed resource.
type ManagedResourceReferencer interface {
	SetResourceReference(r *corev1.ObjectReference)
	GetResourceReference() *corev1.ObjectReference
}

// A ConnectionSecretWriterTo may write a connection secret.
type ConnectionSecretWriterTo interface {
	SetWriteConnectionSecretTo(r corev1.LocalObjectReference)
	GetWriteConnectionSecretTo() corev1.LocalObjectReference
}

// A Claim is a Kubernetes object representing an abstract resource claim (e.g.
// an SQL database) that may be bound to a concrete managed resource (e.g. a
// CloudSQL instance).
type Claim interface {
	runtime.Object
	metav1.Object

	ClassReferencer
	ManagedResourceReferencer
	ConnectionSecretWriterTo

	ConditionSetter
	Bindable
}

// A ManagedResource is a Kubernetes object representing a concrete managed
// resource (e.g. a CloudSQL instance).
type ManagedResource interface {
	runtime.Object
	metav1.Object

	ClassReferencer
	ClaimReferencer
	ConnectionSecretWriterTo

	Bindable
}
