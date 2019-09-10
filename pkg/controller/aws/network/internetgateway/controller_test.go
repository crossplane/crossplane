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
	mockClient         fake.MockInternetGatewayClient

	// an arbitrary managed resource
	unexpecedItem resource.Managed
)

func TestMain(m *testing.M) {

	mockClient = fake.MockInternetGatewayClient{}
	mockExternalClient = external{&mockClient}

	os.Exit(m.Run())
}

func Test_Connect(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := &v1alpha1.InternetGateway{}
	var clientErr error
	var configErr error

	conn := connector{
		client: nil,
		newClientFn: func(conf *aws.Config) (ec2.InternetGatewayClient, error) {
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

	mockManaged := v1alpha1.InternetGateway{
		Status: v1alpha1.InternetGatewayStatus{
			InternetGatewayExternalStatus: v1alpha1.InternetGatewayExternalStatus{
				InternetGatewayID: "some arbitrary id",
			},
		},
	}

	mockExternal := &awsec2.InternetGateway{
		InternetGatewayId: aws.String("some arbitrary Id"),
	}
	var mockClientErr error
	var itemsList []awsec2.InternetGateway
	mockClient.MockDescribeInternetGatewaysRequest = func(input *awsec2.DescribeInternetGatewaysInput) awsec2.DescribeInternetGatewaysRequest {
		return awsec2.DescribeInternetGatewaysRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsec2.DescribeInternetGatewaysOutput{
					InternetGateways: itemsList,
				},
				Error: mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description               string
		managedObj                resource.Managed
		itemsReturned             []awsec2.InternetGateway
		clientErr                 error
		expectedErrNil            bool
		expectedResourceExist     bool
		expectedResoruceAbailable bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			[]awsec2.InternetGateway{*mockExternal},
			nil,
			true,
			true,
			true,
		},
		{
			"if resource is attaching, then the condition should not be avialable",
			mockManaged.DeepCopy(),
			[]awsec2.InternetGateway{
				{
					InternetGatewayId: aws.String("some arbitrary Id"),
					Attachments: []awsec2.InternetGatewayAttachment{
						{
							State: awsec2.AttachmentStatusAttaching,
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
			"if resource is detaching, then the condition should not be avialable",
			mockManaged.DeepCopy(),
			[]awsec2.InternetGateway{
				{
					InternetGatewayId: aws.String("some arbitrary Id"),
					Attachments: []awsec2.InternetGatewayAttachment{
						{
							State: awsec2.AttachmentStatusDetaching,
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
			&v1alpha1.InternetGateway{},
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
			awserr.New(ec2.InternetGatewayIDNotFound, "", nil),
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
			[]awsec2.InternetGateway{},
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

			mgd := tc.managedObj.(*v1alpha1.InternetGateway)

			if tc.expectedResoruceAbailable {
				g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
				g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionTrue), tc.description)
				g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonAvailable), tc.description)

			} else {
				g.Expect(len(mgd.Status.Conditions)).To(gomega.Equal(0), tc.description)
			}
			g.Expect(mgd.Status.InternetGatewayExternalStatus.InternetGatewayID).To(gomega.Equal(aws.StringValue(mockExternal.InternetGatewayId)), tc.description)
		}
	}
}

func Test_Create(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.InternetGateway{
		Spec: v1alpha1.InternetGatewaySpec{
			InternetGatewayParameters: v1alpha1.InternetGatewayParameters{
				VPCID: "arbitrary vpcId",
			},
		},
	}
	mockExternal := &awsec2.InternetGateway{
		InternetGatewayId: aws.String("some arbitrary arn"),
	}
	var mockClientErr error
	mockClient.MockCreateInternetGatewayRequest = func(input *awsec2.CreateInternetGatewayInput) awsec2.CreateInternetGatewayRequest {
		return awsec2.CreateInternetGatewayRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsec2.CreateInternetGatewayOutput{
					InternetGateway: mockExternal,
				},
				Error: mockClientErr,
			},
		}
	}

	var mockClientAttachErr error
	mockClient.MockAttachInternetGatewayRequest = func(input *awsec2.AttachInternetGatewayInput) awsec2.AttachInternetGatewayRequest {
		return awsec2.AttachInternetGatewayRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.AttachInternetGatewayOutput{},
				Error:       mockClientAttachErr,
			},
		}
	}

	for _, tc := range []struct {
		description     string
		managedObj      resource.Managed
		clientErr       error
		clientAttachErr error
		expectedErrNil  bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			nil,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			false,
		},
		{
			"if creating resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			nil,
			false,
		},
		{
			"if attaching IG fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr
		mockClientAttachErr = tc.clientAttachErr

		_, err := mockExternalClient.Create(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.InternetGateway)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonCreating), tc.description)
			g.Expect(mgd.Status.InternetGatewayExternalStatus.InternetGatewayID).To(gomega.Equal(aws.StringValue(mockExternal.InternetGatewayId)), tc.description)
		}
	}
}

