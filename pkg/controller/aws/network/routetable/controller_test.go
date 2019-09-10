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

package routetable

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	v1alpha1 "github.com/crossplaneio/crossplane/aws/apis/network/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/ec2"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/ec2/fake"
)

var (
	mockExternalClient external
	mockClient         fake.MockRouteTableClient

	// an arbitrary managed resource
	unexpecedItem resource.Managed
)

func TestMain(m *testing.M) {

	mockClient = fake.MockRouteTableClient{}
	mockExternalClient = external{&mockClient}

	os.Exit(m.Run())
}

func Test_Connect(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := &v1alpha1.RouteTable{}
	var clientErr error
	var configErr error

	conn := connector{
		client: nil,
		newClientFn: func(conf *aws.Config) (ec2.RouteTableClient, error) {
			return &mockClient, clientErr
		},
		awsConfigFn: func(context.Context, client.Reader, *corev1.ObjectReference) (*aws.Config, error) {
			return &aws.Config{}, configErr
		},
	}

	for _, tc := range []struct {
		description       string
		managedObj        resource.Managed
		configErr         error
		clientErr         error
		expectedClientNil bool
		expectedErrNil    bool
	}{
		{
			"valid input should return expected",
			mockManaged,
			nil,
			nil,
			false,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			true,
			false,
		},
		{
			"if aws config provider fails, should return error",
			mockManaged, // an arbitrary managed resource which is not expected
			errors.New("some error"),
			nil,
			true,
			false,
		},
		{
			"if aws client provider fails, should return error",
			mockManaged, // an arbitrary managed resource which is not expected
			nil,
			errors.New("some error"),
			true,
			false,
		},
	} {
		clientErr = tc.clientErr
		configErr = tc.configErr

		res, err := conn.Connect(context.Background(), tc.managedObj)
		g.Expect(res == nil).To(gomega.Equal(tc.expectedClientNil), tc.description)
		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
	}
}

func Test_Observe(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.RouteTable{
		Status: v1alpha1.RouteTableStatus{
			RouteTableExternalStatus: v1alpha1.RouteTableExternalStatus{
				RouteTableID: "some arbitrary id",
			},
		},
	}

	mockExternal := &awsec2.RouteTable{
		RouteTableId: aws.String("some arbitrary Id"),
	}
	var mockClientErr error
	var itemsList []awsec2.RouteTable
	mockClient.MockDescribeRouteTablesRequest = func(input *awsec2.DescribeRouteTablesInput) awsec2.DescribeRouteTablesRequest {
		return awsec2.DescribeRouteTablesRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsec2.DescribeRouteTablesOutput{
					RouteTables: itemsList,
				},
				Error: mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description               string
		managedObj                resource.Managed
		itemsReturned             []awsec2.RouteTable
		clientErr                 error
		expectedErrNil            bool
		expectedResourceExist     bool
		expectedResoruceAbailable bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			[]awsec2.RouteTable{*mockExternal},
			nil,
			true,
			true,
			true,
		},
		{
			"if any route is not active, then the resource should not be avialable",
			mockManaged.DeepCopy(),
			[]awsec2.RouteTable{
				{
					RouteTableId: aws.String("some arbitrary Id"),
					Routes: []awsec2.Route{
						{
							State: awsec2.RouteStateBlackhole,
						},
					},
				},
			},
			nil,
			true,
			true,
			false,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			false,
			false,
			false,
		},
		{
			"if item's identifier is not yet set, returns expected",
			&v1alpha1.RouteTable{},
			nil,
			nil,
			true,
			false,
			false,
		},
		{
			"if external resource doesn't exist, it should return expected",
			mockManaged.DeepCopy(),
			nil,
			awserr.New(ec2.RouteTableIDNotFound, "", nil),
			true,
			false,
			false,
		},
		{
			"if external resource fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			errors.New("some error"),
			false,
			false,
			false,
		},
		{
			"if external resource returns a list with other than one item, it should return error",
			mockManaged.DeepCopy(),
			[]awsec2.RouteTable{},
			nil,
			false,
			false,
			false,
		},
	} {
		mockClientErr = tc.clientErr
		itemsList = tc.itemsReturned

		result, err := mockExternalClient.Observe(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		g.Expect(result.ResourceExists).To(gomega.Equal(tc.expectedResourceExist), tc.description)
		if tc.expectedResourceExist {

			mgd := tc.managedObj.(*v1alpha1.RouteTable)

			if tc.expectedResoruceAbailable {
				g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
				g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionTrue), tc.description)
				g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonAvailable), tc.description)

			} else {
				g.Expect(len(mgd.Status.Conditions)).To(gomega.Equal(0), tc.description)
			}
			g.Expect(mgd.Status.RouteTableExternalStatus.RouteTableID).To(gomega.Equal(aws.StringValue(mockExternal.RouteTableId)), tc.description)
		}
	}
}

