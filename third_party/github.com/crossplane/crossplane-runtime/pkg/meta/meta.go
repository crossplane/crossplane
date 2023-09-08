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

// Package meta contains functions for dealing with Kubernetes object metadata.
package meta

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	// AnnotationKeyExternalName is the key in the annotations map of a
	// resource for the name of the resource as it appears on provider's
	// systems.
	AnnotationKeyExternalName = "crossplane.io/external-name"

	// AnnotationKeyExternalCreatePending is the key in the annotations map
	// of a resource that indicates the last time creation of the external
	// resource was pending (i.e. about to happen). Its value must be an
	// RFC3999 timestamp.
	AnnotationKeyExternalCreatePending = "crossplane.io/external-create-pending"

	// AnnotationKeyExternalCreateSucceeded is the key in the annotations
	// map of a resource that represents the last time the external resource
	// was created successfully. Its value must be an RFC3339 timestamp,
	// which can be used to determine how long ago a resource was created.
	// This is useful for eventually consistent APIs that may take some time
	// before the API called by Observe will report that a recently created
	// external resource exists.
	AnnotationKeyExternalCreateSucceeded = "crossplane.io/external-create-succeeded"

	// AnnotationKeyExternalCreateFailed is the key in the annotations map
	// of a resource that indicates the last time creation of the external
	// resource failed. Its value must be an RFC3999 timestamp.
	AnnotationKeyExternalCreateFailed = "crossplane.io/external-create-failed"

	// AnnotationKeyReconciliationPaused is the key in the annotations map
	// of a resource that indicates that further reconciliations on the
	// resource are paused. All create/update/delete/generic events on
	// the resource will be filtered and thus no further reconcile requests
	// will be queued for the resource.
	AnnotationKeyReconciliationPaused = "crossplane.io/paused"
)

// ReferenceTo returns an object reference to the supplied object, presumed to
// be of the supplied group, version, and kind.
// Deprecated: use a more specific reference type, such as TypedReference or
// Reference instead of the overly verbose ObjectReference.
// See https://github.com/crossplane/crossplane-runtime/issues/49
func ReferenceTo(o metav1.Object, of schema.GroupVersionKind) *corev1.ObjectReference {
	v, k := of.ToAPIVersionAndKind()
	return &corev1.ObjectReference{
		APIVersion: v,
		Kind:       k,
		Namespace:  o.GetNamespace(),
		Name:       o.GetName(),
		UID:        o.GetUID(),
	}
}

// TypedReferenceTo returns a typed object reference to the supplied object,
// presumed to be of the supplied group, version, and kind.
func TypedReferenceTo(o metav1.Object, of schema.GroupVersionKind) *xpv1.TypedReference {
	v, k := of.ToAPIVersionAndKind()
	return &xpv1.TypedReference{
		APIVersion: v,
		Kind:       k,
		Name:       o.GetName(),
		Namespace:  o.GetNamespace(),
		UID:        o.GetUID(),
	}
}

// AsOwner converts the supplied object reference to an owner reference.
func AsOwner(r *xpv1.TypedReference) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: r.APIVersion,
		Kind:       r.Kind,
		Name:       r.Name,
		UID:        r.UID,
	}
}

// AsController converts the supplied object reference to a controller
// reference. You may also consider using metav1.NewControllerRef.
func AsController(r *xpv1.TypedReference) metav1.OwnerReference {
	t := true
	ref := AsOwner(r)
	ref.Controller = &t
	ref.BlockOwnerDeletion = &t
	return ref
}

// HaveSameController returns true if both supplied objects are controlled by
// the same object.
func HaveSameController(a, b metav1.Object) bool {
	ac := metav1.GetControllerOf(a)
	bc := metav1.GetControllerOf(b)

	// We do not consider two objects without any controller to have
	// the same controller.
	if ac == nil || bc == nil {
		return false
	}

	return ac.UID == bc.UID
}

// NamespacedNameOf returns the referenced object's namespaced name.
func NamespacedNameOf(r *corev1.ObjectReference) types.NamespacedName {
	return types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
}

// AddOwnerReference to the supplied object' metadata. Any existing owner with
// the same UID as the supplied reference will be replaced.
func AddOwnerReference(o metav1.Object, r metav1.OwnerReference) {
	refs := o.GetOwnerReferences()
	for i := range refs {
		if refs[i].UID == r.UID {
			refs[i] = r
			o.SetOwnerReferences(refs)
			return
		}
	}
	o.SetOwnerReferences(append(refs, r))
}

// AddControllerReference to the supplied object's metadata. Any existing owner
// with the same UID as the supplied reference will be replaced. Returns an
// error if the supplied object is already controlled by a different owner.
func AddControllerReference(o metav1.Object, r metav1.OwnerReference) error {
	if c := metav1.GetControllerOf(o); c != nil && c.UID != r.UID {
		return errors.Errorf("%s is already controlled by %s %s (UID %s)", o.GetName(), c.Kind, c.Name, c.UID)
	}

	AddOwnerReference(o, r)
	return nil
}

// AddFinalizer to the supplied Kubernetes object's metadata.
func AddFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return
		}
	}
	o.SetFinalizers(append(f, finalizer))
}

