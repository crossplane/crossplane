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
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// SecretTypeConnection is the type of Crossplane connection secrets.
const SecretTypeConnection corev1.SecretType = "connection.crossplane.io/v1alpha1"

// External resources are tagged/labelled with the following keys in the cloud
// provider API if the type supports.
const (
	ExternalResourceTagKeyKind     = "crossplane-kind"
	ExternalResourceTagKeyName     = "crossplane-name"
	ExternalResourceTagKeyProvider = "crossplane-providerconfig"
)

// A ManagedKind contains the type metadata for a kind of managed resource.
type ManagedKind schema.GroupVersionKind

// A CompositeKind contains the type metadata for a kind of composite resource.
type CompositeKind schema.GroupVersionKind

// A CompositeClaimKind contains the type metadata for a kind of composite
// resource claim.
type CompositeClaimKind schema.GroupVersionKind

// ProviderConfigKinds contains the type metadata for a kind of provider config.
type ProviderConfigKinds struct {
	Config    schema.GroupVersionKind
	Usage     schema.GroupVersionKind
	UsageList schema.GroupVersionKind
}

// A ConnectionSecretOwner is a Kubernetes object that owns a connection secret.
type ConnectionSecretOwner interface {
	Object

	ConnectionSecretWriterTo
	ConnectionDetailsPublisherTo
}

// A LocalConnectionSecretOwner may create and manage a connection secret in its
// own namespace.
type LocalConnectionSecretOwner interface {
	runtime.Object
	metav1.Object

	LocalConnectionSecretWriterTo
	ConnectionDetailsPublisherTo
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
		Type: SecretTypeConnection,
		Data: make(map[string][]byte),
	}
}

// ConnectionSecretFor creates a connection for the supplied
// ConnectionSecretOwner, assumed to be of the supplied kind. The secret is
// written to 'default' namespace if the ConnectionSecretOwner does not specify
// a namespace.
func ConnectionSecretFor(o ConnectionSecretOwner, kind schema.GroupVersionKind) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       o.GetWriteConnectionSecretToReference().Namespace,
			Name:            o.GetWriteConnectionSecretToReference().Name,
			OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(o, kind))},
		},
		Type: SecretTypeConnection,
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

// GetKind returns the GroupVersionKind of the supplied object. It return an
// error if the object is unknown to the supplied ObjectTyper, the object is
// unversioned, or the object does not have exactly one registered kind.
func GetKind(obj runtime.Object, ot runtime.ObjectTyper) (schema.GroupVersionKind, error) {
	kinds, unversioned, err := ot.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, errors.Wrap(err, "cannot get kind of supplied object")
	}
	if unversioned {
		return schema.GroupVersionKind{}, errors.New("supplied object is unversioned")
	}
	if len(kinds) != 1 {
		return schema.GroupVersionKind{}, errors.New("supplied object does not have exactly one kind")
	}
	return kinds[0], nil
}

// MustGetKind returns the GroupVersionKind of the supplied object. It panics if
// the object is unknown to the supplied ObjectTyper, the object is unversioned,
// or the object does not have exactly one registered kind.
func MustGetKind(obj runtime.Object, ot runtime.ObjectTyper) schema.GroupVersionKind {
	gvk, err := GetKind(obj, ot)
	if err != nil {
		panic(err)
	}
	return gvk
}

// An ErrorIs function returns true if an error satisfies a particular condition.
type ErrorIs func(err error) bool

// Ignore any errors that satisfy the supplied ErrorIs function by returning
// nil. Errors that do not satisfy the supplied function are returned unmodified.
func Ignore(is ErrorIs, err error) error {
	if is(err) {
		return nil
	}
	return err
}

// IgnoreAny ignores errors that satisfy any of the supplied ErrorIs functions
// by returning nil. Errors that do not satisfy any of the supplied functions
// are returned unmodified.
func IgnoreAny(err error, is ...ErrorIs) error {
	for _, f := range is {
		if f(err) {
			return nil
		}
	}
	return err
}

// IgnoreNotFound returns the supplied error, or nil if the error indicates a
// Kubernetes resource was not found.
func IgnoreNotFound(err error) error {
	return Ignore(kerrors.IsNotFound, err)
}

