/*
Copyright 2018 The Crossplane Authors.

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

package aws

import (
	"fmt"
	"testing"

	"github.com/go-ini/ini"

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
	id, secret, err := CredentialsIDSecret(credentials, testProfile)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal(testID))
	g.Expect(secret).To(Equal(testSecret))

	// invalid profile - foo does not exist
	id, secret, err = CredentialsIDSecret(credentials, "foo")
	g.Expect(err).To(HaveOccurred())
	g.Expect(id).To(Equal(""))
	g.Expect(secret).To(Equal(""))
}

func TestLoadConfig(t *testing.T) {
	g := NewGomegaWithT(t)

	testProfile := "default"
	testID := "testID"
	testSecret := "testSecret"
	testRegion := "us-west-2"
	credentials := []byte(fmt.Sprintf(awsCredentialsFileFormat, testProfile, testID, testSecret))

	config, err := LoadConfig(credentials, testProfile, testRegion)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config).NotTo(BeNil())
}

func TestValidateInvalid(t *testing.T) {
	g := NewGomegaWithT(t)

	data := []byte(fmt.Sprintf("[%s]\naws_access_key_id = %s\naws_secret_access_key = %s", "default", "foo", "barr"))

	config, err := LoadConfig(data, ini.DEFAULT_SECTION, "us-west-2")
	g.Expect(err).NotTo(HaveOccurred())

	err = ValidateConfig(config)
	g.Expect(err).To(HaveOccurred())
}
