// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package initializer

import "crypto/x509"

// MockCertificateGenerator is used to mock certificate generator because the
// real one takes a few seconds to generate a real certificate.
type MockCertificateGenerator struct {
	MockGenerate func(*x509.Certificate, *CertificateSigner) (key []byte, crt []byte, err error)
}

// Generate calls MockGenerate.
func (m *MockCertificateGenerator) Generate(cert *x509.Certificate, signer *CertificateSigner) (key []byte, crt []byte, err error) {
	return m.MockGenerate(cert, signer)
}
