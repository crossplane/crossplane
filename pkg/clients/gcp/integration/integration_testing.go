package integration

import (
	"context"
	"flag"
	"io/ioutil"
	"testing"

	"github.com/onsi/gomega"
	"golang.org/x/oauth2/google"
)

var (
	// gcpCredsFile - retrieve gcp credentials from the file
	gcpCredsFile = flag.String("gcp-creds", "", "run integration tests that require crossplane-gcp-provider-key.json")
)

func init() {
	flag.Parse()
}

// CredsOrSkip - returns gcp configuration if environment is set, otherwise - skips this test
func CredsOrSkip(t *testing.T, scopes ...string) (*gomega.GomegaWithT, *google.Credentials) {
	if *gcpCredsFile == "" {
		t.Skip()
	}

	g := gomega.NewGomegaWithT(t)

	creds, err := credentialsFromFile(*gcpCredsFile, scopes...)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	return g, creds
}

func credentialsFromFile(file string, scopes ...string) (*google.Credentials, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return google.CredentialsFromJSON(context.Background(), data, scopes...)
}