func Test_Update(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.InternetGateway{}

	_, err := mockExternalClient.Update(context.Background(), &mockManaged)

	g.Expect(err).To(gomega.BeNil())
}

func Test_Delete(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.InternetGateway{
		Spec: v1alpha1.InternetGatewaySpec{
			InternetGatewayParameters: v1alpha1.InternetGatewayParameters{
				VPCID: "arbitrary vpcId",
			},
		},
		Status: v1alpha1.InternetGatewayStatus{
			InternetGatewayExternalStatus: v1alpha1.InternetGatewayExternalStatus{
				InternetGatewayID: "some arbitrary id",
				Attachments: []v1alpha1.InternetGatewayAttachment{
					{VPCID: "arbitrary vpcId"},
				},
			},
		},
	}
	var mockClientErr error
	mockClient.MockDeleteInternetGatewayRequest = func(input *awsec2.DeleteInternetGatewayInput) awsec2.DeleteInternetGatewayRequest {
		g.Expect(aws.StringValue(input.InternetGatewayId)).To(gomega.Equal(mockManaged.Status.InternetGatewayID), "the passed parameters are not valid")
		return awsec2.DeleteInternetGatewayRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.DeleteInternetGatewayOutput{},
				Error:       mockClientErr,
			},
		}
	}

	var mockClientDetachErr error
	mockClient.MockDetachInternetGatewayRequest = func(input *awsec2.DetachInternetGatewayInput) awsec2.DetachInternetGatewayRequest {
		g.Expect(aws.StringValue(input.InternetGatewayId)).To(gomega.Equal(mockManaged.Status.InternetGatewayID), "the passed parameters for DetachInternetGatewayRequest are not valid")
		g.Expect(aws.StringValue(input.VpcId)).To(gomega.Equal(mockManaged.Spec.VPCID), "the passed parameters for DetachInternetGatewayRequest are not valid")
		return awsec2.DetachInternetGatewayRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsec2.DetachInternetGatewayOutput{},
				Error:       mockClientDetachErr,
			},
		}
	}

	for _, tc := range []struct {
		description     string
		managedObj      resource.Managed
		clientErr       error
		clientDetachErr error
		expectedErrNil  bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			nil,
			true,
		},
		{
			"if status doesn't have the resource ID, it should return an error",
			&v1alpha1.InternetGateway{},
			nil,
			nil,
			false,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			nil,
			false,
		},
		{
			"if the resource doesn't exist deleting resource should not return an error",
			mockManaged.DeepCopy(),
			awserr.New(ec2.InternetGatewayIDNotFound, "", nil),
			nil,
			true,
		},
		{
			"if deleting resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			nil,
			false,
		},
		{
			"if detaching IG fails, it should return error",
			mockManaged.DeepCopy(),
			nil,
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr
		mockClientDetachErr = tc.clientDetachErr

		err := mockExternalClient.Delete(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.InternetGateway)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonDeleting), tc.description)
		}
	}
}
