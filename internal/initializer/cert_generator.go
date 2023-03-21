package initializer

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errGenerateCertificate = "cannot generate tls certificate"
)

// CertificateSigner is the parent's certificate and key that will be used to sign the certificate
type CertificateSigner struct {
	certificate    *x509.Certificate
	key            *rsa.PrivateKey
	certificatePEM []byte
}

// CertificateGenerator can return you TLS certificate valid for given domains.
type CertificateGenerator interface {
	Generate(*x509.Certificate, *CertificateSigner) (key []byte, crt []byte, err error)
}

var (
	pkixName = pkix.Name{
		CommonName:   "Crossplane",
		Organization: []string{"Crossplane"},
		Country:      []string{"Earth"},
		Province:     []string{"Earth"},
		Locality:     []string{"Earth"},
	}
)

// NewCertGenerator returns a new CertGenerator.
func NewCertGenerator() *CertGenerator {
	return &CertGenerator{}
}

// CertGenerator generates a root CA and key that can be used by client and
// servers.
type CertGenerator struct{}

// Generate creates TLS Secret with 10 years expiration date that is valid
// for the given domains.
func (*CertGenerator) Generate(cert *x509.Certificate, signer *CertificateSigner) (key []byte, crt []byte, err error) {
	// NOTE(muvaf): Why 2048 and not 4096? Mainly performance.
	// See https://www.fastly.com/blog/key-size-for-tls
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot generate private key")
	}

	if signer == nil {
		signer = &CertificateSigner{
			certificate: cert,
			key:         privateKey,
		}
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, signer.certificate, &privateKey.PublicKey, signer.key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create certificate with key")
	}

	certPEM := new(bytes.Buffer)
	if err := pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode cert into PEM")
	}
	certKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(certKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode private key into PEM")
	}
	return certKeyPEM.Bytes(), certPEM.Bytes(), nil
}
