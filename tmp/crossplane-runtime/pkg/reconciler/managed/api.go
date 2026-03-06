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

package managed

import (
	"context"
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

const (
	// fieldOwnerAPISimpleRefResolver owns the reference fields
	// the managed reconciler resolves.
	fieldOwnerAPISimpleRefResolver = "managed.crossplane.io/api-simple-reference-resolver"
)

// Error strings.
const (
	errCreateOrUpdateSecret      = "cannot create or update connection secret"
	errUpdateManaged             = "cannot update managed resource"
	errPatchManaged              = "cannot patch the managed resource via server-side apply"
	errMarshalExisting           = "cannot marshal the existing object into JSON"
	errMarshalResolved           = "cannot marshal the object with the resolved references into JSON"
	errPreparePatch              = "cannot prepare the JSON merge patch for the resolved object"
	errUpdateManagedStatus       = "cannot update managed resource status"
	errResolveReferences         = "cannot resolve references"
	errUpdateCriticalAnnotations = "cannot update critical annotations"
)

// NameAsExternalName writes the name of the managed resource to
// the external name annotation field in order to be used as name of
// the external resource in provider.
type NameAsExternalName struct{ client client.Client }

// NewNameAsExternalName returns a new NameAsExternalName.
func NewNameAsExternalName(c client.Client) *NameAsExternalName {
	return &NameAsExternalName{client: c}
}

// Initialize the given managed resource.
func (a *NameAsExternalName) Initialize(ctx context.Context, mg resource.Managed) error {
	if meta.GetExternalName(mg) != "" {
		return nil
	}

	meta.SetExternalName(mg, mg.GetName())

	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}

// An APISecretPublisher publishes ConnectionDetails by submitting a Secret to a
// Kubernetes API server.
type APISecretPublisher struct {
	secret resource.Applicator
	typer  runtime.ObjectTyper
}

// NewAPISecretPublisher returns a new APISecretPublisher.
func NewAPISecretPublisher(c client.Client, ot runtime.ObjectTyper) *APISecretPublisher {
	// NOTE(negz): We transparently inject an APIPatchingApplicator in order to maintain
	// backward compatibility with the original API of this function.
	return &APISecretPublisher{
		secret: resource.NewApplicatorWithRetry(resource.NewAPIPatchingApplicator(c),
			resource.IsAPIErrorWrapped, nil),
		typer: ot,
	}
}

// PublishConnection publishes the supplied ConnectionDetails to a Secret in the
// same namespace as the supplied Managed resource. It is a no-op if the secret
// already exists with the supplied ConnectionDetails.
func (a *APISecretPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	if o.GetWriteConnectionSecretToReference() == nil {
		return false, nil
	}

	s := resource.ConnectionSecretFor(o, resource.MustGetKind(o, a.typer))
	s.Data = c

	err := a.secret.Apply(ctx, s,
		resource.ConnectionSecretMustBeControllableBy(o.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.
			//nolint:forcetypeassert // Will always be a secret.
			return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data, cmpopts.EquateEmpty())
		}),
	)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}

	if err != nil {
		return false, errors.Wrap(err, errCreateOrUpdateSecret)
	}

	return true, nil
}

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APISecretPublisher) UnpublishConnection(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error {
	return nil
}

// An APILocalSecretPublisher publishes ConnectionDetails by submitting a Secret to a
// Kubernetes API server.
type APILocalSecretPublisher struct {
	secret resource.Applicator
	typer  runtime.ObjectTyper
}

// NewAPILocalSecretPublisher returns a new APILocalSecretPublisher.
func NewAPILocalSecretPublisher(c client.Client, ot runtime.ObjectTyper) *APILocalSecretPublisher {
	// NOTE(negz): We transparently inject an APIPatchingApplicator in order to maintain
	// backward compatibility with the original API of this function.
	return &APILocalSecretPublisher{
		secret: resource.NewApplicatorWithRetry(resource.NewAPIPatchingApplicator(c),
			resource.IsAPIErrorWrapped, nil),
		typer: ot,
	}
}

// PublishConnection publishes the supplied ConnectionDetails to a Secret in the
// same namespace as the supplied Managed resource. It is a no-op if the secret
// already exists with the supplied ConnectionDetails.
func (a *APILocalSecretPublisher) PublishConnection(ctx context.Context, o resource.LocalConnectionSecretOwner, c ConnectionDetails) (bool, error) {
	// This resource does not want to expose a connection secret.
	if o.GetWriteConnectionSecretToReference() == nil {
		return false, nil
	}

	s := resource.LocalConnectionSecretFor(o, resource.MustGetKind(o, a.typer))
	s.Data = c

	err := a.secret.Apply(ctx, s,
		resource.ConnectionSecretMustBeControllableBy(o.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.
			//nolint:forcetypeassert // Will always be a secret.
			// NOTE(erhancagirici): cmp package is not recommended for production use
			return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data, cmpopts.EquateEmpty())
		}),
	)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}

	if err != nil {
		return false, errors.Wrap(err, errCreateOrUpdateSecret)
	}

	return true, nil
}

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APILocalSecretPublisher) UnpublishConnection(_ context.Context, _ resource.LocalConnectionSecretOwner, _ ConnectionDetails) error {
	return nil
}

