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

var (
	caCertSecretName    = "crossplane-root-ca"
	tlsServerSecretName = "tls-server-certs"
	tlsClientSecretName = "tls-client-certs"
	secretNS            = "crossplane-system"
	subject             = "crossplane"
	owner               = []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "provider",
			Name:       "my-provider",
			UID:        "my-uid",
		},
	}
)

func TestTLSCertificateGenerator_Run(t *testing.T) {
	type args struct {
		kube        client.Client
		certificate CertificateGenerator
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtCannotCreateOrUpdate, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"SuccessfulLoadedCA": {
			reason: "It should return no error after loading the CA from the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return []byte("test-key"), []byte("test-cert"), nil
					},
				},
			},
		},
		"CannotParseCertificateSigner": {
			reason: "It should return error if the CA secret cannot be parsed.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrap(errors.New(errDecodeCert), errLoadOrGenerateSigner),
			},
		},
		"CannotGetServerSecret": {
			reason: "It should return error if the server secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsServerSecretName), "could not generate server certificate"),
			},
		},
		"CannotGetClientSecret": {
			reason: "It should return error if the client secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsClientSecretName), "could not generate client certificate"),
			},
		},
		"SuccessfulGeneratedCA": {
			reason: "It should be successful if the CA and TLS certificates are generated and put into the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
			},
		},
		"SuccessfulCertificatesComplete": {
			reason: "It should be successful if the CA and TLS certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return nil, nil, errBoom
					},
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return nil, nil, errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGenerateCertificate), "could not generate server certificate"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewTLSCertificateGenerator(secretNS, caCertSecretName, tlsServerSecretName, tlsClientSecretName, subject)
			e.certificate = tc.args.certificate

			err := e.Run(context.Background(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTLSCertificateGenerator_GenerateServerCertificate(t *testing.T) {
	type args struct {
		kube        client.Client
		certificate CertificateGenerator
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"CannotGetServerSecret": {
			reason: "It should return error if the server secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsServerSecretName),
			},
		},
		"SuccessfulServerSecretComplete": {
			reason: "It should be successful if the server certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{err: nil},
		},
		"SuccessfulGeneratedServerCert": {
			reason: "It should be successful if the server certificate is generated and put into the Secret.",
			args: args{

				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewTLSCertificateGenerator(secretNS, caCertSecretName, tlsServerSecretName, tlsClientSecretName, subject, TLSCertificateGeneratorWithOwner(owner))
			e.certificate = tc.args.certificate

			err := e.GenerateServerCertificate(context.Background(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTLSCertificateGenerator_GenerateClientCertificate(t *testing.T) {
	type args struct {
		kube        client.Client
		certificate CertificateGenerator
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
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name != caCertSecretName || key.Namespace != secretNS {
							return errors.New("unexpected secret name or namespace")
						}

						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(errBoom, errFmtGetTLSSecret, caCertSecretName), errLoadOrGenerateSigner),
			},
		},
		"CannotGetClientSecret": {
			reason: "It should return error if the client secret cannot be retrieved.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtGetTLSSecret, tlsClientSecretName),
			},
		},
		"SuccessfulClientSecretComplete": {
			reason: "It should be successful if the client certificates are already in the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			want: want{err: nil},
		},
		"SuccessfulGeneratedClientCert": {
			reason: "It should be successful if the client certificate is generated and put into the Secret.",
			args: args{

				kube: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
					MockUpdate: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
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
					MockGenerate: func(cert *x509.Certificate, signer *CertificateSigner) ([]byte, []byte, error) {
						return []byte(caKey), []byte(caCert), nil
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewTLSCertificateGenerator(secretNS, caCertSecretName, tlsServerSecretName, tlsClientSecretName, subject, TLSCertificateGeneratorWithOwner(owner))
			e.certificate = tc.args.certificate

			err := e.GenerateClientCertificate(context.Background(), tc.args.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
