/*
Copyright 2020 The Crossplane Authors.

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

package resource

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestExtractEnv(t *testing.T) {
	credentials := []byte("supersecretcreds")

	type args struct {
		e     EnvLookupFn
		creds xpv1.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EnvVarSuccess": {
			reason: "Successful extraction of credentials from environment variable",
			args: args{
				e: func(string) string { return string(credentials) },
				creds: xpv1.CommonCredentialSelectors{
					Env: &xpv1.EnvSelector{
						Name: "SECRET_CREDS",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"EnvVarFail": {
			reason: "Failed extraction of credentials from environment variable",
			args: args{
				e: func(string) string { return string(credentials) },
			},
			want: want{
				err: errors.New(errExtractEnv),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractEnv(context.TODO(), tc.args.e, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractEnv(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractEnv(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestExtractFs(t *testing.T) {
	credentials := []byte("supersecretcreds")
	mockFs := afero.NewMemMapFs()
	f, _ := mockFs.Create("credentials.txt")
	f.Write(credentials)
	f.Close()

	type args struct {
		fs    afero.Fs
		creds xpv1.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"FsSuccess": {
			reason: "Successful extraction of credentials from filesystem",
			args: args{
				fs: mockFs,
				creds: xpv1.CommonCredentialSelectors{
					Fs: &xpv1.FsSelector{
						Path: "credentials.txt",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"FsFailure": {
			reason: "Failed extraction of credentials from filesystem",
			args: args{
				fs: mockFs,
			},
			want: want{
				err: errors.New(errExtractFs),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractFs(context.TODO(), tc.args.fs, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractFs(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractFs(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestExtractSecret(t *testing.T) {
	errBoom := errors.New("boom")
	credentials := []byte("supersecretcreds")

	type args struct {
		client client.Client
		creds  xpv1.CommonCredentialSelectors
	}

	type want struct {
		b   []byte
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SecretSuccess": {
			reason: "Successful extraction of credentials from Secret",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
						s, _ := o.(*corev1.Secret)
						s.Data = map[string][]byte{
							"creds": credentials,
						}
						return nil
					}),
				},
				creds: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "super",
							Namespace: "secret",
						},
						Key: "creds",
					},
				},
			},
			want: want{
				b: credentials,
			},
		},
		"SecretFailureNotDefined": {
			reason: "Failed extraction of credentials from Secret when key not defined",
			args:   args{},
			want: want{
				err: errors.New(errExtractSecretKey),
			},
		},
		"SecretFailureGet": {
			reason: "Failed extraction of credentials from Secret when client fails",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(client.Object) error {
						return errBoom
					}),
				},
				creds: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{
							Name:      "super",
							Namespace: "secret",
						},
						Key: "creds",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCredentialsSecret),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractSecret(context.TODO(), tc.args.client, tc.args.creds)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\npc.ExtractSecret(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.b, got); diff != "" {
				t.Errorf("\n%s\npc.ExtractSecret(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestTrack(t *testing.T) {
	errBoom := errors.New("boom")
	name := "provisional"

	type fields struct {
		c  Applicator
		of ProviderConfigUsage
	}

	type args struct {
		ctx context.Context
		mg  Managed
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   error
	}{
		"MissingRef": {
			reason: "An error that satisfies IsMissingReference should be returned if the managed resource has no provider config reference",
			fields: fields{
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{},
			},
			want: errMissingRef{errors.New(errMissingPCRef)},
		},
		"NopUpdate": {
			reason: "No error should be returned if the apply fails because it would be a no-op",
			fields: fields{
				c: ApplyFn(func(c context.Context, r client.Object, ao ...ApplyOption) error {
					for _, fn := range ao {
						// Exercise the MustBeControllableBy and AllowUpdateIf
						// ApplyOptions. The former should pass because the
						// current object has no controller ref. The latter
						// should return an error that satisfies IsNotAllowed
						// because the current object has the same PC ref as the
						// new one we would apply.
						current := &fake.ProviderConfigUsage{
							RequiredProviderConfigReferencer: fake.RequiredProviderConfigReferencer{
								Ref: xpv1.Reference{Name: name},
							},
						}
						if err := fn(context.TODO(), current, nil); err != nil {
							return err
						}
					}
					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{
					ProviderConfigReferencer: fake.ProviderConfigReferencer{
						Ref: &xpv1.Reference{Name: name},
					},
				},
			},
			want: nil,
		},
		"ApplyError": {
			reason: "Errors applying the ProviderConfigUsage should be returned",
			fields: fields{
				c: ApplyFn(func(c context.Context, r client.Object, ao ...ApplyOption) error {
					return errBoom
				}),
				of: &fake.ProviderConfigUsage{},
			},
			args: args{
				mg: &fake.Managed{
					ProviderConfigReferencer: fake.ProviderConfigReferencer{
						Ref: &xpv1.Reference{Name: name},
					},
				},
			},
			want: errors.Wrap(errBoom, errApplyPCU),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ut := &ProviderConfigUsageTracker{c: tc.fields.c, of: tc.fields.of}
			got := ut.Track(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nut.Track(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
		})
	}
}
