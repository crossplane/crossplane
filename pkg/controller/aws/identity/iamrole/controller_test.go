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

package iamrole

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/resource"
	v1alpha1 "github.com/crossplaneio/crossplane/aws/apis/identity/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/iam"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/iam/fake"
)

var (
	mockExternalClient external
	mockClient         fake.MockRoleClient

	// an arbitrary managed resource
	unexpecedItem resource.Managed
)

func TestMain(m *testing.M) {

	mockClient = fake.MockRoleClient{}
	mockExternalClient = external{&mockClient}

	os.Exit(m.Run())
}

func Test_Connect(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := &v1alpha1.IAMRole{}
	var clientErr error
	var configErr error

	conn := connector{
		client: nil,
		newClientFn: func(conf *aws.Config) (iam.RoleClient, error) {
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
			mockManaged,
			errors.New("some error"),
			nil,
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

	mockManaged := v1alpha1.IAMRole{}
	mockExternal := &awsiam.Role{
		Arn: aws.String("some arbitrary arn"),
	}
	var mockClientErr error
	mockClient.MockGetRoleRequest = func(input *awsiam.GetRoleInput) awsiam.GetRoleRequest {
		return awsiam.GetRoleRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsiam.GetRoleOutput{
					Role: mockExternal,
				},
				Error: mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description           string
		managedObj            resource.Managed
		clientErr             error
		expectedErrNil        bool
		expectedResourceExist bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			true,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			false,
			false,
		},
		{
			"if external resource doesn't exist, it should return expected",
			mockManaged.DeepCopy(),
			awserr.New(awsiam.ErrCodeNoSuchEntityException, "", nil),
			true,
			false,
		},
		{
			"if external resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
			false,
		},
	} {
		mockClientErr = tc.clientErr

		result, err := mockExternalClient.Observe(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		g.Expect(result.ResourceExists).To(gomega.Equal(tc.expectedResourceExist), tc.description)
		if tc.expectedResourceExist {
			mgd := tc.managedObj.(*v1alpha1.IAMRole)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionTrue), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonAvailable), tc.description)
			g.Expect(mgd.Status.IAMRoleExternalStatus.ARN).To(gomega.Equal(aws.StringValue(mockExternal.Arn)), tc.description)
		}
	}
}

func Test_Create(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.IAMRole{
		Spec: v1alpha1.IAMRoleSpec{
			IAMRoleParameters: v1alpha1.IAMRoleParameters{
				AssumeRolePolicyDocument: "arbitrary role policy doc",
				Description:              "arbitrary role description",
				RoleName:                 "arbitrary role name",
			},
		},
	}
	mockExternal := &awsiam.Role{
		Arn: aws.String("some arbitrary arn"),
	}
	var mockClientErr error
	mockClient.MockCreateRoleRequest = func(input *awsiam.CreateRoleInput) awsiam.CreateRoleRequest {
		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.AssumeRolePolicyDocument)).To(gomega.Equal(mockManaged.Spec.AssumeRolePolicyDocument), "the passed parameters are not valid")
		g.Expect(aws.StringValue(input.Description)).To(gomega.Equal(mockManaged.Spec.Description), "the passed parameters are not valid")
		return awsiam.CreateRoleRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data: &awsiam.CreateRoleOutput{
					Role: mockExternal,
				},
				Error: mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description    string
		managedObj     resource.Managed
		clientErr      error
		expectedErrNil bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			false,
		},
		{
			"if creating resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr

		_, err := mockExternalClient.Create(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.IAMRole)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonCreating), tc.description)
			g.Expect(mgd.Status.IAMRoleExternalStatus.ARN).To(gomega.Equal(aws.StringValue(mockExternal.Arn)), tc.description)
		}
	}
}

func Test_Update(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.IAMRole{}

	_, err := mockExternalClient.Update(context.Background(), &mockManaged)

	g.Expect(err).To(gomega.BeNil())
}

func Test_Delete(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	mockManaged := v1alpha1.IAMRole{
		Spec: v1alpha1.IAMRoleSpec{
			IAMRoleParameters: v1alpha1.IAMRoleParameters{
				RoleName: "arbitrary role name",
			},
		},
	}
	var mockClientErr error
	mockClient.MockDeleteRoleRequest = func(input *awsiam.DeleteRoleInput) awsiam.DeleteRoleRequest {
		g.Expect(aws.StringValue(input.RoleName)).To(gomega.Equal(mockManaged.Spec.RoleName), "the passed parameters are not valid")
		return awsiam.DeleteRoleRequest{
			Request: &aws.Request{
				HTTPRequest: &http.Request{},
				Data:        &awsiam.DeleteRoleOutput{},
				Error:       mockClientErr,
			},
		}
	}

	for _, tc := range []struct {
		description    string
		managedObj     resource.Managed
		clientErr      error
		expectedErrNil bool
	}{
		{
			"valid input should return expected",
			mockManaged.DeepCopy(),
			nil,
			true,
		},
		{
			"unexpected managed resource should return error",
			unexpecedItem,
			nil,
			false,
		},
		{
			"if the resource doesn't exist deleting resource should not return an error",
			mockManaged.DeepCopy(),
			awserr.New(awsiam.ErrCodeNoSuchEntityException, "", nil),
			true,
		},
		{
			"if deleting resource fails, it should return error",
			mockManaged.DeepCopy(),
			errors.New("some error"),
			false,
		},
	} {
		mockClientErr = tc.clientErr

		err := mockExternalClient.Delete(context.Background(), tc.managedObj)

		g.Expect(err == nil).To(gomega.Equal(tc.expectedErrNil), tc.description)
		if tc.expectedErrNil {
			mgd := tc.managedObj.(*v1alpha1.IAMRole)
			g.Expect(mgd.Status.Conditions[0].Type).To(gomega.Equal(corev1alpha1.TypeReady), tc.description)
			g.Expect(mgd.Status.Conditions[0].Status).To(gomega.Equal(corev1.ConditionFalse), tc.description)
			g.Expect(mgd.Status.Conditions[0].Reason).To(gomega.Equal(corev1alpha1.ReasonDeleting), tc.description)
		}
	}
}
