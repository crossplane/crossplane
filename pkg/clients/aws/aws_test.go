package aws

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

const (
	awsCredentialsFileFormat = "[%s]\naws_access_key_id = %s\naws_secret_access_key = %s"
)

func TestCredentialsIdSecret(t *testing.T) {
	g := NewGomegaWithT(t)

	testProfile := "default"
	testID := "testID"
	testSecret := "testSecret"
	credentials := []byte(fmt.Sprintf(awsCredentialsFileFormat, testProfile, testID, testSecret))

	// valid profile
	id, secret, err := CredentialsIDSecret([]byte(credentials), testProfile)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal(testID))
	g.Expect(secret).To(Equal(testSecret))

	// invalid profile - foo does not exist
	id, secret, err = CredentialsIDSecret([]byte(credentials), "foo")
	g.Expect(err).To(HaveOccurred())
	g.Expect(id).To(Equal(""))
	g.Expect(secret).To(Equal(""))
}

func TestConfig(t *testing.T) {
	g := NewGomegaWithT(t)

	testProfile := "default"
	testID := "testID"
	testSecret := "testSecret"
	testRegion := "us-west-2"
	credentials := []byte(fmt.Sprintf(awsCredentialsFileFormat, testProfile, testID, testSecret))

	config, err := Config([]byte(credentials), testProfile, testRegion)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config).NotTo(BeNil())
}

// TestValidate - reads AWS configuration from the local file.
// The file path is provided via TEST_AWS_CREDENTIALS_FILE environment variable, otherwise the test is skipped.
func TestValidate(t *testing.T) {
	g := NewGomegaWithT(t)

	awsCredsFile := os.Getenv("TEST_AWS_CREDENTIALS_FILE")
	if awsCredsFile == "" {
		t.Log("not found: TEST_AWS_CREDENTIALS_FILE")
		t.Skip()
	}
	t.Logf("using: %s", awsCredsFile)

	data, err := ioutil.ReadFile(awsCredsFile)
	g.Expect(err).NotTo(HaveOccurred())

	config, err := Config(data, "default", "us-west-2")
	g.Expect(err).NotTo(HaveOccurred())

	err = ValidateConfig(config)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestValidateInvalid(t *testing.T) {
	g := NewGomegaWithT(t)

	data := []byte(fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s", "default", "foo", "barr"))

	config, err := Config(data, "default", "us-west-2")
	g.Expect(err).NotTo(HaveOccurred())

	err = ValidateConfig(config)
	g.Expect(err).To(HaveOccurred())
}
