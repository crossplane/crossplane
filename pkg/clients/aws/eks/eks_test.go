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

package eks

import (
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/onsi/gomega"
)

// MockAMIClient mocks AMI client which is used to get information about AMI images
type MockAMIClient struct {
	MockImages  []ec2.Image
	VerifyInput func(input *ec2.DescribeImagesInput)
}

// DescribeImagesRequest creates a DescribesImagesRequest
func (m *MockAMIClient) DescribeImagesRequest(input *ec2.DescribeImagesInput) ec2.DescribeImagesRequest {
	if m.VerifyInput != nil {
		m.VerifyInput(input)
	}

	return ec2.DescribeImagesRequest{
		Request: &aws.Request{
			HTTPRequest: &http.Request{},
			Data: &ec2.DescribeImagesOutput{
				Images: m.MockImages,
			},
		},
	}
}

var mockImages = []*ec2.Image{
	{
		CreationDate: aws.String("2019-08-13T11:38:33.006Z"),
		ImageId:      aws.String("img0"),
	},
	{
		CreationDate: aws.String("2019-08-14T11:38:33.001Z"),
		ImageId:      aws.String("img1"),
	},
	{
		CreationDate: aws.String("2019-08-14T11:38:33.000Z"),
		ImageId:      aws.String("img2"),
	},
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestGetMostRecent(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		expected string
	}{
		// img1 is the most recent image
		{"img1"},
	}

	for _, tt := range cases {
		actual := getMostRecentImage(mockImages)
		g.Expect(*actual.ImageId).To(gomega.Equal(tt.expected))
	}
}

func TestGetImageWithID(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	cases := []struct {
		imageID     string
		expectedImg *ec2.Image
		errorNil    bool
	}{
		{"img1", mockImages[1], true},
		{"img3", nil, false},
	}

	for _, tt := range cases {
		img, err := getImageWithID(tt.imageID, mockImages)
		g.Expect(img).To(gomega.Equal(tt.expectedImg))
		g.Expect(err == nil).To(gomega.Equal(tt.errorNil))
	}
}

func Test_GetAvailableImages_ValidVersion_ReturnsExpected(t *testing.T) {

	mockClusterVersion := "v1.13.7"
	expected := []*ec2.Image{{ImageId: aws.String("someami")}}
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{amiClient: &MockAMIClient{

		VerifyInput: func(input *ec2.DescribeImagesInput) {
			g.Expect(len(input.Filters)).To(gomega.Equal(2))
			for _, f := range input.Filters {
				switch *f.Name {
				case "name":
					g.Expect(f.Values[0]).To(gomega.Equal("*amazon-eks-node-1.13*"))
				case "state":
					g.Expect(f.Values[0]).To(gomega.Equal("available"))
				}
			}
		},
		MockImages: []ec2.Image{*expected[0]},
	}}
	res, err := mockEKSClient.getAvailableImages(mockClusterVersion)
	g.Expect(res).To(gomega.Equal(expected))
	g.Expect(err).Should(gomega.BeNil())
}

func Test_GetAvailableImages_InvalidVersion_ReturnsError(t *testing.T) {
	mockInvalidVersion := "1.a"
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{}

	res, err := mockEKSClient.getAvailableImages(mockInvalidVersion)

	g.Expect(res).Should(gomega.BeNil())
	g.Expect(err).ShouldNot(gomega.BeNil())
}

func Test_GetAMIImage_SpecificAMI_ReturnsExpected(t *testing.T) {
	mockClusterVersion := "v1.13.7"
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{amiClient: &MockAMIClient{
		MockImages: []ec2.Image{*mockImages[0], *mockImages[1], *mockImages[2]},
	}}

	// request specific ami
	res, err := mockEKSClient.getAMIImage("img0", mockClusterVersion)

	g.Expect(res).To(gomega.Equal(mockImages[0]))
	g.Expect(err).Should(gomega.BeNil())
}

func Test_GetAMIImage_NoAMIGiven_ReturnsMostRecent(t *testing.T) {
	mockClusterVersion := "v1.13.7"
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{amiClient: &MockAMIClient{
		MockImages: []ec2.Image{*mockImages[0], *mockImages[1], *mockImages[2]},
	}}

	// no specific ami is given (returns the most recent one)
	res, err := mockEKSClient.getAMIImage("", mockClusterVersion)

	g.Expect(res).To(gomega.Equal(mockImages[1])) //mockImages[1] is the most recent
	g.Expect(err).Should(gomega.BeNil())
}

func Test_GetAMIImage_NoAvailableAMI_ReturnsError(t *testing.T) {
	mockClusterVersion := "v1.13.7"
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{amiClient: &MockAMIClient{
		MockImages: []ec2.Image{},
	}}

	// no images for the given cluster, returns an error
	res, err := mockEKSClient.getAMIImage("", mockClusterVersion)

	g.Expect(res).Should(gomega.BeNil())
	g.Expect(err).ShouldNot(gomega.BeNil())
}

func Test_GetAMIImage_InvalidVersion_ReturnsError(t *testing.T) {
	mockInvalidVersion := "1.a"
	g := gomega.NewGomegaWithT(t)
	mockEKSClient := eksClient{}

	res, err := mockEKSClient.getAMIImage("whateverImagename", mockInvalidVersion)

	g.Expect(res).Should(gomega.BeNil())
	g.Expect(err).ShouldNot(gomega.BeNil())
}
