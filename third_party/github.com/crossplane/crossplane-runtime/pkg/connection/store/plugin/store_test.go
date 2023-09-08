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

package plugin

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	ess "github.com/crossplane/crossplane-runtime/apis/proto/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store/plugin/fake"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	parentPath = "crossplane-system"
	secretName = "ess-test-secret"
)

var (
	errBoom = errors.New("boom")
)

func TestReadKeyValues(t *testing.T) {
	type args struct {
		sn     store.ScopedName
		client ess.ExternalSecretStorePluginServiceClient
	}
	type want struct {
		out *store.Secret
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileGetting": {
			reason: "Should return a proper error if secret cannot be obtained",
			args: args{
				client: &fake.ExternalSecretStorePluginServiceClient{
					GetSecretFn: func(ctx context.Context, req *ess.GetSecretRequest, opts ...grpc.CallOption) (*ess.GetSecretResponse, error) {
						return nil, errBoom
					},
				},
			},
			want: want{
				out: &store.Secret{},
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"SuccessfulGet": {
			reason: "Should return key values from a secret with scope",
			args: args{
				sn: store.ScopedName{
					Name:  secretName,
					Scope: parentPath,
				},
				client: &fake.ExternalSecretStorePluginServiceClient{
					GetSecretFn: func(ctx context.Context, req *ess.GetSecretRequest, opts ...grpc.CallOption) (*ess.GetSecretResponse, error) {
						if diff := cmp.Diff(filepath.Join(parentPath, secretName), req.Secret.ScopedName); diff != "" {
							t.Errorf("r: -want, +got:\n%s", diff)
						}
						sec := &ess.Secret{
							ScopedName: req.Secret.ScopedName,
							Data: map[string][]byte{
								"data1": []byte("val1"),
								"data2": []byte("val2"),
							},
							Metadata: map[string]string{
								"meta1": "val1",
								"meta2": "val2",
							},
						}
						res := &ess.GetSecretResponse{
							Secret: sec,
						}
						return res, nil
					},
				},
			},
			want: want{
				out: &store.Secret{
					ScopedName: store.ScopedName{
						Name:  secretName,
						Scope: parentPath,
					},
					Data: map[string][]byte{
						"data1": []byte("val1"),
						"data2": []byte("val2"),
					},
					Metadata: &v1.ConnectionSecretMetadata{
						Labels: map[string]string{
							"meta1": "val1",
							"meta2": "val2",
						},
					},
				},

				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ss := &SecretStore{
				client: tc.args.client,
				config: &v1.Config{
					APIVersion: "v1alpha1",
					Kind:       "VaultConfig",
					Name:       "ess-test",
				},
			}
			s := &store.Secret{}

			err := ss.ReadKeyValues(ctx, tc.args.sn, s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.out, s); diff != "" {
				t.Errorf("\n%s\nss.ReadKeyValues(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWriteKeyValues(t *testing.T) {
	type args struct {
		client ess.ExternalSecretStorePluginServiceClient
	}
	type want struct {
		isChanged bool
		err       error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileWriting": {
			reason: "Should return a proper error if secret cannot be applied",
			args: args{
				client: &fake.ExternalSecretStorePluginServiceClient{
					ApplySecretFn: func(ctx context.Context, req *ess.ApplySecretRequest, opts ...grpc.CallOption) (*ess.ApplySecretResponse, error) {
						return nil, errBoom
					},
				},
			},
			want: want{
				isChanged: false,
				err:       errors.Wrap(errBoom, errApply),
			},
		},
		"SuccessfulWrite": {
			reason: "Should return isChanged true",
			args: args{
				client: &fake.ExternalSecretStorePluginServiceClient{
					ApplySecretFn: func(ctx context.Context, req *ess.ApplySecretRequest, opts ...grpc.CallOption) (*ess.ApplySecretResponse, error) {
						resp := &ess.ApplySecretResponse{
							Changed: true,
						}

						return resp, nil
					},
				},
			},
			want: want{
				isChanged: true,
				err:       nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ss := &SecretStore{
				client: tc.args.client,
				config: &v1.Config{
					APIVersion: "v1alpha1",
					Kind:       "VaultConfig",
					Name:       "ess-test",
				},
			}
			s := &store.Secret{}

			isChanged, err := ss.WriteKeyValues(ctx, s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.WriteKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.isChanged, isChanged); diff != "" {
				t.Errorf("\n%s\nss.WriteKeyValues(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeleteKeyValues(t *testing.T) {
	type args struct {
		client ess.ExternalSecretStorePluginServiceClient
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ErrorWhileDeleting": {
			reason: "Should return a proper error if key values cannot be deleted",
			args: args{
				client: &fake.ExternalSecretStorePluginServiceClient{
					DeleteKeysFn: func(ctx context.Context, req *ess.DeleteKeysRequest, opts ...grpc.CallOption) (*ess.DeleteKeysResponse, error) {
						return nil, errBoom
					}},
			},
			want: want{
				err: errors.Wrap(errBoom, errDelete),
			},
		},
		"SuccessfulDelete": {
			reason: "Should not return error",
			args: args{
				client: &fake.ExternalSecretStorePluginServiceClient{
					DeleteKeysFn: func(ctx context.Context, req *ess.DeleteKeysRequest, opts ...grpc.CallOption) (*ess.DeleteKeysResponse, error) {
						return nil, nil
					},
				},
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			ss := &SecretStore{
				client: tc.args.client,
				config: &v1.Config{
					APIVersion: "v1alpha1",
					Kind:       "VaultConfig",
					Name:       "ess-test",
				},
			}
			s := &store.Secret{}

			err := ss.DeleteKeyValues(ctx, s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nss.DeletKeyValues(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
