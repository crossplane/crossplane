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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	computev1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/compute/v1alpha1"
	gcpapis "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	gcpclients "github.com/crossplaneio/crossplane/pkg/clients/gcp"
)

const (
	// Error strings.
	errNewClient    = "cannot create new Compute Service"
	errNotVPC       = "managed resource is not a Network resource"
	errNameNotGiven = "name for external resource is not provided"
)

// NetworkController is the controller for Network CRD.
type NetworkController struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *NetworkController) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(computev1alpha1.NetworkGroupVersionKind),
		resource.WithExternalConnecter(&connector{client: mgr.GetClient()}),
		resource.WithManagedConnectionPublishers())

	name := strings.ToLower(fmt.Sprintf("%s.%s", computev1alpha1.NetworkKindAPIVersion, computev1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Network{}).
		Complete(r)
}

type connector struct {
	client      client.Client
	newClientFn func(ctx context.Context, opts ...option.ClientOption) (*googlecompute.Service, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return nil, errors.New(errNotVPC)
	}
	// TODO(muvaf): we do not yet have a way for configure the Spec with defaults for statically provisioned resources
	// such as this. Setting it directly here does not work since managed reconciler issues updates only to
	// `status` subresource. We require name to be given until we have a pre-process hook like configurator in Claim
	// reconciler
	if cr.Spec.Name == "" {
		return nil, errors.New(errNameNotGiven)
	}

	provider := &gcpapis.Provider{}
	n := meta.NamespacedNameOf(cr.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, provider); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	gcpCreds, err := gcpclients.ProviderCredentials(c.client, provider, googlecompute.ComputeScope)
	if err != nil {
		return nil, err
	}
	if c.newClientFn == nil {
		c.newClientFn = googlecompute.NewService
	}
	s, err := c.newClientFn(ctx, option.WithCredentials(gcpCreds))
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	return &external{networks: s.Networks, projectID: provider.Spec.ProjectID}, nil
}

type external struct {
	networks  *googlecompute.NetworksService
	projectID string
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotVPC)
	}
	observed, err := c.networks.Get(c.projectID, cr.Spec.Name).Context(ctx).Do()
	if gcpclients.IsErrorNotFound(err) {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, err
	}
	cr.Status.GCPNetworkStatus = *computev1alpha1.GenerateGCPNetworkStatus(*observed)
	return resource.ExternalObservation{
		ResourceExists: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotVPC)
	}
	if _, err := c.networks.Insert(c.projectID, computev1alpha1.GenerateGCPNetworkSpec(cr.Spec.GCPNetworkSpec)).
		Context(ctx).
		Do(); err != nil {
		return resource.ExternalCreation{}, err
	}
	return resource.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotVPC)
	}
	if _, err := c.networks.Patch(
		c.projectID,
		cr.Spec.Name,
		computev1alpha1.GenerateGCPNetworkSpec(cr.Spec.GCPNetworkSpec)).
		Context(ctx).
		Do(); err != nil {
		return resource.ExternalUpdate{}, err
	}
	return resource.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Network)
	if !ok {
		return errors.New(errNotVPC)
	}
	if _, err := c.networks.Delete(c.projectID, cr.Spec.Name).
		Context(ctx).
		Do(); !gcpclients.IsErrorNotFound(err) && err != nil {
		return err
	}
	return nil
}
