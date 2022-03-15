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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// MockCertificateGenerator is used to mock certificate generator because the
// real one takes a few seconds to generate a real certificate.
type MockCertificateGenerator struct {
	MockGenerate func(domain ...string) (key []byte, crt []byte, err error)
}

// Generate calls MockGenerate.
func (m *MockCertificateGenerator) Generate(domain ...string) (key []byte, crt []byte, err error) {
	return m.MockGenerate(domain...)
}

func TestRun(t *testing.T) {
	type args struct {
		kube client.Client
		ca   CertificateGenerator
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "It should be successful if the TLS certificate is generated and put into the Secret.",
			args: args{
				kube: &test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				ca: &MockCertificateGenerator{
					MockGenerate: func(_ ...string) ([]byte, []byte, error) {
						return nil, nil, nil
					},
				},
			},
		},
		"SuccessSecretAlreadyFilled": {
			reason: "It should be successful if the Secret is already filled.",
			args: args{
				kube: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						s := &corev1.Secret{
							Data: map[string][]byte{
								"tls.crt": []byte("CRT"),
								"tls.key": []byte("KEY"),
							},
						}
						s.DeepCopyInto(obj.(*corev1.Secret))
						return nil
					},
				},
			},
		},
		"SecretNotFound": {
			reason: "It should fail if the given secret cannot be fetched.",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetWebhookSecret),
			},
		},
		"CertificateCannotBeGenerated": {
			reason: "It should fail if the secret cannot be updated with the new values.",
			args: args{
				kube: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
				},
				ca: &MockCertificateGenerator{
					MockGenerate: func(_ ...string) ([]byte, []byte, error) {
						return nil, nil, errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGenerateCertificate),
			},
		},
		"UpdateFailed": {
			reason: "It should fail if the secret cannot be updated with the new values.",
			args: args{
				kube: &test.MockClient{
					MockGet:    test.NewMockGetFn(nil),
					MockUpdate: test.NewMockUpdateFn(errBoom),
				},
				ca: &MockCertificateGenerator{
					MockGenerate: func(_ ...string) ([]byte, []byte, error) {
						return []byte("key"), []byte("crt"), nil
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateWebhookSecret),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := NewWebhookCertificateGenerator(
				types.NamespacedName{},
				"crossplane-system",
				logging.NewNopLogger(),
				WithCertificateGenerator(tc.args.ca)).Run(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%sch\nRun(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
