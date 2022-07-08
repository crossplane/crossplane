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
	"math"
	"strings"
	"sync"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
)

const (
	// maxConcurrentUpdates specifies the maximum number of goroutines to use
	// for updating resources.
	maxConcurrentUpdates = 100
)
const (
	errAssertResourceObj            = "cannot assert object to resource.Object"
	errAssertClientObj              = "cannot assert object to client.Object"
	errConversionWithNoWebhookCA    = "cannot deploy a CRD with webhook conversion strategy without having a TLS bundle"
	errGetWebhookTLSSecret          = "cannot get webhook tls secret"
	errWebhookSecretWithoutCABundle = "the value for the key tls.crt cannot be empty"
)

// An Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
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

// Result of the execution of the executeEach function.
type executionResult int

const (
	// stopExecution stop processing the batch
	stopExecution executionResult = iota
	// continueExecution process the next element in the batch
	continueExecution
)

// A function to execute for each element in a slice.
type executeEach func(index int) executionResult

// run executeEach function for each element in the slice concurrently, waits for processing to finish.
func (callable executeEach) concurrently(maxParallelism, numElements int) {
	// calculate the number of items in each batch
	perBatch := int(math.Ceil(float64(numElements) / float64(maxParallelism)))
	wg := &sync.WaitGroup{}
	for start := 0; start < numElements; start += perBatch {
		end := start + perBatch
		if end > numElements {
			end = numElements
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for index := start; index < end; index++ {
				if callable(index) == stopExecution {
					return
				}
			}
		}(start, end)
	}
	wg.Wait()
}

