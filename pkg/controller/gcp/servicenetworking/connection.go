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

package servicenetworking

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	servicenetworking "google.golang.org/api/servicenetworking/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"

	"github.com/crossplaneio/crossplane/gcp/apis/servicenetworking/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/gcp/apis/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/connection"
)

// Error strings.
const (
	errGetProvider       = "cannot get provider"
	errGetProviderSecret = "cannot get provider secret"
	errNewClient         = "cannot create new Compute Service"
	errNotConnection     = "managed resource is not a Connection"
	errListConnections   = "cannot list external Connection resources"
	errGetNetwork        = "cannot get VPC Network"
	errCreateConnection  = "cannot create external Connection resource"
	errUpdateConnection  = "cannot update external Connection resource"
	errDeleteConnection  = "cannot delete external Connection resource"
)

// NOTE(negz): There is no 'Get' method for connections, only 'List', and the
// behaviour of the API is not well documented. I am assuming based on the docs
// and my observations of the API, Console, and Terraform implementation of this
// resource that:
//
// * You can only create connections for service
//   'services/servicenetworking.googleapis.com' via the API.
// * You cannot create multiple connections for service
//   'services/servicenetworking.googleapis.com' via the API.
// * Connections created via the API for service
//   'services/servicenetworking.googleapis.com' always produce a peering named
//   'servicenetworking-googleapis-com'.
//
// I note that when I create a MySQL instance with a private IP via the console
// I am prompted to create a new connection if one does not exist. This creates
// a connection for service 'services/servicenetworking.googleapis.com' with a
// peering (to a different VPC network) named 'cloudsql-mysql-googleapis-com'. I
// presume this is dark Google magic that is not exposed to API callers.
// https://cloud.google.com/service-infrastructure/docs/service-networking/reference/rest/v1/services.connections/list
const peeringName = "servicenetworking-googleapis-com"

// ConnectionController is the controller for Connection CRD.
type ConnectionController struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *ConnectionController) SetupWithManager(mgr ctrl.Manager) error {
	conn := &connector{
		client:               mgr.GetClient(),
		newCompute:           compute.NewService,
		newServiceNetworking: servicenetworking.NewService,
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.ConnectionKindAPIVersion, v1alpha1.Group))).
		For(&v1alpha1.Connection{}).
		Complete(resource.NewManagedReconciler(mgr,
			resource.ManagedKind(v1alpha1.ConnectionGroupVersionKind),
			resource.WithExternalConnecter(conn),
			resource.WithManagedConnectionPublishers()))
}

type connector struct {
	client               client.Client
	newCompute           func(ctx context.Context, opts ...option.ClientOption) (*compute.Service, error)
	newServiceNetworking func(ctx context.Context, opts ...option.ClientOption) (*servicenetworking.APIService, error)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (resource.ExternalClient, error) {
	ga, ok := mg.(*v1alpha1.Connection)
	if !ok {
		return nil, errors.New(errNotConnection)
	}

	p := &gcpv1alpha1.Provider{}
	if err := c.client.Get(ctx, meta.NamespacedNameOf(ga.Spec.ProviderReference), p); err != nil {
		return nil, errors.Wrap(err, errGetProvider)
	}
	s := &v1.Secret{}
	if err := c.client.Get(ctx, types.NamespacedName{Namespace: p.Namespace, Name: p.Spec.Secret.Name}, s); err != nil {
		return nil, errors.Wrap(err, errGetProviderSecret)
	}

	cmp, err := c.newCompute(ctx,
		option.WithCredentialsJSON(s.Data[p.Spec.Secret.Key]),
		option.WithScopes(compute.ComputeScope))
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	sn, err := c.newServiceNetworking(ctx,
		option.WithCredentialsJSON(s.Data[p.Spec.Secret.Key]),
		option.WithScopes(servicenetworking.ServiceManagementScope))
	return &external{sn: sn, compute: cmp, projectID: p.Spec.ProjectID}, errors.Wrap(err, errNewClient)
}

type external struct {
	compute   *compute.Service
	sn        *servicenetworking.APIService
	projectID string
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (resource.ExternalObservation, error) {
	cn, ok := mg.(*v1alpha1.Connection)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errNotConnection)
	}
	r, err := e.sn.Services.Connections.List(cn.Spec.Parent).Network(cn.Spec.Network).Context(ctx).Do()
	if err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errListConnections)
	}

	o := connection.Observation{Connection: findConnection(r.Connections)}
	if o.Connection == nil {
		return resource.ExternalObservation{ResourceExists: false}, nil
	}

	if o.Network, err = e.compute.Networks.Get(e.projectID, path.Base(o.Connection.Network)).Context(ctx).Do(); err != nil {
		return resource.ExternalObservation{}, errors.Wrap(err, errGetNetwork)
	}

	eo := resource.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: connection.UpToDate(cn.Spec.ConnectionParameters, o.Connection),
	}

	connection.UpdateStatus(&cn.Status, o)

	return eo, nil
}

func findConnection(conns []*servicenetworking.Connection) *servicenetworking.Connection {
	for _, c := range conns {
		if c.Peering == peeringName {
			return c
		}
	}
	return nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (resource.ExternalCreation, error) {
	cn, ok := mg.(*v1alpha1.Connection)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errNotConnection)
	}

	cn.Status.SetConditions(runtimev1alpha1.Creating())
	conn := connection.FromParameters(cn.Spec.ConnectionParameters)
	_, err := e.sn.Services.Connections.Create(cn.Spec.Parent, conn).Context(ctx).Do()
	return resource.ExternalCreation{}, errors.Wrap(resource.Ignore(gcp.IsErrorAlreadyExists, err), errCreateConnection)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (resource.ExternalUpdate, error) {
	cn, ok := mg.(*v1alpha1.Connection)
	if !ok {
		return resource.ExternalUpdate{}, errors.New(errNotConnection)
	}

	name := fmt.Sprintf("%s/connections/%s", cn.Spec.Parent, peeringName)
	conn := connection.FromParameters(cn.Spec.ConnectionParameters)
	_, err := e.sn.Services.Connections.Patch(name, conn).Context(ctx).Do()
	return resource.ExternalUpdate{}, errors.Wrap(err, errUpdateConnection)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cn, ok := mg.(*v1alpha1.Connection)
	if !ok {
		return errors.New(errNotConnection)
	}

	cn.Status.SetConditions(runtimev1alpha1.Deleting())
	rm := &compute.NetworksRemovePeeringRequest{Name: cn.Status.Peering}
	_, err := e.compute.Networks.RemovePeering(e.projectID, path.Base(cn.Spec.Network), rm).Context(ctx).Do()
	return errors.Wrap(resource.Ignore(gcp.IsErrorNotFound, err), errDeleteConnection)
}
