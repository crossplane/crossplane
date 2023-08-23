package initializer

import (
	"context"
	"crypto/x509"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	// ESSCACertSecretName is the name of the secret that will store CA certificates
	ESSCACertSecretName = "ess-ca-certs"

	errFmtGetESSSecret = "cannot get ess secret: %s"
)

// ESSCertificateGenerator is an initializer step that will find the given secret
// and fill its tls.crt, tls.key and ca.crt fields to be used for External Secret
// Store plugins
type ESSCertificateGenerator struct {
	namespace        string
	clientSecretName string
	serverSecretName string
	certificate      CertificateGenerator
	log              logging.Logger
}

// ESSCertificateGeneratorOption is used to configure ESSCertificateGenerator behavior.
type ESSCertificateGeneratorOption func(*ESSCertificateGenerator)

// ESSCertificateGeneratorWithLogger returns an ESSCertificateGeneratorOption that configures logger
func ESSCertificateGeneratorWithLogger(log logging.Logger) ESSCertificateGeneratorOption {
	return func(g *ESSCertificateGenerator) {
		g.log = log
	}
}

// NewESSCertificateGenerator returns a new ESSCertificateGenerator.
func NewESSCertificateGenerator(ns, clientSecret, serverSecret string, opts ...ESSCertificateGeneratorOption) *ESSCertificateGenerator {
	e := &ESSCertificateGenerator{
		namespace:        ns,
		clientSecretName: clientSecret,
		serverSecretName: serverSecret,
		certificate:      NewCertGenerator(),
		log:              logging.NewNopLogger(),
	}

	for _, f := range opts {
		f(e)
	}
	return e
}

func (e *ESSCertificateGenerator) loadOrGenerateCA(ctx context.Context, kube client.Client, nn types.NamespacedName) (*CertificateSigner, error) {
	caSecret := &corev1.Secret{}

	err := kube.Get(ctx, nn, caSecret)
	if resource.IgnoreNotFound(err) != nil {
		return nil, errors.Wrapf(err, errFmtGetESSSecret, nn.Name)
	}

	if err == nil {
		kd := caSecret.Data[corev1.TLSPrivateKeyKey]
		cd := caSecret.Data[corev1.TLSCertKey]
		if len(kd) != 0 && len(cd) != 0 {
			e.log.Info("ESS CA secret is complete.")
			return parseCertificateSigner(kd, cd)
		}
	}
	e.log.Info("ESS CA secret is empty or not complete, generating a new CA...")

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

func (e *ESSCertificateGenerator) ensureCertificateSecret(ctx context.Context, kube client.Client, nn types.NamespacedName, cert *x509.Certificate, signer *CertificateSigner) error {
	sec := &corev1.Secret{}

	err := kube.Get(ctx, nn, sec)
	if resource.IgnoreNotFound(err) != nil {
		return errors.Wrapf(err, errFmtGetESSSecret, nn.Name)
	}

	if err == nil {
		if len(sec.Data[SecretKeyCACert]) != 0 && len(sec.Data[corev1.TLSCertKey]) != 0 && len(sec.Data[corev1.TLSPrivateKeyKey]) != 0 {
			e.log.Info("ESS secret is complete.", "secret", nn.Name)
			return nil
		}
	}
	e.log.Info("ESS secret is empty or not complete, generating a new certificate...", "secret", nn.Name)

	keyData, certData, err := e.certificate.Generate(cert, signer)
	if err != nil {
		return errors.Wrap(err, errGenerateCertificate)
	}

	sec.Name = nn.Name
	sec.Namespace = nn.Namespace
	_, err = controllerruntime.CreateOrUpdate(ctx, kube, sec, func() error {
		sec.Data = map[string][]byte{
			corev1.TLSCertKey:       certData,
			corev1.TLSPrivateKeyKey: keyData,
			SecretKeyCACert:         signer.certificatePEM,
		}
		return nil
	})

	return errors.Wrapf(err, errFmtCannotCreateOrUpdate, nn.Name)
}

// Run generates the TLS certificate valid for ESS
func (e *ESSCertificateGenerator) Run(ctx context.Context, kube client.Client) error {
	signer, err := e.loadOrGenerateCA(ctx, kube, types.NamespacedName{
		Name:      ESSCACertSecretName,
		Namespace: e.namespace,
	})
	if err != nil {
		return errors.Wrap(err, errLoadOrGenerateSigner)
	}

	if err := e.ensureCertificateSecret(ctx, kube, types.NamespacedName{
		Name:      e.serverSecretName,
		Namespace: e.namespace,
	}, &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkixName,
		DNSNames:              []string{fmt.Sprintf("*.%s", e.namespace)},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}, signer); err != nil {
		return err
	}

	return e.ensureCertificateSecret(ctx, kube, types.NamespacedName{
		Name:      e.clientSecretName,
		Namespace: e.namespace,
	}, &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		Subject:               pkixName,
		DNSNames:              []string{"ess-client"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}, signer)
}
