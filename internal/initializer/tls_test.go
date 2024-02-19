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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	caCert = `-----BEGIN CERTIFICATE-----
MIIDkTCCAnmgAwIBAgICB+YwDQYJKoZIhvcNAQELBQAwWjEOMAwGA1UEBhMFRWFy
dGgxDjAMBgNVBAgTBUVhcnRoMQ4wDAYDVQQHEwVFYXJ0aDETMBEGA1UEChMKQ3Jv
c3NwbGFuZTETMBEGA1UEAxMKQ3Jvc3NwbGFuZTAeFw0yMzAzMjIxNTMyNTNaFw0z
MzAzMjIxNTMyNTNaMFoxDjAMBgNVBAYTBUVhcnRoMQ4wDAYDVQQIEwVFYXJ0aDEO
MAwGA1UEBxMFRWFydGgxEzARBgNVBAoTCkNyb3NzcGxhbmUxEzARBgNVBAMTCkNy
b3NzcGxhbmUwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDNmbFbNF32
pLxELihBec72qf9fIUl12saK8s6FqvH0uv1vGUbrGMkhvzbdIHo8AJ5N5KKADRe4
ZfDQBESIryFZscbTUkPIlSLWanmBuV3OojZM+G7j38cmN1Kag/fPQ5x5FNg5FhPC
3JCgl3Z/qDLcDDqx/GBgkyfEM11GkLzsJOt/8+8EjcE+mdgwQs3yV4hqUUh3RrM0
wqVDzENfP3PKtnygSQAgp3VxqbHwR2cueemSLClq0JQwNsnpQC+T+Cq8tWkZjdw8
LMJtdbtnOLvM6ofKQA0Sdi4XqaZML1nh0Cv/mGLR9dSDI5Uxl4kGySRE5d0xXC2C
ZUwP6fBuTpaxAgMBAAGjYTBfMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBQ2WbFrZwIu4lWA5tA+l/zWWCV5CDAdBgNVHREEFjAUghJj
cm9zc3BsYW5lLXJvb3QtY2EwDQYJKoZIhvcNAQELBQADggEBAGE4rcSZdWO3E4QY
BfjxBuJfM8VZUP1kllV+IrFO+PhCAFcUSOCdfJcMbdAXbA/m7f2jTHq8isDOYLfn
50/40+myheH/ZAQibC7go1VpjrZHQfanaGEFZPri0ftpQjZ2guCxrxgNA9qZa2Kz
4H1dW4eQCWZnkUOUmBwdp2RN5E0oWVrvqixdcUjmMqGyajkueScuKih6EUYnfUWO
A0N4+bBummJYPRnLNoUsKnEUsUXyQKp2jnYgGH90O71VO6r86tsvhOivwSKVq6E6
r2bka16dVPncliiFI4NBng/SFGyOSE0O1Er/BY38KEALYe7J4mLzr4NxEtib2soM
hs0Mt0k=
-----END CERTIFICATE-----`
	caKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzZmxWzRd9qS8RC4oQXnO9qn/XyFJddrGivLOharx9Lr9bxlG
