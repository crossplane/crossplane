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
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"
	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

const (
	// maxConcurrentEstablishers specifies the maximum number of goroutines to use
	// for establishing resources.
	maxConcurrentEstablishers = 10
)

const (
	errAssertResourceObj            = "cannot assert object to resource.Object"
	errAssertClientObj              = "cannot assert object to client.Object"
	errConversionWithNoWebhookCA    = "cannot deploy a CRD with webhook conversion strategy without having a TLS bundle"
	errGetWebhookTLSSecret          = "cannot get webhook tls secret"
	errWebhookSecretWithoutCABundle = "the value for the key tls.crt cannot be empty"
	errFmtGetOwnedObject            = "cannot get owned object: %s/%s"
	errFmtUpdateOwnedObject         = "cannot update owned object: %s/%s"
)

// An Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
	ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error
}

// NewNopEstablisher returns a new NopEstablisher.
func NewNopEstablisher() *NopEstablisher {
	return &NopEstablisher{}
}

// NopEstablisher does nothing.
type NopEstablisher struct{}

// Establish does nothing.
func (*NopEstablisher) Establish(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
	return nil, nil
}

// ReleaseObjects does nothing.
func (*NopEstablisher) ReleaseObjects(_ context.Context, _ v1.PackageRevision) error {
	return nil
}

// APIEstablisher establishes control or ownership of resources in the API
// server for a parent.
type APIEstablisher struct {
	client    client.Client
	namespace string
}

// NewAPIEstablisher creates a new APIEstablisher.
func NewAPIEstablisher(client client.Client, namespace string) *APIEstablisher {
	return &APIEstablisher{
		client:    client,
		namespace: namespace,
	}
}

// currentDesired caches resources while checking for control or ownership so
// that they do not have to be fetched from the API server again when control or
// ownership is established.
type currentDesired struct {
	Current resource.Object
	Desired resource.Object
	Exists  bool
}

// Establish checks that control or ownership of resources can be established by
// parent, then establishes it.
func (e *APIEstablisher) Establish(ctx context.Context, objs []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error) {
	err := e.addLabels(objs, parent)
	if err != nil {
		return nil, err
	}
	allObjs, err := e.validate(ctx, objs, parent, control)
	if err != nil {
		return nil, err
	}

	resourceRefs, err := e.establish(ctx, allObjs, parent, control)
	if err != nil {
		return nil, err
	}
	return resourceRefs, nil
}