// IsAPIError returns true if the given error's type is of Kubernetes API error.
func IsAPIError(err error) bool {
	_, ok := err.(kerrors.APIStatus) //nolint: errorlint // we assert against the kerrors.APIStatus Interface which does not implement the error interface
	return ok
}

// IsAPIErrorWrapped returns true if err is a K8s API error, or recursively wraps a K8s API error
func IsAPIErrorWrapped(err error) bool {
	return IsAPIError(errors.Cause(err))
}

// IsConditionTrue returns if condition status is true
func IsConditionTrue(c xpv1.Condition) bool {
	return c.Status == corev1.ConditionTrue
}

// An Applicator applies changes to an object.
type Applicator interface {
	Apply(context.Context, client.Object, ...ApplyOption) error
}

type shouldRetryFunc func(error) bool

// An ApplicatorWithRetry applies changes to an object, retrying on transient failures
type ApplicatorWithRetry struct {
	Applicator
	shouldRetry shouldRetryFunc
	backoff     wait.Backoff
}

// Apply invokes nested Applicator's Apply retrying on designated errors
func (awr *ApplicatorWithRetry) Apply(ctx context.Context, c client.Object, opts ...ApplyOption) error {
	return retry.OnError(awr.backoff, awr.shouldRetry, func() error {
		return awr.Applicator.Apply(ctx, c, opts...)
	})
}

// NewApplicatorWithRetry returns an ApplicatorWithRetry for the specified
// applicator and with the specified retry function.
//
//	If backoff is nil, then retry.DefaultRetry is used as the default.
func NewApplicatorWithRetry(applicator Applicator, shouldRetry shouldRetryFunc, backoff *wait.Backoff) *ApplicatorWithRetry {
	result := &ApplicatorWithRetry{
		Applicator:  applicator,
		shouldRetry: shouldRetry,
		backoff:     retry.DefaultRetry,
	}

	if backoff != nil {
		result.backoff = *backoff
	}

	return result
}

// A ClientApplicator may be used to build a single 'client' that satisfies both
// client.Client and Applicator.
type ClientApplicator struct {
	client.Client
	Applicator
}

// An ApplyFn is a function that satisfies the Applicator interface.
type ApplyFn func(context.Context, client.Object, ...ApplyOption) error

// Apply changes to the supplied object.
func (fn ApplyFn) Apply(ctx context.Context, o client.Object, ao ...ApplyOption) error {
	return fn(ctx, o, ao...)
}

// An ApplyOption is called before patching the current object to match the
// desired object. ApplyOptions are not called if no current object exists.
type ApplyOption func(ctx context.Context, current, desired runtime.Object) error

// UpdateFn returns an ApplyOption that is used to modify the current object to
// match fields of the desired.
func UpdateFn(fn func(current, desired runtime.Object)) ApplyOption {
	return func(_ context.Context, c, d runtime.Object) error {
		fn(c, d)
		return nil
	}
}

type errNotControllable struct{ error }

func (e errNotControllable) NotControllable() bool {
	return true
}

// IsNotControllable returns true if the supplied error indicates that a
// resource is not controllable - i.e. that it another resource is not and may
// not become its controller reference.
func IsNotControllable(err error) bool {
	_, ok := err.(interface { //nolint: errorlint // Skip errorlint for interface type
		NotControllable() bool
	})
	return ok
}

// MustBeControllableBy requires that the current object is controllable by an
// object with the supplied UID. An object is controllable if its controller
// reference matches the supplied UID, or it has no controller reference. An
// error that satisfies IsNotControllable will be returned if the current object
// cannot be controlled by the supplied UID.
func MustBeControllableBy(u types.UID) ApplyOption {
	return func(_ context.Context, current, _ runtime.Object) error {
		c := metav1.GetControllerOf(current.(metav1.Object))
		if c == nil {
			return nil
		}

		if c.UID != u {
			return errNotControllable{errors.Errorf("existing object is not controlled by UID %q", u)}

		}
		return nil
	}
}