func Test_Create(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.RouteTable{
		Spec: v1alpha1.RouteTableSpec{
			RouteTableParameters: v1alpha1.RouteTableParameters{
				VPCID: "arbitrary vpcId",
				Routes: []v1alpha1.Route{
					{
						DestinationCIDRBlock: "arbitrary dcb 0",
						GatewayID:            "arbitrary gi 0",
					},
				},
				Associations: []v1alpha1.Association{
					{
						SubnetID: "arbitrary subnet 0",
					},
				},
			},
		},
	}
	mockExternal := &awsec2.RouteTable{
		RouteTableId: aws.String("some arbitrary arn"),
	}
	var externalObj *awsec2.RouteTable
	var mockClientErr error
	mockClient.MockCreateRouteTableRequest = func(input *awsec2.CreateRouteTableInput) awsec2.CreateRouteTableRequest {
		g.Expect(aws.StringValue(input.VpcId)).To(gomega.Equal(mockManaged.Spec.VPCID), "the passed parameters are not valid")
		return awsec2.CreateRouteTableRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsec2.CreateRouteTableOutput{
					RouteTable: externalObj,
				},
				Error: mockClientErr,
			},
		}
	}

	var mockClientCreateRouteErr error
	var createRouteCalled bool
	mockClient.MockCreateRouteRequest = func(input *awsec2.CreateRouteInput) awsec2.CreateRouteRequest {
		createRouteCalled = true
		g.Expect(aws.StringValue(input.RouteTableId)).To(gomega.Equal(aws.StringValue(mockExternal.RouteTableId)), "the passed parameters are not valid")
		return awsec2.CreateRouteRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.CreateRouteOutput{},
				Error:       mockClientCreateRouteErr,
			},
		}
	}

	var mockClientAssociateRouteErr error
	var associateCalled bool
	mockClient.MockAssociateRouteTableRequest = func(input *awsec2.AssociateRouteTableInput) awsec2.AssociateRouteTableRequest {
		associateCalled = true
		g.Expect(aws.StringValue(input.RouteTableId)).To(gomega.Equal(aws.StringValue(mockExternal.RouteTableId)), "the passed parameters are not valid")
		return awsec2.AssociateRouteTableRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.AssociateRouteTableOutput{},
				Error:       mockClientAssociateRouteErr,
			},
		}
	}

	for _, tc := range []struct {
		description             string
		managedObj              resource.Managed
		extObj                  *awsec2.RouteTable
		clientErr               error
		clientCreateRouteErr    error
		clientAssociateErr      error
		expectedErrNil          bool
		expectedCreateRouteCall bool
		expectedAssociateCall   bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			mockExternal,
			nil,
			nil,
			nil,
			true,
			true,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			mockExternal,
			nil,
			nil,
			nil,
			false,
			false,
			false,
		},
		{
			"if creating resource fails, it should return error",
			mockManaged.DeepCopy(),
			mockExternal,
			errors.New("some error"),
			nil,
			nil,
			false,
			false,
			false,
		},
		{
			"if creating a route fails, it should return error",
			mockManaged.DeepCopy(),
			mockExternal,
			nil,
			errors.New("some error"),
			nil,
			false,
			true,
			false,
		},
		{
			"if a route is already created, it should return expected",
			mockManaged.DeepCopy(),
			&awsec2.RouteTable{
				RouteTableId: aws.String("some arbitrary arn"),
				Routes: []awsec2.Route{
					{
						DestinationCidrBlock: aws.String("arbitrary dcb 0"),
						GatewayId:            aws.String("arbitrary gi 0"),
					},
				},
			},
			nil,
			nil,
			nil,
			true,
			false,
			true,
		},
		{
			"if associating a subnet fails, it should return error",
			mockManaged.DeepCopy(),
			mockExternal,
			nil,
			nil,
			errors.New("some error"),
			false,
			true,
			true,
		},
		{
			"if a subnet is already associated, it should return expected",
			mockManaged.DeepCopy(),
			&awsec2.RouteTable{
				RouteTableId: aws.String("some arbitrary arn"),
				Associations: []awsec2.RouteTableAssociation{
					{
						SubnetId: aws.String("arbitrary subnet 0"),
					},
				},
			},
			nil,
			nil,
			nil,
			true,
			true,
			false,
		},
	} {
		associateCalled = false
		createRouteCalled = false
		mockClientErr = tc.clientErr
		mockClientCreateRouteErr = tc.clientCreateRouteErr
		mockClientAssociateRouteErr = tc.clientAssociateErr

		externalObj = tc.extObj

		_, err := mockExternalClient.Create(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.RouteTable)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonCreating), tc.description)
			g.Expect(mgd.Status.RouteTableExternalStatus.RouteTableID).To(gomega.Equal(aws.StringValue(mockExternal.RouteTableId)), tc.description)
		}

		g.Expect(associateCalled).To(gomega.Equal(tc.expectedAssociateCall), tc.description)
		g.Expect(createRouteCalled).To(gomega.Equal(tc.expectedCreateRouteCall), tc.description)
	}
}

