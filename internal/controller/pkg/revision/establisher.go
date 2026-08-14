/*
Copyright 2020 The Crossplane Authors.

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

package revision

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

const (
	errAssertResourceObj            = "cannot assert object to resource.Object"
	errAssertClientObj              = "cannot assert object to client.Object"
	errConversionWithNoWebhookCA    = "cannot deploy a CRD with webhook conversion strategy without having a TLS bundle"
	errGetWebhookTLSSecret          = "cannot get webhook tls secret"
	errWebhookSecretNotPresent      = "waiting for package runtime controller to set revision's webhook TLS secret"
	errWebhookSecretWithoutCABundle = "the value for the key tls.crt cannot be empty"
	errFmtGetOwnedObject            = "cannot get owned object: %s/%s"
	errFmtUpdateOwnedObject         = "cannot update owned object: %s/%s"
)

const (
	// ServicePort is the port number used by package services for webhook communication.
	ServicePort = 9443

	// FieldOwnerAPIEstablisher is the field manager used to apply package
	// objects the active package revision controls.
	FieldOwnerAPIEstablisher = "pkg.crossplane.io/establisher"

	// fieldOwnerAPIEstablisherOwnership is intentionally distinct from the
	// controlling field manager. An inactive revision applies only owner
	// references; using the controlling manager would cause SSA to prune the
	// package fields that manager previously applied.
	fieldOwnerAPIEstablisherOwnership = "pkg.crossplane.io/establisher-ownership"

	// annotationKeyEstablisherHash records the last package intent applied by
	// the establisher. It lets us skip the API call when a stored object differs
	// only by fields defaulted by the API server.
	annotationKeyEstablisherHash = "pkg.crossplane.io/establisher-hash"
)

// An Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv2.TypedReference, error)
	ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error
}

// NewNopEstablisher returns a new NopEstablisher.
func NewNopEstablisher() *NopEstablisher {
	return &NopEstablisher{}
}

// NopEstablisher does nothing.
type NopEstablisher struct{}

// Establish does nothing.
func (*NopEstablisher) Establish(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv2.TypedReference, error) {
	return nil, nil
}

// ReleaseObjects does nothing.
func (*NopEstablisher) ReleaseObjects(_ context.Context, _ v1.PackageRevision) error {
	return nil
}

// APIEstablisher establishes control or ownership of resources in the API
// server for a parent.
type APIEstablisher struct {
	client                           client.Client
	namespace                        string
	MaxConcurrentPackageEstablishers int
}

// NewAPIEstablisher creates a new APIEstablisher.
func NewAPIEstablisher(client client.Client, namespace string, maxConcurrentPackageEstablishers int) *APIEstablisher {
	return &APIEstablisher{
		client:                           client,
		namespace:                        namespace,
		MaxConcurrentPackageEstablishers: maxConcurrentPackageEstablishers,
	}
}

// desiredApply caches the apply intent while checking for control or ownership
// so that the same intent can be used for dry-run validation and establishment.
type desiredApply struct {
	Desired    resource.Object
	Reference  resource.Object
	Apply      *unstructured.Unstructured
	FieldOwner string
}

// Establish checks that control or ownership of resources can be established by
// parent, then establishes it.
func (e *APIEstablisher) Establish(ctx context.Context, objs []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv2.TypedReference, error) {
	err := e.addLabels(objs, parent)
	if err != nil {
		return nil, err
	}

	err = e.addAnnotations(objs, parent)
	if err != nil {
		return nil, err
	}

	allObjs, err := e.validate(ctx, objs, parent, control)
	if err != nil {
		return nil, err
	}

	resourceRefs, err := e.establish(ctx, allObjs)
	if err != nil {
		return nil, err
	}

	return resourceRefs, nil
}

// ReleaseObjects removes control of owned resources in the API server for a
// package revision.
func (e *APIEstablisher) ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error { //nolint:gocognit // complexity coming from parallelism.
	// Note(turkenh): We rely on status.objectRefs to get the list of objects
	// that are controlled by the package revision. Relying on the status is
	// not ideal as it might get lost (e.g. if the status subresource is
	// not properly restored after a backup/restore operation). However, we will
	// handle this by conditionally fetching/parsing package if there is no
	// referenced resources available and rebuilding the status.
	// In the next reconciliation loop, and we will be able to remove the
	// control/ownership of the objects using the new status.
	allObjs := parent.GetObjects()
	if len(allObjs) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(e.MaxConcurrentPackageEstablishers)

	for _, ref := range allObjs {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			u := unstructured.Unstructured{}
			u.SetAPIVersion(ref.APIVersion)
			u.SetKind(ref.Kind)
			u.SetName(ref.Name)

			if err := e.client.Get(ctx, types.NamespacedName{Name: u.GetName()}, &u); err != nil {
				if kerrors.IsNotFound(err) {
					// This is not expected, but still not an error for releasing objects.
					return nil
				}

				return errors.Wrapf(err, errFmtGetOwnedObject, u.GetKind(), u.GetName())
			}

			ors := u.GetOwnerReferences()
			found := false
			changed := false

			for i := range ors {
				if ors[i].UID == parent.GetUID() {
					found = true

					if ors[i].Controller != nil && *ors[i].Controller {
						ors[i].Controller = ptr.To(false)
						changed = true
					}

					break
				}
				// Note(turkenh): What if we cannot find our UID in the owner
				// references? This is not expected unless another party stripped
				// out ownerRefs. I believe this is a fairly unlikely scenario,
				// and we can ignore it for now especially considering that if that
				// happens active revision or the package itself will still take
				// over the ownership of such resources.
			}

			if !found {
				// Make sure the package revision exists as an owner.
				ors = append(ors, meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())))
				changed = true
			}

			if changed {
				u.SetOwnerReferences(ors)

				if err := e.client.Update(ctx, &u); err != nil {
					return errors.Wrapf(err, errFmtUpdateOwnedObject, u.GetKind(), u.GetName())
				}
			}

			return nil
		})
	}

	return g.Wait()
}

func (e *APIEstablisher) addLabels(objs []runtime.Object, parent v1.PackageRevision) error {
	commonLabels := parent.GetCommonLabels()

	for _, obj := range objs {
		// convert to resource.Object to be able to access metadata
		d, ok := obj.(resource.Object)
		if !ok {
			return errors.New(errConfResourceObject)
		}

		labels := d.GetLabels()
		if labels != nil {
			maps.Copy(labels, commonLabels)
		} else {
			d.SetLabels(commonLabels)
		}
	}

	return nil
}

func (e *APIEstablisher) addAnnotations(objs []runtime.Object, parent v1.PackageRevision) error {
	commonAnnotations := parent.GetCommonAnnotations()

	for _, obj := range objs {
		// convert to resource.Object to be able to access metadata
		d, ok := obj.(resource.Object)
		if !ok {
			return errors.New(errConfResourceObject)
		}

		annotations := d.GetAnnotations()
		if annotations != nil {
			maps.Copy(annotations, commonAnnotations)
		} else {
			d.SetAnnotations(commonAnnotations)
		}
	}

	return nil
}

func (e *APIEstablisher) validate(ctx context.Context, objs []runtime.Object, parent v1.PackageRevision, control bool) (allObjs []desiredApply, err error) { //nolint:gocognit // TODO(negz): Refactor this to break up complexity.
	var webhookTLSCert []byte
	if parentWithRuntime, ok := parent.(v1.PackageRevisionWithRuntime); ok && control {
		webhookTLSCert, err = e.getWebhookTLSCert(ctx, parentWithRuntime)
		if err != nil {
			return nil, err
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(e.MaxConcurrentPackageEstablishers)

	out := make(chan desiredApply, len(objs))
	for _, res := range objs {
		g.Go(func() error {
			// Assert desired object to resource.Object so that we can access its
			// metadata.
			desired, ok := res.(resource.Object)
			if !ok {
				return errors.New(errAssertResourceObj)
			}

			if control {
				if err := e.enrichControlledResource(res, webhookTLSCert, parent); err != nil {
					return err
				}
			}

			// Make a copy of the desired object to be populated with existing
			// object, if it exists.
			resCopy := res.DeepCopyObject()

			current, ok := resCopy.(client.Object)
			if !ok {
				return errors.New(errAssertClientObj)
			}

			err := e.client.Get(ctx, types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}, current)
			if resource.IgnoreNotFound(err) != nil {
				return err
			}

			// If the resource does not already exist, dry-run the same SSA intent
			// we will use to create it. We will not create a resource if we are not
			// going to control it, which prevents inactive revisions from racing
			// active revisions.
			if kerrors.IsNotFound(err) {
				if control {
					apply, _, err := e.prepareApply(nil, desired, parent, true)
					if err != nil {
						return err
					}
					if err := e.patch(ctx, apply.DeepCopy(), FieldOwnerAPIEstablisher, client.DryRunAll); err != nil {
						return err
					}

					select {
					case out <- desiredApply{Desired: desired, Reference: apply, Apply: apply, FieldOwner: FieldOwnerAPIEstablisher}:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}

				select {
				case out <- desiredApply{Desired: desired, Reference: desired}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			currentResource, ok := current.(resource.Object)
			if !ok {
				return errors.New(errAssertResourceObj)
			}

			apply, needed, err := e.prepareApply(currentResource, desired, parent, control)
			if err != nil {
				return err
			}
			if needed {
				owner := FieldOwnerAPIEstablisher
				if !control {
					owner = fieldOwnerAPIEstablisherOwnership
				}
				if err := e.patch(ctx, apply.DeepCopy(), owner, client.DryRunAll); err != nil {
					return err
				}

				select {
				case out <- desiredApply{Desired: desired, Reference: apply, Apply: apply, FieldOwner: owner}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			select {
			case out <- desiredApply{Desired: desired, Reference: currentResource}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	close(out)

	for obj := range out {
		allObjs = append(allObjs, obj)
	}

	return allObjs, nil
}

func (e *APIEstablisher) enrichControlledResource(res runtime.Object, webhookTLSCert []byte, parent v1.PackageRevision) error { //nolint:gocognit // just a switch
	// The generated webhook configurations have a static hard-coded name
	// that the developers of the providers can't affect. Here, we make sure
	// to distinguish one from the other by setting the name to the parent
	// since there is always a single ValidatingWebhookConfiguration and/or
	// single MutatingWebhookConfiguration object in a provider package.
	// See https://github.com/kubernetes-sigs/controller-tools/issues/658
	switch conf := res.(type) {
	case *admv1.ValidatingWebhookConfiguration:
		if len(webhookTLSCert) == 0 {
			return nil
		}

		if pkgRef, ok := GetPackageOwnerReference(parent); ok {
			conf.SetName(fmt.Sprintf("crossplane-%s-%s", strings.ToLower(pkgRef.Kind), pkgRef.Name))
		}

		for i := range conf.Webhooks {
			conf.Webhooks[i].ClientConfig.CABundle = webhookTLSCert
			if conf.Webhooks[i].ClientConfig.Service == nil {
				conf.Webhooks[i].ClientConfig.Service = &admv1.ServiceReference{}
			}

			conf.Webhooks[i].ClientConfig.Service.Name = parent.GetLabels()[v1.LabelParentPackage]
			conf.Webhooks[i].ClientConfig.Service.Namespace = e.namespace
			conf.Webhooks[i].ClientConfig.Service.Port = ptr.To[int32](ServicePort)
		}
	case *admv1.MutatingWebhookConfiguration:
		if len(webhookTLSCert) == 0 {
			return nil
		}

		if pkgRef, ok := GetPackageOwnerReference(parent); ok {
			conf.SetName(fmt.Sprintf("crossplane-%s-%s", strings.ToLower(pkgRef.Kind), pkgRef.Name))
		}

		for i := range conf.Webhooks {
			conf.Webhooks[i].ClientConfig.CABundle = webhookTLSCert
			if conf.Webhooks[i].ClientConfig.Service == nil {
				conf.Webhooks[i].ClientConfig.Service = &admv1.ServiceReference{}
			}

			conf.Webhooks[i].ClientConfig.Service.Name = parent.GetLabels()[v1.LabelParentPackage]
			conf.Webhooks[i].ClientConfig.Service.Namespace = e.namespace
			conf.Webhooks[i].ClientConfig.Service.Port = ptr.To[int32](ServicePort)
		}
	case *extv1.CustomResourceDefinition:
		if conf.Spec.Conversion != nil && conf.Spec.Conversion.Strategy == extv1.WebhookConverter {
			if len(webhookTLSCert) == 0 {
				return errors.New(errConversionWithNoWebhookCA)
			}

			if conf.Spec.Conversion.Webhook == nil {
				conf.Spec.Conversion.Webhook = &extv1.WebhookConversion{}
			}

			if conf.Spec.Conversion.Webhook.ClientConfig == nil {
				conf.Spec.Conversion.Webhook.ClientConfig = &extv1.WebhookClientConfig{}
			}

			if conf.Spec.Conversion.Webhook.ClientConfig.Service == nil {
				conf.Spec.Conversion.Webhook.ClientConfig.Service = &extv1.ServiceReference{}
			}

			conf.Spec.Conversion.Webhook.ClientConfig.CABundle = webhookTLSCert
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Name = parent.GetLabels()[v1.LabelParentPackage]
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Namespace = e.namespace
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Port = ptr.To[int32](ServicePort)
		}
	case *v1alpha1.ManagedResourceDefinition:
		if conf.Spec.Conversion != nil && conf.Spec.Conversion.Strategy == extv1.WebhookConverter {
			if len(webhookTLSCert) == 0 {
				return errors.New(errConversionWithNoWebhookCA)
			}

			if conf.Spec.Conversion.Webhook == nil {
				conf.Spec.Conversion.Webhook = &extv1.WebhookConversion{}
			}

			if conf.Spec.Conversion.Webhook.ClientConfig == nil {
				conf.Spec.Conversion.Webhook.ClientConfig = &extv1.WebhookClientConfig{}
			}

			if conf.Spec.Conversion.Webhook.ClientConfig.Service == nil {
				conf.Spec.Conversion.Webhook.ClientConfig.Service = &extv1.ServiceReference{}
			}

			conf.Spec.Conversion.Webhook.ClientConfig.CABundle = webhookTLSCert
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Name = parent.GetLabels()[v1.LabelParentPackage]
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Namespace = e.namespace
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Port = ptr.To[int32](ServicePort)
		}
	}

	return nil
}

// getWebhookTLSCert returns the TLS certificate of the webhook server if the
// revision has a TLS server secret name.
func (e *APIEstablisher) getWebhookTLSCert(ctx context.Context, parentWithRuntime v1.PackageRevisionWithRuntime) (webhookTLSCert []byte, err error) {
	tlsServerSecretName := parentWithRuntime.GetObservedTLSServerSecretName()
	if tlsServerSecretName == nil {
		return nil, errors.New(errWebhookSecretNotPresent)
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{Name: *tlsServerSecretName, Namespace: e.namespace}

	err = e.client.Get(ctx, nn, s)
	if err != nil {
		return nil, errors.Wrap(err, errGetWebhookTLSSecret)
	}

	if len(s.Data["tls.crt"]) == 0 {
		return nil, errors.New(errWebhookSecretWithoutCABundle)
	}

	webhookTLSCert = s.Data["tls.crt"]

	return webhookTLSCert, nil
}

func (e *APIEstablisher) establish(ctx context.Context, allObjs []desiredApply) ([]xpv2.TypedReference, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(e.MaxConcurrentPackageEstablishers)

	out := make(chan xpv2.TypedReference, len(allObjs))
	for _, da := range allObjs {
		g.Go(func() error {
			if da.Apply != nil {
				if err := e.patch(ctx, da.Apply, da.FieldOwner); err != nil {
					return err
				}
			}

			select {
			case out <- *meta.TypedReferenceTo(da.Reference, da.Desired.GetObjectKind().GroupVersionKind()):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	close(out)

	resourceRefs := []xpv2.TypedReference{}
	for ref := range out {
		resourceRefs = append(resourceRefs, ref)
	}

	return resourceRefs, nil
}

// prepareApply returns the SSA intent for a package object and whether applying
// it would change fields owned by the establisher. Controlled objects include
// the package payload. Owned objects include only owner references so inactive
// revisions cannot modify package or user data.
func (e *APIEstablisher) prepareApply(current, desired, parent resource.Object, control bool) (*unstructured.Unstructured, bool, error) {
	apply, err := applyObject(desired, control)
	if err != nil {
		return nil, false, err
	}

	if current == nil {
		if !control {
			return apply, false, nil
		}

		refs := []metav1.OwnerReference{
			meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())),
		}
		if pkgRef, ok := GetPackageOwnerReference(parent); ok {
			pkgRef.Controller = ptr.To(false)
			refs = append(refs, pkgRef)
		}
		apply.SetOwnerReferences(refs)

		if err := setApplyHash(apply); err != nil {
			return nil, false, err
		}
		return apply, true, nil
	}

	apply.SetOwnerReferences(slices.Clone(current.GetOwnerReferences()))
	if pkgRef, ok := GetPackageOwnerReference(parent); ok {
		pkgRef.Controller = ptr.To(false)
		meta.AddOwnerReference(apply, pkgRef)
	}

	if control {
		if err := meta.AddControllerReference(apply, meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))); err != nil {
			return nil, false, err
		}

		// The activation policy controller, not the package establisher, owns
		// state after an MRD has been created. Omitting it also avoids comparing
		// the package's absent value with the API server's Inactive default.
		if _, ok := desired.(*v1alpha1.ManagedResourceDefinition); ok {
			unstructured.RemoveNestedField(apply.Object, "spec", "state")
		}
		if err := setApplyHash(apply); err != nil {
			return nil, false, err
		}
	} else {
		meta.AddOwnerReference(apply, meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())))
	}

	if !reflect.DeepEqual(current.GetOwnerReferences(), apply.GetOwnerReferences()) {
		return apply, true, nil
	}
	if !control {
		return apply, false, nil
	}

	cu, err := runtime.DefaultUnstructuredConverter.ToUnstructured(current)
	if err != nil {
		return nil, false, errors.Wrap(err, "cannot convert current object to unstructured")
	}
	unstructured.RemoveNestedField(cu, "metadata", "ownerReferences")
	want := apply.DeepCopy()
	unstructured.RemoveNestedField(want.Object, "metadata", "ownerReferences")
	// Typed objects returned by controller-runtime are not guaranteed to have
	// TypeMeta populated. Identity was already established by Get, so it is not
	// part of the drift comparison.
	delete(cu, "apiVersion")
	delete(cu, "kind")
	delete(want.Object, "apiVersion")
	delete(want.Object, "kind")

	return apply, !jsonSubset(want.Object, cu), nil
}

func applyObject(desired resource.Object, control bool) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desired)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert desired object to unstructured")
	}

	if !control {
		metadata := map[string]any{
			"name": desired.GetName(),
		}
		if desired.GetNamespace() != "" {
			metadata["namespace"] = desired.GetNamespace()
		}
		u = map[string]any{
			"apiVersion": desired.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			"kind":       desired.GetObjectKind().GroupVersionKind().Kind,
			"metadata":   metadata,
		}
	}

	for _, field := range []string{
		"creationTimestamp", "deletionGracePeriodSeconds", "deletionTimestamp",
		"generation", "managedFields", "resourceVersion", "selfLink", "uid",
	} {
		unstructured.RemoveNestedField(u, "metadata", field)
	}
	unstructured.RemoveNestedField(u, "status")

	return &unstructured.Unstructured{Object: u}, nil
}

func setApplyHash(apply *unstructured.Unstructured) error {
	intent := apply.DeepCopy()
	unstructured.RemoveNestedField(intent.Object, "metadata", "ownerReferences")
	annotations := intent.GetAnnotations()
	delete(annotations, annotationKeyEstablisherHash)
	if len(annotations) == 0 {
		unstructured.RemoveNestedField(intent.Object, "metadata", "annotations")
	} else {
		intent.SetAnnotations(annotations)
	}

	b, err := json.Marshal(intent.Object)
	if err != nil {
		return errors.Wrap(err, "cannot marshal apply intent")
	}

	annotations = apply.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[annotationKeyEstablisherHash] = fmt.Sprintf("%x", sha256.Sum256(b))
	apply.SetAnnotations(annotations)

	return nil
}

// jsonSubset reports whether all desired fields are present with equal values
// in current. Extra current fields are ignored because they may be API defaults
// or fields owned by another SSA manager.
func jsonSubset(desired, current any) bool {
	switch d := desired.(type) {
	case map[string]any:
		c, ok := current.(map[string]any)
		if !ok {
			return false
		}
		for k, dv := range d {
			cv, ok := c[k]
			if !ok {
				if isJSONEmpty(dv) {
					continue
				}
				return false
			}
			if !jsonSubset(dv, cv) {
				return false
			}
		}
		return true
	case []any:
		c, ok := current.([]any)
		if !ok || len(d) != len(c) {
			return false
		}
		for i := range d {
			if !jsonSubset(d[i], c[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(desired, current)
	}
}

func isJSONEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		return len(x) == 0
	case []any:
		return len(x) == 0
	}
	return false
}

func (e *APIEstablisher) patch(ctx context.Context, obj client.Object, owner string, opts ...client.PatchOption) error {
	opts = append(opts, client.ForceOwnership, client.FieldOwner(owner))

	// Force ownership deliberately handles objects previously written by the
	// classic Update path (typically manager "crossplane"). It transfers only
	// fields present in our apply intent; omitted defaults and user fields stay
	// with their existing manager.
	//
	//nolint:staticcheck // Client.Apply requires typed apply configurations.
	return e.client.Patch(ctx, obj, client.Apply, opts...)
}

// GetPackageOwnerReference returns the owner reference that points to the owner
// package of given revision, if it can find one.
func GetPackageOwnerReference(rev resource.Object) (metav1.OwnerReference, bool) {
	name := rev.GetLabels()[v1.LabelParentPackage]
	for _, owner := range rev.GetOwnerReferences() {
		if owner.Name == name {
			return owner, true
		}
	}

	return metav1.OwnerReference{}, false
}

// FilteringEstablisher wraps another establisher, but filters the objects that
// get passed to it so that only certain kinds are allowed.
type FilteringEstablisher struct {
	wrap Establisher
	gks  []schema.GroupKind
}

// NewFilteringEstablisher creates a new FilteringEstablisher.
func NewFilteringEstablisher(wrap Establisher, gks ...schema.GroupKind) *FilteringEstablisher {
	return &FilteringEstablisher{
		wrap: wrap,
		gks:  gks,
	}
}

// Establish filters objects, then uses the wrapped establisher to establish
// them.
func (e *FilteringEstablisher) Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv2.TypedReference, error) {
	filtered := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		if slices.Contains(e.gks, obj.GetObjectKind().GroupVersionKind().GroupKind()) {
			filtered = append(filtered, obj)
		}
	}

	return e.wrap.Establish(ctx, filtered, parent, control)
}

// ReleaseObjects uses the wrapped establisher to release objects.
func (e *FilteringEstablisher) ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error {
	return e.wrap.ReleaseObjects(ctx, parent)
}
