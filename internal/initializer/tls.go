/*
Copyright 2023 The Crossplane Authors.

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
	"context"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errGenerateCA              = "cannot generate CA certificate"
	errParseCACertificate      = "cannot parse CA certificate"
	errParseCAKey              = "cannot parse CA key"
	errLoadOrGenerateSigner    = "cannot load or generate certificate signer"
	errDecodeKey               = "cannot decode key"
	errDecodeCert              = "cannot decode cert"
	errFmtGetTLSSecret         = "cannot get TLS secret: %s"
	errFmtCannotCreateOrUpdate = "cannot create or update secret: %s"
)

const (
	// RootCACertSecretName is the name of the secret that will store CA certificates and rest of the
	// certificates created per entities will be signed by this CA
	RootCACertSecretName = "crossplane-root-ca"

	// SecretKeyCACert is the secret key of CA certificate
	SecretKeyCACert = "ca.crt"
)

// TLSCertificateGenerator is an initializer step that will find the given secret
// and fill its tls.crt, tls.key and ca.crt fields to be used for External Secret
// Store plugins
type TLSCertificateGenerator struct {
	namespace           string
	caSecretName        string
	tlsServerSecretName *string
	tlsClientSecretName *string
	subject             string
	owner               []metav1.OwnerReference
	certificate         CertificateGenerator
	log                 logging.Logger
}

// TLSCertificateGeneratorOption is used to configure TLSCertificateGenerator behavior.
type TLSCertificateGeneratorOption func(*TLSCertificateGenerator)

// TLSCertificateGeneratorWithLogger returns an TLSCertificateGeneratorOption that configures logger
func TLSCertificateGeneratorWithLogger(log logging.Logger) TLSCertificateGeneratorOption {
	return func(g *TLSCertificateGenerator) {
		g.log = log
	}
}

// TLSCertificateGeneratorWithOwner returns an TLSCertificateGeneratorOption that sets owner reference
func TLSCertificateGeneratorWithOwner(owner []metav1.OwnerReference) TLSCertificateGeneratorOption {
	return func(g *TLSCertificateGenerator) {
		g.owner = owner
	}
}

// TLSCertificateGeneratorWithServerSecretName returns an TLSCertificateGeneratorOption that sets server secret name.
func TLSCertificateGeneratorWithServerSecretName(s *string) TLSCertificateGeneratorOption {
	return func(g *TLSCertificateGenerator) {
		g.tlsServerSecretName = s
	}
}

// TLSCertificateGeneratorWithClientSecretName returns an TLSCertificateGeneratorOption that sets client secret name.
func TLSCertificateGeneratorWithClientSecretName(s *string) TLSCertificateGeneratorOption {
	return func(g *TLSCertificateGenerator) {
		g.tlsClientSecretName = s
	}
}

// NewTLSCertificateGenerator returns a new TLSCertificateGenerator.
func NewTLSCertificateGenerator(ns, caSecret, subject string, opts ...TLSCertificateGeneratorOption) *TLSCertificateGenerator {
	e := &TLSCertificateGenerator{
		namespace:    ns,
		caSecretName: caSecret,
		subject:      subject,
		certificate:  NewCertGenerator(),
		log:          logging.NewNopLogger(),
	}

	for _, f := range opts {
		f(e)
	}
	return e
}

func (e *TLSCertificateGenerator) loadOrGenerateCA(ctx context.Context, kube client.Client, nn types.NamespacedName) (*CertificateSigner, error) {
	caSecret := &corev1.Secret{}

	err := kube.Get(ctx, nn, caSecret)
	if resource.IgnoreNotFound(err) != nil {
		return nil, errors.Wrapf(err, errFmtGetTLSSecret, nn.Name)
	}

	if err == nil {
		kd := caSecret.Data[corev1.TLSPrivateKeyKey]
		cd := caSecret.Data[corev1.TLSCertKey]
		if len(kd) != 0 && len(cd) != 0 {
			e.log.Info("TLS CA secret is complete.")
			return parseCertificateSigner(kd, cd)
		}
	}
	e.log.Info("TLS CA secret is empty or not complete, generating a new CA...")

	a := &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkixName,
		Issuer:                pkixName,
		DNSNames:              []string{"crossplane-root-ca"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCRLSign | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caKeyByte, caCrtByte, err := e.certificate.Generate(a, nil)
	if err != nil {
		return nil, errors.Wrap(err, errGenerateCA)
	}

	caSecret.Name = nn.Name
	caSecret.Namespace = nn.Namespace
	_, err = controllerruntime.CreateOrUpdate(ctx, kube, caSecret, func() error {
		caSecret.Data = map[string][]byte{
			corev1.TLSCertKey:       caCrtByte,
			corev1.TLSPrivateKeyKey: caKeyByte,
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, errFmtCannotCreateOrUpdate, nn.Name)
	}

	return parseCertificateSigner(caKeyByte, caCrtByte)
}

func (e *TLSCertificateGenerator) ensureClientCertificate(ctx context.Context, kube client.Client, nn types.NamespacedName, signer *CertificateSigner) error {
	sec := &corev1.Secret{}

	err := kube.Get(ctx, nn, sec)
	if resource.IgnoreNotFound(err) != nil {
		return errors.Wrapf(err, errFmtGetTLSSecret, nn.Name)
	}

	if err == nil {
		if len(sec.Data[corev1.TLSPrivateKeyKey]) != 0 || len(sec.Data[corev1.TLSCertKey]) != 0 || len(sec.Data[SecretKeyCACert]) != 0 {
			e.log.Info("TLS secret contains client certificate.", "secret", nn.Name)
			return nil
		}
	}
	e.log.Info("Client certificates are empty nor not complete, generating a new pair...", "secret", nn.Name)

	cert := &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkixName,
		DNSNames:              []string{e.subject},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	keyData, certData, err := e.certificate.Generate(cert, signer)
	if err != nil {
		return errors.Wrap(err, errGenerateCertificate)
	}

	sec.Name = nn.Name
	sec.Namespace = nn.Namespace
	if e.owner != nil {
		sec.OwnerReferences = e.owner
	}
	_, err = controllerruntime.CreateOrUpdate(ctx, kube, sec, func() error {
		if sec.Data == nil {
			sec.Data = make(map[string][]byte)
		}
		sec.Data[corev1.TLSCertKey] = certData
		sec.Data[corev1.TLSPrivateKeyKey] = keyData
		sec.Data[SecretKeyCACert] = signer.certificatePEM

		return nil
	})

	return errors.Wrapf(err, errFmtCannotCreateOrUpdate, nn.Name)
}

func (e *TLSCertificateGenerator) ensureServerCertificate(ctx context.Context, kube client.Client, nn types.NamespacedName, signer *CertificateSigner) error {
	sec := &corev1.Secret{}

	err := kube.Get(ctx, nn, sec)
	if resource.IgnoreNotFound(err) != nil {
		return errors.Wrapf(err, errFmtGetTLSSecret, nn.Name)
	}

	if err == nil {
		if len(sec.Data[corev1.TLSCertKey]) != 0 || len(sec.Data[corev1.TLSPrivateKeyKey]) != 0 || len(sec.Data[SecretKeyCACert]) != 0 {
			e.log.Info("TLS secret contains server certificate.", "secret", nn.Name)
			return nil
		}
	}
	e.log.Info("Server certificates are empty nor not complete, generating a new pair...", "secret", nn.Name)

	cert := &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkixName,
		DNSNames:              []string{e.subject},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	keyData, certData, err := e.certificate.Generate(cert, signer)
	if err != nil {
		return errors.Wrap(err, errGenerateCertificate)
	}

	sec.Name = nn.Name
	sec.Namespace = nn.Namespace
	if e.owner != nil {
		sec.OwnerReferences = e.owner
	}
	_, err = controllerruntime.CreateOrUpdate(ctx, kube, sec, func() error {
		if sec.Data == nil {
			sec.Data = make(map[string][]byte)
		}
		sec.Data[corev1.TLSCertKey] = certData
		sec.Data[corev1.TLSPrivateKeyKey] = keyData
		sec.Data[SecretKeyCACert] = signer.certificatePEM

		return nil
	})

	return errors.Wrapf(err, errFmtCannotCreateOrUpdate, nn.Name)
}

// Run generates the TLS certificate bundle and stores it in k8s secrets
func (e *TLSCertificateGenerator) Run(ctx context.Context, kube client.Client) error {
	signer, err := e.loadOrGenerateCA(ctx, kube, types.NamespacedName{
		Name:      e.caSecretName,
		Namespace: e.namespace,
	})
	if err != nil {
		return errors.Wrap(err, errLoadOrGenerateSigner)
	}

	if err := e.ensureServerCertificate(ctx, kube, types.NamespacedName{
		Name:      *e.tlsServerSecretName,
		Namespace: e.namespace,
	}, signer); err != nil {
		return errors.Wrap(err, "could not generate server certificate")
	}

	return errors.Wrap(e.ensureClientCertificate(ctx, kube, types.NamespacedName{
		Name:      *e.tlsClientSecretName,
		Namespace: e.namespace,
	}, signer), "could not generate client certificate")
}

// GenerateServerCertificate generates a server certificate and stores it in k8s secrets
func (e *TLSCertificateGenerator) GenerateServerCertificate(ctx context.Context, kube client.Client) error {
	signer, err := e.loadOrGenerateCA(ctx, kube, types.NamespacedName{
		Name:      e.caSecretName,
		Namespace: e.namespace,
	})
	if err != nil {
		return errors.Wrap(err, errLoadOrGenerateSigner)
	}

	return e.ensureServerCertificate(ctx, kube, types.NamespacedName{
		Name:      *e.tlsServerSecretName,
		Namespace: e.namespace,
	}, signer)
}

// GenerateClientCertificate generates a client certificate and stores it in k8s secrets
func (e *TLSCertificateGenerator) GenerateClientCertificate(ctx context.Context, kube client.Client) error {
	signer, err := e.loadOrGenerateCA(ctx, kube, types.NamespacedName{
		Name:      e.caSecretName,
		Namespace: e.namespace,
	})
	if err != nil {
		return errors.Wrap(err, errLoadOrGenerateSigner)
	}

	return e.ensureClientCertificate(ctx, kube, types.NamespacedName{
		Name:      *e.tlsClientSecretName,
		Namespace: e.namespace,
	}, signer)
}

func parseCertificateSigner(key, cert []byte) (*CertificateSigner, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New(errDecodeKey)
	}

	sKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, errParseCAKey)
	}

	block, _ = pem.Decode(cert)
	if block == nil {
		return nil, errors.New(errDecodeCert)
	}

	sCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, errParseCACertificate)
	}

	return &CertificateSigner{
		key:            sKey,
		certificate:    sCert,
		certificatePEM: cert,
	}, nil
}