// An APISimpleReferenceResolver resolves references from one managed resource
// to others by calling the referencing resource's ResolveReferences method, if
// any.
type APISimpleReferenceResolver struct {
	client client.Client
}

// NewAPISimpleReferenceResolver returns a ReferenceResolver that resolves
// references from one managed resource to others by calling the referencing
// resource's ResolveReferences method, if any.
func NewAPISimpleReferenceResolver(c client.Client) *APISimpleReferenceResolver {
	return &APISimpleReferenceResolver{client: c}
}

func prepareJSONMerge(existing, resolved runtime.Object) ([]byte, error) {
	// restore the to be replaced GVK so that the existing object is
	// not modified by this function.
	defer existing.GetObjectKind().SetGroupVersionKind(existing.GetObjectKind().GroupVersionKind())
	// we need the apiVersion and kind in the patch document so we set them
	// to their zero values and make them available in the calculated patch
	// in the first place, instead of an unmarshal/marshal from the prepared
	// patch []byte later.
	existing.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})

	eBuff, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalExisting)
	}

	rBuff, err := json.Marshal(resolved)
	if err != nil {
		return nil, errors.Wrap(err, errMarshalResolved)
	}

	patch, err := jsonpatch.CreateMergePatch(eBuff, rBuff)

	return patch, errors.Wrap(err, errPreparePatch)
}

// ResolveReferences of the supplied managed resource by calling its
// ResolveReferences method, if any.
func (a *APISimpleReferenceResolver) ResolveReferences(ctx context.Context, mg resource.Managed) error {
	rr, ok := mg.(interface {
		ResolveReferences(ctx context.Context, r client.Reader) error
	})
	if !ok {
		// This managed resource doesn't have any references to resolve.
		return nil
	}

	existing := mg.DeepCopyObject()

	if err := rr.ResolveReferences(ctx, a.client); err != nil {
		return errors.Wrap(err, errResolveReferences)
	}

	if cmp.Equal(existing, mg, cmpopts.EquateEmpty()) {
		// The resource didn't change during reference resolution.
		return nil
	}

	patch, err := prepareJSONMerge(existing, mg)
	if err != nil {
		return err
	}

	return errors.Wrap(a.client.Patch(ctx, mg, client.RawPatch(types.ApplyPatchType, patch), client.FieldOwner(fieldOwnerAPISimpleRefResolver), client.ForceOwnership), errPatchManaged)
}

// A RetryingCriticalAnnotationUpdater is a CriticalAnnotationUpdater that
// retries annotation updates in the face of API server errors.
type RetryingCriticalAnnotationUpdater struct {
	client client.Client
}

// NewRetryingCriticalAnnotationUpdater returns a CriticalAnnotationUpdater that
// retries annotation updates in the face of API server errors.
func NewRetryingCriticalAnnotationUpdater(c client.Client) *RetryingCriticalAnnotationUpdater {
	return &RetryingCriticalAnnotationUpdater{client: c}
}

// UpdateCriticalAnnotations updates (i.e. persists) the annotations of the
// supplied Object. It retries in the face of any API server error several times
// in order to ensure annotations that contain critical state are persisted.
// Pending changes to the supplied Object's spec, status, or other metadata
// might get reset to their current state according to the API server, e.g. in
// case of a conflict error.
func (u *RetryingCriticalAnnotationUpdater) UpdateCriticalAnnotations(ctx context.Context, o client.Object) error {
	a := o.GetAnnotations()
	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return !errors.Is(err, context.Canceled)
	}, func() error {
		err := u.client.Update(ctx, o)
		if kerrors.IsConflict(err) {
			if getErr := u.client.Get(ctx, client.ObjectKeyFromObject(o), o); getErr != nil {
				return getErr
			}

			meta.AddAnnotations(o, a)
		}

		return err
	})

	return errors.Wrap(err, errUpdateCriticalAnnotations)
}
