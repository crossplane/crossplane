package rds

import (
	"flag"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	. "github.com/onsi/gomega"
	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	awsclient "github.com/upbound/conductor/pkg/clients/aws"
	"k8s.io/apimachinery/pkg/util/rand"
)

var (
	// awsCredsFile - retrieve aws credentials from the file
	awsCredsFile = flag.String("aws-creds", "", "run integration tests that require .aws/credentials")

	// TestRDSInstanceName - name of the database instance for this test run
	TestRDSInstanceName = "test-" + rand.String(8)
)

func init() {
	flag.Parse()
}

// ConfigOrSkip - returns aws configuration if environment is set, otherwise - skips this test
func ConfigOrSkip(t *testing.T) (*GomegaWithT, *aws.Config) {
	if *awsCredsFile == "" {
		t.Skip()
	}

	g := NewGomegaWithT(t)

	config, err := awsclient.ConfigFromFile(*awsCredsFile, "us-east-1")
	g.Expect(err).NotTo(HaveOccurred())

	err = awsclient.ValidateConfig(config)
	g.Expect(err).NotTo(HaveOccurred())

	return g, config
}

func TestIntegrationCreateInstance(t *testing.T) {
	g, config := ConfigOrSkip(t)

	rds := NewClient(config)

	spec := &databasev1alpha1.RDSInstanceSpec{
		MasterUsername: "masteruser",
		Engine:         "mysql",
		Class:          "db.t2.small",
		Size:           int64(20),
	}

	password := "test-pass"

	instanceName := TestRDSInstanceName
	db, err := rds.CreateInstance(instanceName, password, spec)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(db).NotTo(BeNil())
	g.Expect(db.Name).To(Equal(instanceName))
	g.Expect(db.ARN).NotTo(And(BeNil(), BeEmpty()))
	g.Expect(db.Status).To(Equal(DBInstanceStatusCreating.String()))
}

func TestIntegrationGetInstance(t *testing.T) {
	g, config := ConfigOrSkip(t)

	rds := NewClient(config)

	instanceName := TestRDSInstanceName
	db, err := rds.GetInstance(instanceName)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(db).NotTo(BeNil())
	g.Expect(db.Name).To(Equal(instanceName))
	g.Expect(db.Status).To(Or(Equal(DBInstanceStatusCreating.String())))

}

func TestIntegrationDeleteInstance(t *testing.T) {
	g, config := ConfigOrSkip(t)
	rds := NewClient(config)

	instanceName := TestRDSInstanceName
	db, err := rds.DeleteInstance(instanceName)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(db).NotTo(BeNil())
	g.Expect(db.Name).To(Equal(instanceName))
	g.Expect(db.Status).To(Equal(DBInstanceStatusDeleting.String()))
}

func TestConditionType(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(ConditionType(DBInstanceStatusAvailable.String())).To(Equal(corev1alpha1.Running))
	g.Expect(ConditionType(DBInstanceStatusCreating.String())).To(Equal(corev1alpha1.Creating))
	g.Expect(ConditionType(DBInstanceStatusDeleting.String())).To(Equal(corev1alpha1.Deleting))
	g.Expect(ConditionType(DBInstanceStatusFailed.String())).To(Equal(corev1alpha1.Failed))
	g.Expect(ConditionType("foobar")).To(Equal(corev1alpha1.Pending))
}