// ConnectionSecretMustBeControllableBy requires that the current object is a
// connection secret that is controllable by an object with the supplied UID.
// Contemporary connection secrets are of SecretTypeConnection, while legacy
// connection secrets are of corev1.SecretTypeOpaque. Contemporary connection
// secrets are considered controllable if they are already controlled by the
// supplied UID, or have no controller reference. Legacy connection secrets are
// only considered controllable if they are already controlled by the supplied
// UID. It is not safe to assume legacy connection secrets without a controller
// reference are controllable because they are indistinguishable from Kubernetes
// secrets that have nothing to do with Crossplane. An error that satisfies
// IsNotControllable will be returned if the current secret is not a connection
// secret or cannot be controlled by the supplied UID.
func ConnectionSecretMustBeControllableBy(u types.UID) ApplyOption {
	return func(_ context.Context, current, _ runtime.Object) error {
		s := current.(*corev1.Secret)
		c := metav1.GetControllerOf(s)

		switch {
		case c == nil && s.Type != SecretTypeConnection:
			return errNotControllable{errors.Errorf("refusing to modify uncontrolled secret of type %q", s.Type)}
		case c == nil:
			return nil
		case c.UID != u:
			return errNotControllable{errors.Errorf("existing secret is not controlled by UID %q", u)}
		}

		return nil
	}
}

type errNotAllowed struct{ error }

func (e errNotAllowed) NotAllowed() bool {
	return true
}

// NewNotAllowed returns a new NotAllowed error
func NewNotAllowed(message string) error {
	return errNotAllowed{error: errors.New(message)}
}

// IsNotAllowed returns true if the supplied error indicates that an operation
// was not allowed.
func IsNotAllowed(err error) bool {
	_, ok := err.(interface { //nolint: errorlint // Skip errorlint for interface type
		NotAllowed() bool
	})
	return ok
}

// AllowUpdateIf will only update the current object if the supplied fn returns
// true. An error that satisfies IsNotAllowed will be returned if the supplied
// function returns false. Creation of a desired object that does not currently
// exist is always allowed.
func AllowUpdateIf(fn func(current, desired runtime.Object) bool) ApplyOption {
	return func(_ context.Context, current, desired runtime.Object) error {
		if fn(current, desired) {
			return nil
		}
		return errNotAllowed{errors.New("update not allowed")}
	}
}

// StoreCurrentRV stores the resource version of the current object in the
// supplied string pointer. This is useful to detect whether the Apply call
// was a no-op.
func StoreCurrentRV(origRV *string) ApplyOption {
	return func(_ context.Context, current, desired runtime.Object) error {
		*origRV = current.(client.Object).GetResourceVersion()
		return nil
	}
}

// GetExternalTags returns the identifying tags to be used to tag the external
// resource in provider API.
func GetExternalTags(mg Managed) map[string]string {
	tags := map[string]string{
		ExternalResourceTagKeyKind: strings.ToLower(mg.GetObjectKind().GroupVersionKind().GroupKind().String()),
		ExternalResourceTagKeyName: mg.GetName(),
	}

	if mg.GetProviderConfigReference() != nil && mg.GetProviderConfigReference().Name != "" {
		tags[ExternalResourceTagKeyProvider] = mg.GetProviderConfigReference().Name
	}
	return tags
}

// DefaultFirstN is the default number of names to return in FirstNAndSomeMore.
const DefaultFirstN = 3

// FirstNAndSomeMore returns a string that contains the first n names in the
// supplied slice, followed by ", and <count> more" if there are more than n.
// The slice is not sorted, i.e. the caller must make sure the order is stable
// e.g. when using this in conditions.
func FirstNAndSomeMore(n int, names []string) string {
	if n <= 0 {
		return fmt.Sprintf("%d", len(names))
	}
	if len(names) > n {
		return fmt.Sprintf("%s, and %d more", strings.Join(names[:n], ", "), len(names)-n)
	}
	if len(names) == n {
		return fmt.Sprintf("%s, and %s", strings.Join(names[:n-1], ", "), names[n-1])
	}
	return strings.Join(names, ", ")
}

// StableNAndSomeMore is like FirstNAndSomeMore, but sorts the names before.
// The input slice is not modified.
func StableNAndSomeMore(n int, names []string) string {
	cpy := make([]string, len(names))
	copy(cpy, names)
	sort.Strings(cpy)
	return FirstNAndSomeMore(n, cpy)
}
