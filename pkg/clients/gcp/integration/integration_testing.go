package integration

import (
	"flag"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/upbound/conductor/pkg/clients/gcp"
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

	creds, err := gcp.CredentialsFromFile(*gcpCredsFile, scopes...)
	g.Expect(err).NotTo(HaveOccurred())

	return g, creds
}
