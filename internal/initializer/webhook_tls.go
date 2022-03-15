/*
Copyright 2022 The Crossplane Authors.

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

package initializer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

const (
	errGetWebhookSecret    = "cannot get webhook tls secret"
	errUpdateWebhookSecret = "cannot update webhook tls secret"
	errGenerateCertificate = "cannot generate tls certificate"
)

// CertificateGenerator can return you TLS certificate valid for given domains.
type CertificateGenerator interface {
	Generate(domain ...string) (key []byte, crt []byte, err error)
}

// WebhookCertificateGeneratorOption is used to configure WebhookCertificateGenerator behavior.
type WebhookCertificateGeneratorOption func(*WebhookCertificateGenerator)

// WithCertificateGenerator sets the CertificateGenerator that
// WebhookCertificateGenerator uses.
func WithCertificateGenerator(cg CertificateGenerator) WebhookCertificateGeneratorOption {
	return func(w *WebhookCertificateGenerator) {
		w.certificate = cg
	}
}

// NewWebhookCertificateGenerator returns a new WebhookCertificateGenerator.
func NewWebhookCertificateGenerator(nn types.NamespacedName, svcNamespace string, log logging.Logger, opts ...WebhookCertificateGeneratorOption) *WebhookCertificateGenerator {
	w := &WebhookCertificateGenerator{
		SecretRef:        nn,
		ServiceNamespace: svcNamespace,
		certificate:      NewRootCAGenerator(),
		log:              log,
	}
	for _, f := range opts {
		f(w)
	}
	return w
}

// WebhookCertificateGenerator is an initializer step that will find the given secret
// and fill its tls.crt and tls.key fields with a TLS certificate that is signed
// for *.<namespace>.svc domains so that all webhooks in that namespace can use
// it.
type WebhookCertificateGenerator struct {
	SecretRef        types.NamespacedName
	ServiceNamespace string

	certificate CertificateGenerator
	log         logging.Logger
}

// Run generates the TLS certificate valid for *.<namespace>.svc domain and
// updates the given secret.
func (wt *WebhookCertificateGenerator) Run(ctx context.Context, kube client.Client) error {
	s := &corev1.Secret{}
	if err := kube.Get(ctx, wt.SecretRef, s); err != nil {
		return errors.Wrap(err, errGetWebhookSecret)
	}
	// NOTE(muvaf): After 10 years, user will have to delete either of these
	// keys and re-create the pod to have the initializer re-generate the
	// certificate. No expiration check is put here.
	if len(s.Data["tls.key"]) != 0 && len(s.Data["tls.crt"]) != 0 {
		wt.log.Info("Given tls secret is already filled, skipping tls certificate generation")
		return nil
	}
	wt.log.Info("Given tls secret is empty, generating a new tls certificate")
	key, crt, err := wt.certificate.Generate(fmt.Sprintf("*.%s.svc", wt.ServiceNamespace))
	if err != nil {
		return errors.Wrap(err, errGenerateCertificate)
	}
	if s.Data == nil {
		s.Data = make(map[string][]byte, 2)
	}
	s.Data["tls.key"] = key
	s.Data["tls.crt"] = crt

	return errors.Wrap(kube.Update(ctx, s), errUpdateWebhookSecret)
}

// NewRootCAGenerator returns a new RootCAGenerator.
func NewRootCAGenerator() *RootCAGenerator {
	return &RootCAGenerator{}
}

// RootCAGenerator generates a root CA and key that can be used by client and
// servers.
type RootCAGenerator struct{}

// Generate creates TLS Secret with 10 years expiration date that is valid
// for the given domains.
func (*RootCAGenerator) Generate(domains ...string) (key []byte, crt []byte, err error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			CommonName:   "Crossplane",
			Organization: []string{"Crossplane"},
			Country:      []string{"Earth"},
			Province:     []string{"Earth"},
			Locality:     []string{"Earth"},
		},
		Issuer: pkix.Name{
			CommonName:   "Crossplane",
			Organization: []string{"Crossplane"},
			Country:      []string{"Earth"},
			Province:     []string{"Earth"},
			Locality:     []string{"Earth"},
		},
		DNSNames:              domains,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCRLSign | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	// NOTE(muvaf): Why 2048 and not 4096? Mainly performance.
	// See https://www.fastly.com/blog/key-size-for-tls
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot generate private key")
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create certificate with key")
	}

	caPEM := new(bytes.Buffer)
	if err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode cert into PEM")
	}
	caPrivateKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(caPrivateKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivateKey),
	}); err != nil {
		return nil, nil, errors.Wrap(err, "cannot encode private key into PEM")
	}
	return caPrivateKeyPEM.Bytes(), caPEM.Bytes(), nil
}
