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

package subnet

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane/azure/apis/network/v1alpha1"
	azurev1alpha1 "github.com/crossplaneio/crossplane/azure/apis/v1alpha1"
	azureclients "github.com/crossplaneio/crossplane/pkg/clients/azure"
	"github.com/crossplaneio/crossplane/pkg/clients/azure/network"
)

// Error strings.
const (
	errNewClient    = "cannot create new Subnet"
	errNotSubnet    = "managed resource is not an Subnet"
	errCreateSubnet = "cannot create Subnet"
	errUpdateSubnet = "cannot update Subnet"
	errGetSubnet    = "cannot get Subnet"
	errDeleteSubnet = "cannot delete Subnet"
)

// Controller is responsible for adding the Subnet
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Subnet Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the Controller and
// start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(v1alpha1.SubnetGroupVersionKind),
		resource.WithManagedConnectionPublishers(),
		resource.WithExternalConnecter(&connecter{client: mgr.GetClient()}))

	name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.SubnetKind, v1alpha1.Group))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Subnet{}).
		Complete(r)
}

type connecter struct {
	client      client.Client
	newClientFn func(ctx context.Context, credentials []byte) (network.SubnetsClient, error)
}

func (c *connecter) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	g, ok := mg.(*v1alpha1.Subnet)
	if !ok {
		return nil, errors.New(errNotSubnet)
	}

	p := &azurev1alpha1.Provider{}
	n := meta.NamespacedNameOf(g.Spec.ProviderReference)
	if err := c.client.Get(ctx, n, p); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider %s", n)
	}

	s := &corev1.Secret{}
	n = types.NamespacedName{Namespace: p.GetNamespace(), Name: p.Spec.Secret.Name}
	if err := c.client.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get provider secret %s", n)
	}
	newClientFn := network.NewSubnetsClient
	if c.newClientFn != nil {
		newClientFn = c.newClientFn
	}
	client, err := newClientFn(ctx, s.Data[p.Spec.Secret.Key])
	return &external{client: client}, errors.Wrap(err, errNewClient)
}

type external struct{ client network.SubnetsClient }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	s, ok := mg.(*v1alpha1.Subnet)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotSubnet)
	}

	az, err := e.client.Get(ctx, s.Spec.ResourceGroupName, s.Spec.VirtualNetworkName, s.Spec.Name, "")
	if azureclients.IsNotFound(err) {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errGetSubnet)
	}

	s.Status = network.SubnetStatusFromAzure(az)
	s.SetConditions(runtimev1alpha1.Available())

	o := resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}

	return o, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	s, ok := mg.(*v1alpha1.Subnet)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotSubnet)
	}

	s.Status.SetConditions(runtimev1alpha1.Creating())

	snet := network.NewSubnetParameters(s)
	if _, err := e.client.CreateOrUpdate(ctx, s.Spec.ResourceGroupName, s.Spec.VirtualNetworkName, s.Spec.Name, snet); err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errCreateSubnet)
	}

	return resource.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	s, ok := mg.(*v1alpha1.Subnet)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotSubnet)
	}

	az, err := e.client.Get(ctx, s.Spec.ResourceGroupName, s.Spec.VirtualNetworkName, s.Spec.Name, "")
	if err != nil {
		return resource.ExternalUpdate{}, errors.Wrap(err, errGetSubnet)
	}

	if network.SubnetNeedsUpdate(s, az) {
		snet := network.NewSubnetParameters(s)
		if _, err := e.client.CreateOrUpdate(ctx, s.Spec.ResourceGroupName, s.Spec.VirtualNetworkName, s.Spec.Name, snet); err != nil {
			return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateSubnet)
		}
	}
	return resource.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	s, ok := mg.(*v1alpha1.Subnet)
	if !ok {
		return errors.New(errNotSubnet)
	}

	mg.SetConditions(runtimev1alpha1.Deleting())

	_, err := e.client.Delete(ctx, s.Spec.ResourceGroupName, s.Spec.VirtualNetworkName, s.Spec.Name)
	return errors.Wrap(resource.Ignore(azureclients.IsNotFound, err), errDeleteSubnet)
}
