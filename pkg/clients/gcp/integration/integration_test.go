/*
Copyright 2018 The Conductor Authors.

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

// package integration contains unit tests that run against with actual GCP credentials and against the actual
// GCP project
package integration

import (
	"flag"
	"testing"

	. "github.com/onsi/gomega"
	. "github.com/upbound/conductor/pkg/clients/gcp"
	"golang.org/x/oauth2/google"
)

var (
	// gcpCredsFile - retrieve gcp credentials from the file
	gcpCredsFile = flag.String("gcp-creds", "", "run integration tests that require key.json")
)

func init() {
	flag.Parse()
}

// CredsOrSkip - returns gcp configuration if environment is set, otherwise - skips this test
func CredsOrSkip(t *testing.T, scopes ...string) (*GomegaWithT, *google.Credentials) {
	if *gcpCredsFile == "" {
		t.Skip()
	}

	g := NewGomegaWithT(t)

	creds, err := CredentialsFromFile(*gcpCredsFile, scopes...)
	g.Expect(err).NotTo(HaveOccurred())

	return g, creds
}

func TestProject(t *testing.T) {
	g, creds := CredsOrSkip(t, DefaultScope)
	p, err := Project(creds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(p.ID).To(Equal(creds.ProjectID))
}

func TestTestPermissions(t *testing.T) {
	g, creds := CredsOrSkip(t, DefaultScope)
	g.Expect(TestPermissions(creds, []string{})).NotTo(HaveOccurred())
	g.Expect(TestPermissions(creds, []string{"foo.enterprises.manage"})).To(HaveOccurred())
	// comment out and update assertions below
	//g.Expect(TestPermissions(creds, []string{"cloudsql.instances.list"})).NotTo(HaveOccurred())
	//g.Expect(TestPermissions(creds, []string{"compute.instances.list"})).NotTo(HaveOccurred())
}