// ReleaseObjects removes control of owned resources in the API server for a
// package revision.
func (e *APIEstablisher) ReleaseObjects(ctx context.Context, parent v1.PackageRevision) error { //nolint:gocyclo // complexity coming from parallelism.
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
	g.SetLimit(maxConcurrentEstablishers)
	for _, ref := range allObjs {
		ref := ref // Pin the loop variable.
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
			changed := false
			for i := range ors {
				if ors[i].UID == parent.GetUID() {
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
			for key, value := range commonLabels {
				labels[key] = value
			}
		} else {
			d.SetLabels(commonLabels)
		}
	}
	return nil
}

func (e *APIEstablisher) validate(ctx context.Context, objs []runtime.Object, parent v1.PackageRevision, control bool) (allObjs []currentDesired, err error) { //nolint:gocyclo // TODO(negz): Refactor this to break up complexity.
	var webhookTLSCert []byte
	if parentWithRuntime, ok := parent.(v1.PackageRevisionWithRuntime); ok && control {
		webhookTLSCert, err = e.getWebhookTLSCert(ctx, parentWithRuntime)
		if err != nil {
			return nil, err
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentEstablishers)
	out := make(chan currentDesired, len(objs))
	for _, res := range objs {
		res := res // Pin the range variable before using it in a Goroutine.
		g.Go(func() error {
			// Assert desired object to resource.Object so that we can access its
			// metadata.
			d, ok := res.(resource.Object)
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
			err := e.client.Get(ctx, types.NamespacedName{Name: d.GetName(), Namespace: d.GetNamespace()}, current)
			if resource.IgnoreNotFound(err) != nil {
				return err
			}

			// If resource does not already exist, we must attempt to dry run create
			// it.
			if kerrors.IsNotFound(err) {
				// We will not create a resource if we are not going to control it,
				// so we don't need to check with dry run.
				if control {
					if err := e.create(ctx, d, parent, client.DryRunAll); err != nil {
						return err
					}
				}
				// Add to objects as not existing.
				select {
				case out <- currentDesired{Desired: d, Current: nil, Exists: false}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			c := current.(resource.Object)
			if err := e.update(ctx, c, d, parent, control, client.DryRunAll); err != nil {
				return err
			}
			// Add to objects as existing.
			select {
			case out <- currentDesired{Desired: d, Current: c, Exists: true}:
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

func (e *APIEstablisher) enrichControlledResource(res runtime.Object, webhookTLSCert []byte, parent v1.PackageRevision) error { //nolint:gocyclo // just a switch
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
			conf.Webhooks[i].ClientConfig.Service.Port = ptr.To[int32](servicePort)
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
			conf.Webhooks[i].ClientConfig.Service.Port = ptr.To[int32](servicePort)
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
			conf.Spec.Conversion.Webhook.ClientConfig.Service.Port = ptr.To[int32](servicePort)
		}
	}
	return nil
}

// getWebhookTLSCert returns the TLS certificate of the webhook server if the
// revision has a TLS server secret name.
func (e *APIEstablisher) getWebhookTLSCert(ctx context.Context, parentWithRuntime v1.PackageRevisionWithRuntime) (webhookTLSCert []byte, err error) {
	tlsServerSecretName := parentWithRuntime.GetTLSServerSecretName()
	if tlsServerSecretName == nil {
		return nil, nil
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

func (e *APIEstablisher) establish(ctx context.Context, allObjs []currentDesired, parent client.Object, control bool) ([]xpv1.TypedReference, error) { //nolint:gocyclo // Only slightly over (12).
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentEstablishers)
	out := make(chan xpv1.TypedReference, len(allObjs))
	for _, cd := range allObjs {
		cd := cd // Pin the loop variable.
		g.Go(func() error {
			if !cd.Exists {
				// Only create a missing resource if we are going to control it.
				// This prevents an inactive revision from racing to create a
				// resource before an active revision of the same parent.
				if control {
					if err := e.create(ctx, cd.Desired, parent); err != nil {
						return err
					}
				}
				select {
				case out <- *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if err := e.update(ctx, cd.Current, cd.Desired, parent, control); err != nil {
				return err
			}
			select {
			case out <- *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()):
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
	resourceRefs := []xpv1.TypedReference{}
	for ref := range out {
		resourceRefs = append(resourceRefs, ref)
	}
	return resourceRefs, nil
}

func (e *APIEstablisher) create(ctx context.Context, obj resource.Object, parent resource.Object, opts ...client.CreateOption) error {
	refs := []metav1.OwnerReference{
		meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())),
	}
	// We add the parent as `owner` of the resources so that the resource doesn't
	// get deleted when the new revision doesn't include it in order not to lose
	// user data, such as custom resources of an old CRD.
	if pkgRef, ok := GetPackageOwnerReference(parent); ok {
		pkgRef.Controller = ptr.To(false)
		refs = append(refs, pkgRef)
	}
	// Overwrite any owner references on the desired object.
	obj.SetOwnerReferences(refs)
	return e.client.Create(ctx, obj, opts...)
}

func (e *APIEstablisher) update(ctx context.Context, current, desired resource.Object, parent resource.Object, control bool, opts ...client.UpdateOption) error {
	// We add the parent as `owner` of the resources so that the resource doesn't
	// get deleted when the new revision doesn't include it in order not to lose
	// user data, such as custom resources of an old CRD.
	if pkgRef, ok := GetPackageOwnerReference(parent); ok {
		pkgRef.Controller = ptr.To(false)
		meta.AddOwnerReference(current, pkgRef)
	}

	if !control {
		meta.AddOwnerReference(current, meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())))
		return e.client.Update(ctx, current, opts...)
	}

	// If desire is to control object, we attempt to update the object by
	// setting the desired owner references equal to that of the current, adding
	// a controller reference to the parent, and setting the desired resource
	// version to that of the current.
	desired.SetOwnerReferences(current.GetOwnerReferences())
	if err := meta.AddControllerReference(desired, meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))); err != nil {
		return err
	}
	desired.SetResourceVersion(current.GetResourceVersion())
	return e.client.Update(ctx, desired, opts...)
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