// Establish checks that control or ownership of resources can be established by
// parent, then establishes it.
func (e *APIEstablisher) Establish(ctx context.Context, objs []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error) { // nolint:gocyclo
	allObjs := []currentDesired{}
	resourceRefs := []xpv1.TypedReference{}
	var webhookTLSCert []byte
	if parent.GetWebhookTLSSecretName() != nil {
		s := &corev1.Secret{}
		nn := types.NamespacedName{Name: *parent.GetWebhookTLSSecretName(), Namespace: e.namespace}
		if err := e.client.Get(ctx, nn, s); err != nil {
			return nil, errors.Wrap(err, errGetWebhookTLSSecret)
		}
		if len(s.Data["tls.crt"]) == 0 {
			return nil, errors.New(errWebhookSecretWithoutCABundle)
		}
		webhookTLSCert = s.Data["tls.crt"]
	}
	var errEx error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	executionError := func(err error) executionResult {
		errEx = err
		cancel()
		return stopExecution
	}
	objectStatusCh := make(chan currentDesired, len(objs))
	executeEach(func(i int) executionResult {
		if ctx.Err() != nil {
			return stopExecution
		}
		res := objs[i]
		// Assert desired object to resource.Object so that we can access its
		// metadata.
		d, ok := res.(resource.Object)
		if !ok {
			return executionError(errors.New(errAssertResourceObj))
		}

		// The generated webhook configurations have a static hard-coded name
		// that the developers of the providers can't affect. Here, we make sure
		// to distinguish one from the other by setting the name to the parent
		// since there is always a single ValidatingWebhookConfiguration and/or
		// single MutatingWebhookConfiguration object in a provider package.
		// See https://github.com/kubernetes-sigs/controller-tools/issues/658
		switch conf := res.(type) {
		case *admv1.ValidatingWebhookConfiguration:
			if len(webhookTLSCert) == 0 {
				return continueExecution
			}
			if pkgRef, ok := GetPackageOwnerReference(parent); ok {
				conf.SetName(fmt.Sprintf("crossplane-%s-%s", strings.ToLower(pkgRef.Kind), pkgRef.Name))
			}
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = webhookTLSCert
				if conf.Webhooks[i].ClientConfig.Service == nil {
					conf.Webhooks[i].ClientConfig.Service = &admv1.ServiceReference{}
				}
				conf.Webhooks[i].ClientConfig.Service.Name = parent.GetName()
				conf.Webhooks[i].ClientConfig.Service.Namespace = e.namespace
				conf.Webhooks[i].ClientConfig.Service.Port = pointer.Int32(webhookPort)
			}
		case *admv1.MutatingWebhookConfiguration:
			if len(webhookTLSCert) == 0 {
				return continueExecution
			}
			if pkgRef, ok := GetPackageOwnerReference(parent); ok {
				conf.SetName(fmt.Sprintf("crossplane-%s-%s", strings.ToLower(pkgRef.Kind), pkgRef.Name))
			}
			for i := range conf.Webhooks {
				conf.Webhooks[i].ClientConfig.CABundle = webhookTLSCert
				if conf.Webhooks[i].ClientConfig.Service == nil {
					conf.Webhooks[i].ClientConfig.Service = &admv1.ServiceReference{}
				}
				conf.Webhooks[i].ClientConfig.Service.Name = parent.GetName()
				conf.Webhooks[i].ClientConfig.Service.Namespace = e.namespace
				conf.Webhooks[i].ClientConfig.Service.Port = pointer.Int32(webhookPort)
			}
		case *extv1.CustomResourceDefinition:
			if conf.Spec.Conversion != nil && conf.Spec.Conversion.Strategy == extv1.WebhookConverter {
				if len(webhookTLSCert) == 0 {
					return executionError(errors.New(errConversionWithNoWebhookCA))
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
				conf.Spec.Conversion.Webhook.ClientConfig.Service.Name = parent.GetName()
				conf.Spec.Conversion.Webhook.ClientConfig.Service.Namespace = e.namespace
				conf.Spec.Conversion.Webhook.ClientConfig.Service.Port = pointer.Int32(webhookPort)
			}
		}

		// Make a copy of the desired object to be populated with existing
		// object, if it exists.
		copy := res.DeepCopyObject()
		current, ok := copy.(client.Object)
		if !ok {
			return executionError(errors.New(errAssertClientObj))
		}
		err := e.client.Get(ctx, types.NamespacedName{Name: d.GetName(), Namespace: d.GetNamespace()}, current)
		if resource.IgnoreNotFound(err) != nil {
			return executionError(err)
		}

		// If resource does not already exist, we must attempt to dry run create
		// it.
		if kerrors.IsNotFound(err) {
			// Add to objects as not existing.
			objectStatusCh <- currentDesired{
				Desired: d,
				Current: nil,
				Exists:  false,
			}
			// We will not create a resource if we are not going to control it,
			// so we don't need to check with dry run.
			if control {
				if err := e.create(ctx, d, parent, client.DryRunAll); err != nil {
					return executionError(err)
				}
			}
			return continueExecution
		}

		c := current.(resource.Object)
		// Add to objects as existing.
		objectStatusCh <- currentDesired{
			Desired: d,
			Current: c,
			Exists:  true,
		}

		if err := e.update(ctx, c, d, parent, control, client.DryRunAll); err != nil {
			return executionError(err)
		}
		return continueExecution
	}).concurrently(maxConcurrentUpdates, len(objs))
	close(objectStatusCh)
	if errEx != nil {
		return nil, errEx
	}

	for obj := range objectStatusCh {
		allObjs = append(allObjs, obj)
	}
	resourceRefsCh := make(chan xpv1.TypedReference, len(allObjs))
	executeEach(func(i int) executionResult {
		cd := allObjs[i]
		if ctx.Err() != nil {
			return stopExecution
		}
		if !cd.Exists {
			// Only create a missing resource if we are going to control it.
			// This prevents an inactive revision from racing to create a
			// resource before an active revision of the same parent.
			if control {
				if err := e.create(ctx, cd.Desired, parent); err != nil {
					return executionError(err)
				}
			}
			resourceRefsCh <- *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind())
			return continueExecution
		}

		if err := e.update(ctx, cd.Current, cd.Desired, parent, control); err != nil {
			return executionError(err)
		}
		resourceRefsCh <- *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind())
		return continueExecution
	}).concurrently(maxConcurrentUpdates, len(allObjs))
	close(resourceRefsCh)
	if errEx != nil {
		return nil, errEx
	}

	for resourceRef := range resourceRefsCh {
		resourceRefs = append(resourceRefs, resourceRef)
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
		pkgRef.Controller = pointer.BoolPtr(false)
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
		pkgRef.Controller = pointer.BoolPtr(false)
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
