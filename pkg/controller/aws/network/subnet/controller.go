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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	v1alpha1 "github.com/crossplaneio/crossplane/aws/apis/network/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/ec2"
	"github.com/crossplaneio/crossplane/pkg/controller/aws/utils"
)

const (
	errUnexpectedObject = "The managed resource is not an Subnet resource"
	errClient           = "cannot create a new SubnetClient"
	errDescribe         = "failed to describe Subnet with id: %v"
	errMultipleItems    = "retrieved multiple Subnet for the given subnetId: %v"
	errCreate           = "failed to create the Subnet resource"
	errDeleteNotPresent = "cannot delete the Subnet, since the SubnetId is not present"
	errDelete           = "failed to delete the Subnet resource"
)

// Controller is the controller for Subnet objects
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(v1alpha1.SubnetGroupVersionKind),
		resource.WithExternalConnecter(&connector{client: mgr.GetClient(), newClientFn: ec2.NewSubnetClient, awsConfigFn: utils.RetrieveAwsConfigFromProvider}),
		resource.WithManagedConnectionPublishers())
	name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.SubnetKindAPIVersion, v1alpha1.Group))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Subnet{}).
		Complete(r)
}

type connector struct {
	client      client.Client
	newClientFn func(*aws.Config) (ec2.SubnetClient, error)
	awsConfigFn func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error)
}

func (conn *connector) Connect(ctx context.Context, mgd resource.Managed) (resource.ExternalClient, error) {
	cr, ok := mgd.(*v1alpha1.Subnet)
	if !ok {
		return nil, errors.New(errUnexpectedObject)
	}

	awsconfig, err := conn.awsConfigFn(ctx, conn.client, cr.Spec.ProviderReference)
	if err != nil {
		return nil, err
	}

	c, err := conn.newClientFn(awsconfig)
	if err != nil {
		return nil, errors.Wrap(err, errClient)
	}

	return &external{c}, nil
}

type external struct {
	client ec2.SubnetClient
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (resource.ExternalObservation, error) {
	cr, ok := mgd.(*v1alpha1.Subnet)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	// To find out whether a Subnet exist:
	// - the object's ExternalState should have subnetId populated
	// - a Subnet with the given subnetId should exist
	if cr.Status.SubnetID == "" {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	req := e.client.DescribeSubnetsRequest(&awsec2.DescribeSubnetsInput{
		SubnetIds: []string{cr.Status.SubnetID},
	})
	req.SetContext(ctx)

	response, err := req.Send()

	if ec2.IsSubnetNotFoundErr(err) {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return resource.ExternalObservation{}, errors.Wrapf(err, errDescribe, cr.Status.SubnetID)
	}

	// in a successful response, there should be one and only one object
	if len(response.Subnets) != 1 {
		return resource.ExternalObservation{}, errors.Errorf(errMultipleItems, cr.Status.SubnetID)
	}

	observed := response.Subnets[0]

	if observed.State == awsec2.SubnetStateAvailable {
		cr.SetConditions(runtimev1alpha1.Available())
	} else if observed.State == awsec2.SubnetStatePending {
		cr.SetConditions(runtimev1alpha1.Creating())
	}

	cr.UpdateExternalStatus(observed)

	return resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (resource.ExternalCreation, error) {
	cr, ok := mgd.(*v1alpha1.Subnet)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(runtimev1alpha1.Creating())

	req := e.client.CreateSubnetRequest(&awsec2.CreateSubnetInput{
		VpcId:            aws.String(cr.Spec.VPCID),
		AvailabilityZone: aws.String(cr.Spec.AvailabilityZone),
		CidrBlock:        aws.String(cr.Spec.CIDRBlock),
	})
	req.SetContext(ctx)

	result, err := req.Send()
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	cr.UpdateExternalStatus(*result.Subnet)

	return resource.ExternalCreation{ConnectionDetails: resource.ConnectionDetails{}}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (resource.ExternalUpdate, error) {
	// TODO(soorena776): add more sophisticated Update logic, once we
	// categorize immutable vs mutable fields (see #727)

	return resource.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1alpha1.Subnet)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	if cr.Status.SubnetID == "" {
		return errors.New(errDeleteNotPresent)
	}

	cr.Status.SetConditions(runtimev1alpha1.Deleting())

	req := e.client.DeleteSubnetRequest(&awsec2.DeleteSubnetInput{
		SubnetId: aws.String(cr.Status.SubnetID),
	})
	req.SetContext(ctx)

	_, err := req.Send()

	if ec2.IsSubnetNotFoundErr(err) {
		return nil
	}

	return errors.Wrap(err, errDelete)
}
