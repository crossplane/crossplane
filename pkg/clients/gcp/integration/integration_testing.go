package integration

import (
	"flag"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2/google"

	"github.com/crossplaneio/crossplane/pkg/clients/gcp"
)

var (
	// gcpCredsFile - retrieve gcp credentials from the file
	gcpCredsFile = flag.String("gcp-creds", "", "run integration tests that require crossplane-gcp-provider-key.json")
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

	creds, err := gcp.CredentialsFromFile(*gcpCredsFile, scopes...)
	g.Expect(err).NotTo(HaveOccurred())

	return g, creds
}