// RemoveFinalizer from the supplied Kubernetes object's metadata.
func RemoveFinalizer(o metav1.Object, finalizer string) {
	f := o.GetFinalizers()
	for i, e := range f {
		if e == finalizer {
			f = append(f[:i], f[i+1:]...)
		}
	}
	o.SetFinalizers(f)
}

// FinalizerExists checks whether given finalizer is already set.
func FinalizerExists(o metav1.Object, finalizer string) bool {
	f := o.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return true
		}
	}
	return false
}

// AddLabels to the supplied object.
func AddLabels(o metav1.Object, labels map[string]string) {
	l := o.GetLabels()
	if l == nil {
		o.SetLabels(labels)
		return
	}
	for k, v := range labels {
		l[k] = v
	}
	o.SetLabels(l)
}

// RemoveLabels with the supplied keys from the supplied object.
func RemoveLabels(o metav1.Object, labels ...string) {
	l := o.GetLabels()
	if l == nil {
		return
	}
	for _, k := range labels {
		delete(l, k)
	}
	o.SetLabels(l)
}

// AddAnnotations to the supplied object.
func AddAnnotations(o metav1.Object, annotations map[string]string) {
	a := o.GetAnnotations()
	if a == nil {
		o.SetAnnotations(annotations)
		return
	}
	for k, v := range annotations {
		a[k] = v
	}
	o.SetAnnotations(a)
}

// RemoveAnnotations with the supplied keys from the supplied object.
func RemoveAnnotations(o metav1.Object, annotations ...string) {
	a := o.GetAnnotations()
	if a == nil {
		return
	}
	for _, k := range annotations {
		delete(a, k)
	}
	o.SetAnnotations(a)
}

// WasDeleted returns true if the supplied object was deleted from the API server.
func WasDeleted(o metav1.Object) bool {
	return !o.GetDeletionTimestamp().IsZero()
}

// WasCreated returns true if the supplied object was created in the API server.
func WasCreated(o metav1.Object) bool {
	// This looks a little different from WasDeleted because DeletionTimestamp
	// returns a reference while CreationTimestamp returns a value.
	t := o.GetCreationTimestamp()
	return !t.IsZero()
}

// GetExternalName returns the external name annotation value on the resource.
func GetExternalName(o metav1.Object) string {
	return o.GetAnnotations()[AnnotationKeyExternalName]
}

// SetExternalName sets the external name annotation of the resource.
func SetExternalName(o metav1.Object, name string) {
	AddAnnotations(o, map[string]string{AnnotationKeyExternalName: name})
}

// GetExternalCreatePending returns the time at which the external resource
// was most recently pending creation.
func GetExternalCreatePending(o metav1.Object) time.Time {
	a := o.GetAnnotations()[AnnotationKeyExternalCreatePending]
	t, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SetExternalCreatePending sets the time at which the external resource was
// most recently pending creation to the supplied time.
func SetExternalCreatePending(o metav1.Object, t time.Time) {
	AddAnnotations(o, map[string]string{AnnotationKeyExternalCreatePending: t.Format(time.RFC3339)})
}

// GetExternalCreateSucceeded returns the time at which the external resource
// was most recently created.
func GetExternalCreateSucceeded(o metav1.Object) time.Time {
	a := o.GetAnnotations()[AnnotationKeyExternalCreateSucceeded]
	t, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SetExternalCreateSucceeded sets the time at which the external resource was
// most recently created to the supplied time.
func SetExternalCreateSucceeded(o metav1.Object, t time.Time) {
	AddAnnotations(o, map[string]string{AnnotationKeyExternalCreateSucceeded: t.Format(time.RFC3339)})
}

// GetExternalCreateFailed returns the time at which the external resource
// recently failed to create.
func GetExternalCreateFailed(o metav1.Object) time.Time {
	a := o.GetAnnotations()[AnnotationKeyExternalCreateFailed]
	t, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SetExternalCreateFailed sets the time at which the external resource most
// recently failed to create.
func SetExternalCreateFailed(o metav1.Object, t time.Time) {
	AddAnnotations(o, map[string]string{AnnotationKeyExternalCreateFailed: t.Format(time.RFC3339)})
}

// ExternalCreateIncomplete returns true if creation of the external resource
// appears to be incomplete. We deem creation to be incomplete if the 'external
// create pending' annotation is the newest of all tracking annotations that are
// set (i.e. pending, succeeded, and failed).
func ExternalCreateIncomplete(o metav1.Object) bool {
	pending := GetExternalCreatePending(o)
	succeeded := GetExternalCreateSucceeded(o)
	failed := GetExternalCreateFailed(o)

	// If creation never started it can't be incomplete.
	if pending.IsZero() {
		return false
	}

	latest := succeeded
	if failed.After(succeeded) {
		latest = failed
	}

	return pending.After(latest)
}

// ExternalCreateSucceededDuring returns true if creation of the external
// resource that corresponds to the supplied managed resource succeeded within
// the supplied duration.
func ExternalCreateSucceededDuring(o metav1.Object, d time.Duration) bool {
	t := GetExternalCreateSucceeded(o)
	if t.IsZero() {
		return false
	}
	return time.Since(t) < d
}

// IsPaused returns true if the object has the AnnotationKeyReconciliationPaused
// annotation set to `true`.
func IsPaused(o metav1.Object) bool {
	return o.GetAnnotations()[AnnotationKeyReconciliationPaused] == "true"
}
