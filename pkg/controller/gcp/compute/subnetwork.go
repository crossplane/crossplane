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

package compute

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	googlecompute "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	gcpapis "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	gcpclients "github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/subnetwork"
)

const (
	// Error strings.
	errNotSubnetwork              = "managed resource is not a Subnetwork resource"
	errInsufficientSubnetworkSpec = "name or region for network external resource is not provided"

	errUpdateSubnetworkFailed = "update of Subnetwork resource has failed"
	errCreateSubnetworkFailed = "creation of Subnetwork resource has failed"
	errDeleteSubnetworkFailed = "deletion of Subnetwork resource has failed"
)

// SubnetworkController is the controller for Subnetwork CRD.
type SubnetworkController struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *SubnetworkController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(computev1alpha1.SubnetworkGroupVersionKind),
		resource.WithExternalConnecter(&subnetworkConnector{kube: mgr.GetClient()}),
		resource.WithManagedConnectionPublishers())

	name := strings.ToLower(fmt.Sprintf("%s.%s", computev1alpha1.SubnetworkKindAPIVersion, computev1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Subnetwork{}).
		Complete(r)
}

type subnetworkConnector struct {
	kube         client.Client
	newServiceFn func(ctx context.Context, opts ...option.ClientOption) (*googlecompute.Service, error)
}

func (c *subnetworkConnector) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Subnetwork)
	if !ok {
		return nil, errors.New(errNotSubnetwork)
	}
	// TODO(muvaf): we do not yet have a way for configure the Spec with defaults for statically provisioned resources
	// such as this. Setting it directly here does not work since managed reconciler issues updates only to
	// `status` subresource. We require name to be given until we have a pre-process hook like configurator in Claim
	// reconciler
	if cr.Spec.Name == "" || cr.Spec.Region == "" {
		return nil, errors.New(errInsufficientSubnetworkSpec)
	}

	provider := &gcpapis.Provider{}
	n := meta.NamespacedNameOf(cr.Spec.ProviderReference)
	if err := c.kube.Get(ctx, n, provider); err != nil {
		return nil, errors.Wrap(err, errProviderNotRetrieved)
	}
	secret := &v1.Secret{}
	name := meta.NamespacedNameOf(&v1.ObjectReference{
		Name:      provider.Spec.Secret.Name,
		Namespace: provider.Namespace,
	})
	if err := c.kube.Get(ctx, name, secret); err != nil {
		return nil, errors.Wrap(err, errProviderSecretNotRetrieved)
	}

	if c.newServiceFn == nil {
		c.newServiceFn = googlecompute.NewService
	}
	s, err := c.newServiceFn(ctx,
		option.WithCredentialsJSON(secret.Data[provider.Spec.Secret.Key]),
		option.WithScopes(googlecompute.ComputeScope))
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	return &subnetworkExternal{Service: s, projectID: provider.Spec.ProjectID}, nil
}

type subnetworkExternal struct {
	*googlecompute.Service
	projectID string
}

func (c *subnetworkExternal) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Subnetwork)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotSubnetwork)
	}
	observed, err := c.Subnetworks.Get(c.projectID, cr.Spec.Region, cr.Spec.Name).Context(ctx).Do()
	if gcpclients.IsErrorNotFound(err) {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, err
	}
	cr.Status.GCPSubnetworkStatus = subnetwork.GenerateGCPSubnetworkStatus(observed)
	cr.Status.SetConditions(runtimev1alpha1.Available())
	return resource.ExternalObservation{
		ResourceExists: true,
	}, nil
}

func (c *subnetworkExternal) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Subnetwork)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotSubnetwork)
	}
	_, err := c.Subnetworks.Insert(c.projectID, cr.Spec.Region, subnetwork.GenerateSubnetwork(cr.Spec.GCPSubnetworkSpec)).
		Context(ctx).
		Do()
	if gcpclients.IsErrorAlreadyExists(err) {
		return resource.ExternalCreation{}, nil
	}
	cr.Status.SetConditions(runtimev1alpha1.Creating())
	return resource.ExternalCreation{}, errors.Wrap(err, errCreateSubnetworkFailed)
}

func (c *subnetworkExternal) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Subnetwork)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotSubnetwork)
	}
	if cr.Spec.IsSameAs(cr.Status.GCPSubnetworkStatus) {
		return resource.ExternalUpdate{}, nil
	}
	subnetworkBody := subnetwork.GenerateSubnetwork(cr.Spec.GCPSubnetworkSpec)
	// Fingerprint from the last GET is required for updates.
	subnetworkBody.Fingerprint = cr.Status.Fingerprint
	// The API rejects region to be updated, in fact, it rejects the update when this field is even included. Calm down.
	subnetworkBody.Region = ""
	_, err := c.Subnetworks.Patch(c.projectID, cr.Spec.Region, cr.Spec.Name, subnetworkBody).Context(ctx).Do()
	return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateSubnetworkFailed)
}

func (c *subnetworkExternal) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Subnetwork)
	if !ok {
		return errors.New(errNotSubnetwork)
	}
	_, err := c.Subnetworks.Delete(c.projectID, cr.Spec.Region, cr.Spec.Name).Context(ctx).Do()
	if gcpclients.IsErrorNotFound(err) {
		return nil
	}
	cr.Status.SetConditions(runtimev1alpha1.Deleting())
	return errors.Wrap(err, errDeleteSubnetworkFailed)
}
