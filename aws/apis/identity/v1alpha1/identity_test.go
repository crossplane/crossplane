package v1alpha1

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/onsi/gomega"

	"github.com/crossplaneio/crossplane/pkg/clients/aws"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func Test_IAMRole_BuildExternalStatusFromObservation(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	r := IAMRole{}

	role := iam.Role{
		Arn:                      aws.String("arbitrary arn"),
		RoleId:                   aws.String("arbitrary id"),
		RoleName:                 aws.String("arbitrary name"),
		AssumeRolePolicyDocument: aws.String("arbitrary policy"),
	}

	r.UpdateExternalStatus(role)

	g.Expect(r.Status.IAMRoleExternalStatus.ARN).To(gomega.Equal(aws.StringValue(role.Arn)))
	g.Expect(r.Status.IAMRoleExternalStatus.RoleID).To(gomega.Equal(aws.StringValue(role.RoleId)))
}
