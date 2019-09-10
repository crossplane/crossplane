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

package vpc

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
	errUnexpectedObject = "The managed resource is not an VPC resource"
	errClient           = "cannot create a new VPCClient"
	errDescribe         = "failed to describe VPC with id: %v"
	errMultipleItems    = "retrieved multiple VPCs for the given vpcId: %v"
	errCreate           = "failed to create the VPC resource"
	errDeleteNotPresent = "cannot delete the VPC, since the VPCID is not present"
	errDelete           = "failed to delete the VPC resource"
)

// Controller is the controller for VPC objects
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(v1alpha1.VPCGroupVersionKind),
		resource.WithExternalConnecter(&connector{client: mgr.GetClient(), newClientFn: ec2.NewVPCClient, awsConfigFn: utils.RetrieveAwsConfigFromProvider}),
		resource.WithManagedConnectionPublishers())
	name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.VPCKindAPIVersion, v1alpha1.Group))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.VPC{}).
		Complete(r)
}

type connector struct {
	client      client.Client
	newClientFn func(*aws.Config) (ec2.VPCClient, error)
	awsConfigFn func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error)
}

func (conn *connector) Connect(ctx context.Context, mgd resource.Managed) (resource.ExternalClient, error) {
	cr, ok := mgd.(*v1alpha1.VPC)
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
	client ec2.VPCClient
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (resource.ExternalObservation, error) {
	cr, ok := mgd.(*v1alpha1.VPC)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	// To find out whether a VPC exist:
	// - the object's ExternalState should have vpcId populated
	// - a VPC with the given vpcId should exist
	if cr.Status.VPCID == "" {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	req := e.client.DescribeVpcsRequest(&awsec2.DescribeVpcsInput{
		VpcIds: []string{cr.Status.VPCID},
	})
	req.SetContext(ctx)

	response, err := req.Send()

	if ec2.IsVPCNotFoundErr(err) {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return resource.ExternalObservation{}, errors.Wrapf(err, errDescribe, cr.Status.VPCID)
	}

	// in a successful response, there should be one and only one object
	if len(response.Vpcs) != 1 {
		return resource.ExternalObservation{}, errors.Errorf(errMultipleItems, cr.Status.VPCID)
	}

	observed := response.Vpcs[0]

	if observed.State == awsec2.VpcStateAvailable {
		cr.SetConditions(runtimev1alpha1.Available())
	}

	cr.UpdateExternalStatus(observed)

	return resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (resource.ExternalCreation, error) {
	cr, ok := mgd.(*v1alpha1.VPC)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(runtimev1alpha1.Creating())

	req := e.client.CreateVpcRequest(&awsec2.CreateVpcInput{
		CidrBlock: aws.String(cr.Spec.CIDRBlock),
	})
	req.SetContext(ctx)

	result, err := req.Send()
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	cr.UpdateExternalStatus(*result.Vpc)

	return resource.ExternalCreation{ConnectionDetails: resource.ConnectionDetails{}}, nil
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (resource.ExternalUpdate, error) {
	// TODO(soorena776): add more sophisticated Update logic, once we
	// categorize immutable vs mutable fields (see #727)

	return resource.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1alpha1.VPC)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	if cr.Status.VPCID == "" {
		return errors.New(errDeleteNotPresent)
	}

	cr.Status.SetConditions(runtimev1alpha1.Deleting())

	req := e.client.DeleteVpcRequest(&awsec2.DeleteVpcInput{
		VpcId: aws.String(cr.Status.VPCID),
	})
	req.SetContext(ctx)

	_, err := req.Send()

	if ec2.IsVPCNotFoundErr(err) {
		return nil
	}

	return errors.Wrap(err, errDelete)
}