func Test_Update(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.RouteTable{}

	_, err := mockExternalClient.Update(context.Background(), &mockManaged)

	g.Expect(err).To(gomega.BeNil())
}

func Test_Delete(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.RouteTable{
		Status: v1alpha1.RouteTableStatus{
			RouteTableExternalStatus: v1alpha1.RouteTableExternalStatus{
				RouteTableID: "an arbitrary id",
				Routes: []v1alpha1.RouteState{
					{
						Route: v1alpha1.Route{
							DestinationCIDRBlock: "arbitrary dcb 0",
							GatewayID:            "arbitrary gatewayid 0",
						},
					},
				},
				Associations: []v1alpha1.AssociationState{
					{
						AssociationID: "arbitrary association id 0",
					},
				},
			},
		},
	}
	var mockClientErr error
	mockClient.MockDeleteRouteTableRequest = func(input *awsec2.DeleteRouteTableInput) awsec2.DeleteRouteTableRequest {
		g.Expect(aws.StringValue(input.RouteTableId)).To(gomega.Equal(mockManaged.Status.RouteTableID), "the passed parameters are not valid")
		return awsec2.DeleteRouteTableRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.DeleteRouteTableOutput{},
				Error:       mockClientErr,
			},
		}
	}

	var mockClientDeleteRouteErr error
	var deleteRouteCalled bool
	mockClient.MockDeleteRouteRequest = func(input *awsec2.DeleteRouteInput) awsec2.DeleteRouteRequest {
		deleteRouteCalled = true
		g.Expect(aws.StringValue(input.RouteTableId)).To(gomega.Equal(mockManaged.Status.RouteTableID), "the passed parameters for DeleteRouteRequest are not valid")
		return awsec2.DeleteRouteRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.DeleteRouteOutput{},
				Error:       mockClientDeleteRouteErr,
			},
		}
	}

	var mockClientDisassociateErr error
	var disassociateCalled bool
	mockClient.MockDisassociateRouteTableRequest = func(input *awsec2.DisassociateRouteTableInput) awsec2.DisassociateRouteTableRequest {
		disassociateCalled = true
		return awsec2.DisassociateRouteTableRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.DisassociateRouteTableOutput{},
				Error:       mockClientDisassociateErr,
			},
		}
	}

	testCases := []struct {
		description              string
		managedObj               resource.Managed
		clientErr                error
		clientDeleteRouteErr     error
		clientDissassociateErr   error
		expectedErrNil           bool
		expectedDeleteRouteCall  bool
		expectedDisassociateCall bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			nil,
			nil,
			true,
			true,
			true,
		},
		{
			"if status doesn't have the resource ID, it should return an error",
			&v1alpha1.RouteTable{},
			nil,
			nil,
			nil,
			false,
			false,
			false,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			nil,
			false,
			false,
			false,
		},

		{
			"if deleting resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			nil,
			nil,
			false,
			true,
			true,
		},
		{
			"if the resource doesn't exist deleting resource should not return an error",
			mockManaged.DeepCopy(),
			awserr.New(ec2.RouteTableIDNotFound, "", nil),
			nil,
			nil,
			true,
			true,
			true,
		},
		{
			"if deleting a route fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			errors.New("some error"),
			nil,
			false,
			true,
			false,
		},
		{
			"if a route is local, it should not be deleted",
			&v1alpha1.RouteTable{
				Status: v1alpha1.RouteTableStatus{
					RouteTableExternalStatus: v1alpha1.RouteTableExternalStatus{
						RouteTableID: "an arbitrary id",
						Routes: []v1alpha1.RouteState{
							{Route: v1alpha1.Route{
								DestinationCIDRBlock: "arbitrary dcb 0",
								GatewayID:            ec2.LocalGatewayID,
							}},
						},
					},
				},
			},
			nil,
			errors.New("some error"),
			nil,
			true,
			false,
			false,
		},
		{
			"if disassociating a subnet fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			nil,
			errors.New("some error"),
			false,
			true,
			true,
		},
	}

	for _, tc := range testCases {
		deleteRouteCalled = false
		disassociateCalled = false
		mockClientErr = tc.clientErr
		mockClientDeleteRouteErr = tc.clientDeleteRouteErr
		mockClientDisassociateErr = tc.clientDissassociateErr

		err := mockExternalClient.Delete(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.RouteTable)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonDeleting), tc.description)
		}

		g.Expect(deleteRouteCalled).To(gomega.Equal(tc.expectedDeleteRouteCall), tc.description)
		g.Expect(disassociateCalled).To(gomega.Equal(tc.expectedDisassociateCall), tc.description)
	}
}