6xjJIb823SB6PACeTeSigA0XuGXw0AREiK8hWbHG01JDyJUi1mp5gbldzqI2TPhu
49/HJjdSmoP3z0OceRTYORYTwtyQoJd2f6gy3Aw6sfxgYJMnxDNdRpC87CTrf/Pv
BI3BPpnYMELN8leIalFId0azNMKlQ8xDXz9zyrZ8oEkAIKd1camx8EdnLnnpkiwp
atCUMDbJ6UAvk/gqvLVpGY3cPCzCbXW7Zzi7zOqHykANEnYuF6mmTC9Z4dAr/5hi
0fXUgyOVMZeJBskkROXdMVwtgmVMD+nwbk6WsQIDAQABAoIBAQDExbrDomvnuaRh
0JdAixb0ZqD9Z/tJq3fn1hioP4JQioIxyUxhhxhAjyQwIHw8Xw8jV5Xa3iz8k7wV
KnB5LLvLf2TeLVaoa2urML5X1JQeRouXwRFIUIzmW35YWcNbf8cK71M9145UKgrV
WADWjqEWjzHB1NxcsZoWol48Qhw+GCRP88QN1CyVIXQqFWm+b8YraeUDpBt9FY3R
mrEk4WjcIsQH7fGGIwgQBxzGuZ9iVzHfJUBVUUU92wHr9i3mNPQhfmZqWEkvHhGd
JVgRxIPlyVbTtQ3Zto+nYf53f92YLYORHcUuCOazELjAErhPLjv9LDZZVVYbYbse
vXxNldnBAoGBAO13F3BcxKdFXb7e11zizHaQAoq1QlFdJYq4Shqgj5qp+BZrysEJ
Ai+KpOF3SyvAR4lCHeRDRePKX6abNIdF/ZHIlWP+MNuu35cNEqQE69214kyHlFj2
syOqz2O/CAXNoUeGwFv5prN54MpN4jaXxiXztguT7vtfV1PBUz9Rx9/JAoGBAN2l
5PBweyxC4UxG1ICsPmwE5J436sdgGMaVxnaJ76eB9PrIaadcSwkmZQfsdbMJgV8f
pj6dGdwJOS/rs/CTvlZ3FYCg6L2BKYb/9IMXuMta3VuJR7KpFYRUbkHw9KYacp7y
Pq2B1dmn8xY+83PBQSg4NzqDig3MBc0KtTE3GIOpAoGAcZIzs5mqtBWI8HDDr7kI
8OuPS6fFQAS8n8vkJTgFdoM0FAUZw5j7YqF8mhjj6tjbXdoxUaqbEocHmDdCuC/R
RpgYWuqHk4nfhe7Kq4dvB2qmANQXLzVOGBDpf1suCxh9uifIeDS+dbgkupzlRBby
vdQBjSgDdFX0/inIFtCWN4ECgYEA3RjE3Mt3MtmsIAhvpcMrqVjgLKueuS80x7NT
+57wvuk11IviSJ4aA5CXK2ZGqkeLE7ZggQj5aLKSpyi5n/vg3COCAYOBZrfXEuFz
qOka309OjCbOrHtaCVynd4PCp4auW7tNpopjJfEQ3VoCQ6+9LT+WZ/oa1lR0XOqX
f/Zzr7ECgYBo/oyGxVlOZ51k27m0SB0WQohmxaTDpLl0841vVX62jQpEPr0jropj
CoRJv9VaKVXp8dgkULxiy0C35iGbCLVK5o/qROcRMJlw1rfCM6Gxv7LppqwvmYHI
aAJ/I/MBEGIitV7G1MRwVz56Yvv8cP/mQ712faD7iwBHC9bqO6umCA==
-----END RSA PRIVATE KEY-----`
)

var (
	caCertSecretName    = "crossplane-root-ca"
	tlsServerSecretName = "tls-server-certs"
	tlsClientSecretName = "tls-client-certs"
	secretNS            = "crossplane-system"
	subject             = "crossplane"
)

func TestTLSCertificateGeneratorRun(t *testing.T) {
	type args struct {
		kube        client.Client
		certificate CertificateGenerator
		opts        []TLSCertificateGeneratorOption
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CannotGetCASecret": {
			reason: "It should return error if the CA secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"CannotUpdateCASecret": {
			reason: "It should return error if the CA secret cannot be updated.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}
						s := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      caCertSecretName,
								Namespace: secretNS,
							},
							Data: map[string][]byte{
								corev1.TLSCertKey: []byte(caCert),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				certificate: NewCertGenerator(),
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtCannotCreateOrUpdate, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"SuccessfulLoadedCA": {
			reason: "It should return no error after loading the CA from the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName {
							return nil
						}
						s := &corev1.Secret{
							Data: map[string][]byte{
								corev1.TLSCertKey:       []byte(caCert),
								corev1.TLSPrivateKeyKey: []byte(caKey),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return []byte("test-key"), []byte("test-cert"), nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
		},
		"CannotParseCertificateSigner": {
			reason: "It should return error if the CA secret cannot be parsed.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}
						s := &corev1.Secret{
							Data: map[string][]byte{
								corev1.TLSCertKey:       []byte("invalid"),
								corev1.TLSPrivateKeyKey: []byte(caKey),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.New(errDecodeCert), errLoadOrGenerateSigner),
			},
		},
		"DoNothingIfNothingToDo": {
			reason: "It should no error if there is nothing to do.",
			args: args{
				kube: &test.MockClient{},
				opts: nil,
			},
			want: want{
				err: nil,
			},
		},
		"CannotGetServerSecret": {
			reason: "It should return error if the server secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name != tlsServerSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsServerSecretName), errGenerateServerCert),
			},
		},
		"CannotGetClientSecret": {
			reason: "It should return error if the client secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						if key.Name == tlsServerSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("test-cert"),
									corev1.TLSPrivateKeyKey: []byte("test-key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						if key.Name != tlsClientSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsClientSecretName), errGenerateClientCert),
			},
		},
		"SuccessfulGeneratedCA": {
			reason: "It should be successful if the CA and TLS certificates are generated and put into the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name == tlsServerSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      tlsServerSecretName,
									Namespace: secretNS,
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name == tlsClientSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      tlsClientSecretName,
									Namespace: secretNS,
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						return errors.New("unexpected secret name or namespace")
					},
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						if obj.GetName() == tlsServerSecretName && obj.GetNamespace() == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						if obj.GetName() == tlsClientSecretName && obj.GetNamespace() == secretNS {
							s := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      tlsClientSecretName,
									Namespace: secretNS,
								},
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						return errors.New("unexpected secret name or namespace")
					},
				},
				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
		},
		"SuccessfulCertificatesComplete": {
			reason: "It should be successful if the CA and TLS certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						if key.Name == tlsServerSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						if key.Name == tlsClientSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						return errors.New("unexpected secret name or namespace")
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{err: nil},
		},
		"CannotGenerateCACertificate": {
			reason: "It should return error if the CA and TLS certificates cannot be generated.",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return nil, nil, errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGenerateCA), errLoadOrGenerateSigner),
			},
		},
		"CannotGenerateCertificate": {
			reason: "It should return error if the CA and TLS certificates cannot be generated.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}
						return nil
					},
				},
				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return nil, nil, errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGenerateCertificate), "could not generate server certificate"),
			},
		},

		// OnlyServerCertificate test cases
		"OnlyServerCertificateCannotGetCASecret": {
			reason: "It should return error if the CA secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"OnlyServerCertificateCannotGetServerSecret": {
			reason: "It should return error if the server secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name != tlsServerSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsServerSecretName), errGenerateServerCert),
			},
		},
		"OnlyServerCertificateSuccessfulServerSecretComplete": {
			reason: "It should be successful if the server certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name != tlsServerSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						s := &corev1.Secret{
							Data: map[string][]byte{
								corev1.TLSCertKey:       []byte("cert"),
								corev1.TLSPrivateKeyKey: []byte("key"),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
				},
			},
			want: want{err: nil},
		},
		"OnlyServerCertificateSuccessfulGeneratedServerCert": {
			reason: "It should be successful if the server certificate is generated and put into the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name == tlsServerSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      tlsServerSecretName,
									Namespace: secretNS,
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						return errors.New("unexpected secret name or namespace")
					},
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						if obj.GetName() == tlsServerSecretName && obj.GetNamespace() == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						return errors.New("unexpected secret name or namespace")
					},
				},

				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithServerSecretName(tlsServerSecretName, []string{subject}),
				},
			},
		},
		// OnlyClientCertificate test cases
		"OnlyClientCertificateCannotGetCASecret": {
			reason: "It should return error if the CA secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"OnlyClientCertificateCannotGetClientSecret": {
			reason: "It should return error if the client secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name != tlsClientSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsClientSecretName), errGenerateClientCert),
			},
		},
		"OnlyClientCertificateSuccessfulClientSecretComplete": {
			reason: "It should be successful if the client certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name != tlsClientSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						s := &corev1.Secret{
							Data: map[string][]byte{
								corev1.TLSCertKey:       []byte("cert"),
								corev1.TLSPrivateKeyKey: []byte("key"),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
			want: want{err: nil},
		},
		"OnlyClientCertificateSuccessfulGeneratedClientCert": {
			reason: "It should be successful if the client certificate is generated and put into the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == caCertSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte(caCert),
									corev1.TLSPrivateKeyKey: []byte(caKey),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						if key.Name == tlsClientSecretName && key.Namespace == secretNS {
							s := &corev1.Secret{
								ObjectMeta: metav1.ObjectMeta{
									Name:      tlsClientSecretName,
									Namespace: secretNS,
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						return errors.New("unexpected secret name or namespace")
					},
					MockUpdate: func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
						if obj.GetName() == tlsClientSecretName && obj.GetNamespace() == secretNS {
							s := &corev1.Secret{
								Data: map[string][]byte{
									corev1.TLSCertKey:       []byte("cert"),
									corev1.TLSPrivateKeyKey: []byte("key"),
								},
							}
							s.DeepCopyInto(obj.(*corev1.Secret))
							return nil
						}

						return errors.New("unexpected secret name or namespace")
					},
				},

				certificate: &MockCertificateGenerator{
					MockGenerate: func(_ *x509.Certificate, _ *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
				opts: []TLSCertificateGeneratorOption{
					TLSCertificateGeneratorWithClientSecretName(tlsClientSecretName, []string{subject}),
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewTLSCertificateGenerator(secretNS, caCertSecretName,
				tc.args.opts...)
			e.certificate = tc.args.certificate

			err := e.Run(context.Background(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
