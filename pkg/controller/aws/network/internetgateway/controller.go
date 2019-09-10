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

package internetgateway

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
	errUnexpectedObject = "The managed resource is not an InternetGateway resource"
	errClient           = "cannot create a new InternetGatewayClient"
	errDescribe         = "failed to describe InternetGateway with id: %v"
	errMultipleItems    = "retrieved multiple InternetGateways for the given internetGatewaysId: %v"
	errCreate           = "failed to create the InternetGateway resource"
	errDeleteNotPresent = "cannot delete the InternetGateway, since the internetGatewayID is not present"
	errDetach           = "failed to detach the InternetGateway %v from VPC %v"
	errDelete           = "failed to delete the InternetGateway resource"
)

// Controller is the controller for InternetGateway objects
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	r := resource.NewManagedReconciler(mgr,
		resource.ManagedKind(v1alpha1.InternetGatewayGroupVersionKind),
		resource.WithExternalConnecter(&connector{client: mgr.GetClient(), newClientFn: ec2.NewInternetGatewayClient, awsConfigFn: utils.RetrieveAwsConfigFromProvider}),
		resource.WithManagedConnectionPublishers())
	name := strings.ToLower(fmt.Sprintf("%s.%s", v1alpha1.InternetGatewayKindAPIVersion, v1alpha1.Group))
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.InternetGateway{}).
		Complete(r)
}

type connector struct {
	client      client.Client
	newClientFn func(*aws.Config) (ec2.InternetGatewayClient, error)
	awsConfigFn func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error)
}

func (conn *connector) Connect(ctx context.Context, mgd resource.Managed) (resource.ExternalClient, error) {
	cr, ok := mgd.(*v1alpha1.InternetGateway)
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
	client ec2.InternetGatewayClient
}

func (e *external) Observe(ctx context.Context, mgd resource.Managed) (resource.ExternalObservation, error) {
	cr, ok := mgd.(*v1alpha1.InternetGateway)
	if !ok {
		return resource.ExternalObservation{}, errors.New(errUnexpectedObject)
	}

	// To find out whether an InternetGateway exist:
	// - the object's ExternalState should have internetGatewayID populated
	// - an InternetGateway with the given internetGatewayID should exist
	if cr.Status.InternetGatewayID == "" {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	req := e.client.DescribeInternetGatewaysRequest(&awsec2.DescribeInternetGatewaysInput{
		InternetGatewayIds: []string{cr.Status.InternetGatewayID},
	})
	req.SetContext(ctx)

	response, err := req.Send()

	if ec2.IsInternetGatewayNotFoundErr(err) {
		return resource.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	if err != nil {
		return resource.ExternalObservation{}, errors.Wrapf(err, errDescribe, cr.Status.InternetGatewayID)
	}

	// in a successful response, there should be one and only one object
	if len(response.InternetGateways) != 1 {
		return resource.ExternalObservation{}, errors.Errorf(errMultipleItems, cr.Status.InternetGatewayID)
	}

	observed := response.InternetGateways[0]

	// if non of the attachments are currently in progress, then the IG is available
	isAvailable := true
	for _, a := range observed.Attachments {
		if a.State == awsec2.AttachmentStatusAttaching || a.State == awsec2.AttachmentStatusDetaching {
			isAvailable = false
			break
		}
	}

	if isAvailable {
		cr.SetConditions(runtimev1alpha1.Available())
	}

	cr.UpdateExternalStatus(observed)

	return resource.ExternalObservation{
		ResourceExists:    true,
		ConnectionDetails: resource.ConnectionDetails{},
	}, nil
}

func (e *external) Create(ctx context.Context, mgd resource.Managed) (resource.ExternalCreation, error) {
	cr, ok := mgd.(*v1alpha1.InternetGateway)
	if !ok {
		return resource.ExternalCreation{}, errors.New(errUnexpectedObject)
	}

	cr.Status.SetConditions(runtimev1alpha1.Creating())

	req := e.client.CreateInternetGatewayRequest(&awsec2.CreateInternetGatewayInput{})
	req.SetContext(ctx)

	ig, err := req.Send()
	if err != nil {
		return resource.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	cr.UpdateExternalStatus(*ig.InternetGateway)

	// after creating the IG, attach the VPC
	aReq := e.client.AttachInternetGatewayRequest(&awsec2.AttachInternetGatewayInput{
		InternetGatewayId: ig.InternetGateway.InternetGatewayId,
		VpcId:             aws.String(cr.Spec.VPCID),
	})
	aReq.SetContext(ctx)

	_, err = aReq.Send()

	return resource.ExternalCreation{}, errors.Wrap(err, errCreate)
}

func (e *external) Update(ctx context.Context, mgd resource.Managed) (resource.ExternalUpdate, error) {
	// TODO(soorena776): add more sophisticated Update logic, once we
	// categorize immutable vs mutable fields (see #727)

	return resource.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mgd resource.Managed) error {
	cr, ok := mgd.(*v1alpha1.InternetGateway)
	if !ok {
		return errors.New(errUnexpectedObject)
	}

	if cr.Status.InternetGatewayID == "" {
		return errors.New(errDeleteNotPresent)
	}

	cr.Status.SetConditions(runtimev1alpha1.Deleting())

	// first detach all vpc attachments
	for _, a := range cr.Status.Attachments {
		// after creating the IG, attach the VPC
		dReq := e.client.DetachInternetGatewayRequest(&awsec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(cr.Status.InternetGatewayID),
			VpcId:             aws.String(a.VPCID),
		})
		dReq.SetContext(ctx)

		if _, err := dReq.Send(); err != nil {
			if ec2.IsInternetGatewayNotFoundErr(err) {
				continue
			}
			return errors.Wrapf(err, errDetach, cr.Status.InternetGatewayID, a.VPCID)
		}
	}

	// now delete the IG
	req := e.client.DeleteInternetGatewayRequest(&awsec2.DeleteInternetGatewayInput{
		InternetGatewayId: aws.String(cr.Status.InternetGatewayID),
	})
	req.SetContext(ctx)

	_, err := req.Send()
	if ec2.IsInternetGatewayNotFoundErr(err) {
		return nil
	}
	return errors.Wrap(err, errDelete)
}
