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
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

// A ConnectionSecretOwner may create and manage a connection secret.
type ConnectionSecretOwner interface {
	metav1.Object
	ConnectionSecretWriterTo
}

// ConnectionSecretFor the supplied ConnectionSecretOwner, assumed to be of the
// supplied kind.
func ConnectionSecretFor(o ConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetNamespace(),
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.ReferenceTo(o, kind))},
		},
		Data: make(map[string][]byte),
	}
}

// MustCreateObject returns a new Object of the supplied kind. It panics if the
// kind is unknown to the supplied ObjectCreator.
func MustCreateObject(kind schema.GroupVersionKind, oc runtime.ObjectCreater) runtime.Object {
	obj, err := oc.New(kind)
	if err != nil {
		panic(err)
	}
	return obj
}

// MustGetKind returns the GroupVersionKind of the supplied object. It panics if
// the object is unknown to the supplied ObjectTyper, the object is unversioned,
// or the object does not have exactly one registered kind.
func MustGetKind(obj runtime.Object, ot runtime.ObjectTyper) schema.GroupVersionKind {
	kinds, unversioned, err := ot.ObjectKinds(obj)
	if unversioned {
		panic("supplied object is unversioned")
	}
	if err != nil {
		panic(err)
	}
	if len(kinds) != 1 {
		panic("supplied ")
	}
	return kinds[0]
}

// An ErrorIs function returns true if an error satisfies a particular condition.
type ErrorIs func(err error) bool

// Ignore any errors that satisfy the supplied ErrorIs function by returning
// nil. Errors that do not satisfy the suppled function are returned unmodified.
func Ignore(is ErrorIs, err error) error {
	if is(err) {
		return nil
	}
	return err
}

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// Kubernetes resource was not found.
func IgnoreNotFound(err error) error {
	return Ignore(kerrors.IsNotFound, err)
}

// ResolveClassClaimValues validates the supplied claim value against the
// supplied resource class value. If both are non-zero they must match.
func ResolveClassClaimValues(classValue, claimValue string) (string, error) {
	if classValue == "" {
		return claimValue, nil
	}
	if claimValue == "" {
		return classValue, nil
	}
	if classValue != claimValue {
		return "", errors.Errorf("claim value [%s] does not match class value [%s]", claimValue, classValue)
	}
	return claimValue, nil
}

// SetBindable indicates that the supplied Bindable is ready for binding to
// another Bindable, such as a resource claim or managed resource.
func SetBindable(b Bindable) {
	if b.GetBindingPhase() == v1alpha1.BindingPhaseBound {
		return
	}
	b.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
}

// IsBindable returns true if the supplied Bindable is ready for binding to
// another Bindable, such as a resource claim or managed resource.
func IsBindable(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseUnbound
}

// IsBound returns true if the supplied Bindable is bound to another Bindable,
// such as a resource claim or managed resource.
func IsBound(b Bindable) bool {
	return b.GetBindingPhase() == v1alpha1.BindingPhaseBound
}
