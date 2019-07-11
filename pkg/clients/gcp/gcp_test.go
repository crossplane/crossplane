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

package gcp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2/google"
)

func TestCredentialsFromFile(t *testing.T) {
	g := NewGomegaWithT(t)

	projectID := "test-project-123456"
	privateKeyID := "testkeyid"
	privateKeyValue := "testkeyvalue"
	clientEmail := "test-service@test-project-123456.iam.gserviceaccount.com"
	clientID := "123456789012345678901"
	clientCertURL := "https://www.googleapis.com/robot/v1/metadata/x509/test-service%40test-project-123456.iam.gserviceaccount.com"

	content := `{
	"type": "service_account",
	"project_id": "%s",
	"private_key_id": "%s",
	"private_key": "-----BEGIN PRIVATE KEY-----\n%s\n-----END PRIVATE KEY-----\n",
	"client_email": "%s",
	"client_id": "%s",
	"auth_uri": "https://accounts.google.com/o/oauth2/auth",
	"token_uri": "https://oauth2.googleapis.com/token",
	"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
	"client_x509_cert_url": "%s"}`

	tmpfile, err := ioutil.TempFile("", "test")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write([]byte(fmt.Sprintf(content, projectID, privateKeyID, privateKeyValue, clientEmail, clientID, clientCertURL))); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	testScope := "https://www.googleapis.com/auth/test-scope"

	creds, err := credentialsFromFile(tmpfile.Name(), []string{testScope}...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(creds).NotTo(BeNil())
	g.Expect(creds.ProjectID).To(Equal(projectID))
}

func TestCredentialsFromFileError(t *testing.T) {
	g := NewGomegaWithT(t)

	testScope := "https://www.googleapis.com/auth/test-scope"

	creds, err := credentialsFromFile("file", []string{testScope}...)
	g.Expect(err).To(HaveOccurred())
	g.Expect(creds).To(BeNil())
}

func TestMissingPermissions(t *testing.T) {
	g := NewGomegaWithT(t)

	g.Expect(getMissingPermissions([]string{}, []string{})).To(BeNil())
	g.Expect(getMissingPermissions([]string{"a"}, []string{})).To(Equal([]string{"a"}))
	g.Expect(getMissingPermissions([]string{"a", "a"}, []string{})).To(Equal([]string{"a", "a"}))
	g.Expect(getMissingPermissions([]string{"a", "a"}, []string{"a"})).To(BeNil())
	g.Expect(getMissingPermissions([]string{"a", "b"}, []string{"a"})).To(Equal([]string{"b"}))
}

func credentialsFromFile(file string, scopes ...string) (*google.Credentials, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return google.CredentialsFromJSON(context.Background(), data, scopes...)
}
