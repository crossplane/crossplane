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

package connection

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection/fake"
	"github.com/crossplane/crossplane-runtime/pkg/connection/store"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	resourcefake "github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

const (
	secretStoreFake = v1.SecretStoreType("Fake")
	fakeConfig      = "fake"
	testUID         = "e8587e99-15c9-4069-a530-1d2205032848"
)

const (
	errBuildStore = "cannot build store"
)

var (
	fakeStore = secretStoreFake

	errBoom = errors.New("boom")
)

func TestManagerConnectStore(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		p *v1.PublishConnectionDetailsTo
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ConfigNotFound": {
			reason: "We should return a proper error if referenced StoreConfig does not exist.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{}),
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
					},
				},
			},
			want: want{
				err: errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, fakeConfig), errGetStoreConfig),
			},
		},
		"BuildStoreError": {
			reason: "We should return any error encountered while building the Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: func(ctx context.Context, local client.Client, tCfg *tls.Config, cfg v1.SecretStoreConfig) (Store, error) {
					return nil, errors.New(errBuildStore)
				},
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
					},
				},
			},
			want: want{
				err: errors.New(errBuildStore),
			},
		},
		"SuccessfulConnect": {
			reason: "We should not return an error when connected successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{}),
				p: &v1.PublishConnectionDetailsTo{
					SecretStoreConfigRef: &v1.Reference{
						Name: fakeConfig,
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
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			_, err := m.connectStore(context.Background(), tc.args.p)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.connectStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerPublishConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		conn managed.ConnectionDetails
		so   resource.ConnectionSecretOwner
	}

	type want struct {
		published bool
		err       error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotPublishTo": {
			reason: "We should return a proper error when publish to secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteStore),
			},
		},
		"SuccessfulPublishWithOwnerUID": {
			reason: "We should return no error when published successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						if diff := cmp.Diff(testUID, s.Metadata.GetOwnerUID()); diff != "" {
							t.Errorf("\nReason: %s\nm.publishConnection(...): -want ownerUID, +got ownerUID:\n%s", testUID, diff)
						}
						return true, nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
					WriterTo: nil,
				},
			},
			want: want{
				published: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			published, err := m.PublishConnection(context.Background(), tc.args.so, tc.args.conn)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.publishConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.published, published); diff != "" {
				t.Errorf("\nReason: %s\nm.publishConnection(...): -want published, +got published:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerUnpublishConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		conn managed.ConnectionDetails
		so   resource.ConnectionSecretOwner
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotUnpublish": {
			reason: "We should return a proper error when delete from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					DeleteKeyValuesFn: func(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
						return errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteFromStore),
			},
		},
		"CannotUnpublishUnowned": {
			reason: "We should return a proper error when attempted to unpublish a secret that is not owned.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					DeleteKeyValuesFn: func(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: "00000000-1111-2222-3333-444444444444",
							},
						}
						for _, o := range do {
							if err := o(ctx, s); err != nil {
								return err
							}
						}
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtNotOwnedBy, testUID), errDeleteFromStore),
			},
		},
		"SuccessfulUnpublish": {
			reason: "We should return no error when unpublished successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					DeleteKeyValuesFn: func(ctx context.Context, s *store.Secret, do ...store.DeleteOption) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						for _, o := range do {
							if err := o(ctx, s); err != nil {
								return err
							}
						}
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
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
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			err := m.UnpublishConnection(context.Background(), tc.args.so, tc.args.conn)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.unpublishConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerFetchConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		so resource.ConnectionSecretOwner
	}

	type want struct {
		conn managed.ConnectionDetails
		err  error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetails": {
			reason: "We should return no error if resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				so: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnect": {
			reason: "We should return any error encountered while connecting to Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotFetch": {
			reason: "We should return a proper error when fetch from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						return errBoom
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadStore),
			},
		},
		"SuccessfulFetch": {
			reason: "We should return no error when fetched successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Data = store.KeyValues{
							"key1": []byte("val1"),
						}
						return nil
					},
				}),
				so: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				conn: map[string][]byte{
					"key1": []byte("val1"),
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			got, err := m.FetchConnection(context.Background(), tc.args.so)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.FetchConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.conn, got); diff != "" {
				t.Errorf("\nReason: %s\nm.FetchConnection(...): -want connDetails, +got connDetails:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestManagerPropagateConnection(t *testing.T) {
	type args struct {
		c  client.Client
		sb StoreBuilderFn

		to   resource.LocalConnectionSecretOwner
		from resource.ConnectionSecretOwner
	}

	type want struct {
		propagated bool
		err        error
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoConnectionDetailsSource": {
			reason: "We should return no error if source resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				from: &resourcefake.MockConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"NoConnectionDetailsDestination": {
			reason: "We should return no error if destination resource does not want to expose a connection secret.",
			args: args{
				c: &test.MockClient{
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				from: &resourcefake.MockConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
				to:   &resourcefake.MockLocalConnectionSecretOwner{To: nil},
			},
			want: want{
				err: nil,
			},
		},
		"CannotConnectSource": {
			reason: "We should return any error encountered while connecting to Source Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotFetch": {
			reason: "We should return a proper error when fetch from secret store failed.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						return errBoom
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{To: &v1.PublishConnectionDetailsTo{}},
			},
			want: want{
				err: errors.Wrap(errBoom, errReadStore),
			},
		},
		"CannotEstablishControlOfUnowned": {
			reason: "We should return a proper error if source secret is not owned by any resource",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == fakeConfig {
							*obj.(*fake.StoreConfig) = fake.StoreConfig{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeConfig,
								},
								Config: v1.SecretStoreConfig{
									Type: &fakeStore,
								},
							}
							return nil
						}

						return kerrors.NewNotFound(schema.GroupResource{}, "non-existing")
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{}
						return nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
					WriterTo: nil,
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.New(errSecretConflict),
			},
		},
		"CannotEstablishControlOfAnotherOwner": {
			reason: "We should return a proper error if source secret is owned by another resource",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == fakeConfig {
							*obj.(*fake.StoreConfig) = fake.StoreConfig{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeConfig,
								},
								Config: v1.SecretStoreConfig{
									Type: &fakeStore,
								},
							}
							return nil
						}

						return kerrors.NewNotFound(schema.GroupResource{}, "non-existing")
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: "00000000-1111-2222-3333-444444444444",
							},
						}
						return nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
					WriterTo: nil,
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.New(errSecretConflict),
			},
		},
		"CannotConnectDestination": {
			reason: "We should return any error encountered while connecting to Destination Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == fakeConfig {
							*obj.(*fake.StoreConfig) = fake.StoreConfig{
								ObjectMeta: metav1.ObjectMeta{
									Name: fakeConfig,
								},
								Config: v1.SecretStoreConfig{
									Type: &fakeStore,
								},
							}
							return nil
						}

						return kerrors.NewNotFound(schema.GroupResource{}, "non-existing")
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
					WriterTo: nil,
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: "non-existing",
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrapf(kerrors.NewNotFound(schema.GroupResource{}, "non-existing"), errGetStoreConfig), errConnectStore),
			},
		},
		"CannotPublish": {
			reason: "We should return any error encountered while publishing to Destination Store.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return false, errBoom
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errWriteStore),
			},
		},
		"DestinationSecretCannotBeOwned": {
			reason: "We should return a proper error if destination secret cannot be owned by destination resource.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						for _, o := range wo {
							if err := o(context.Background(), &store.Secret{
								Metadata: &v1.ConnectionSecretMetadata{
									Labels: map[string]string{
										v1.LabelKeyOwnerUID: "00000000-1111-2222-3333-444444444444",
									},
								},
							}, s); err != nil {
								return false, err
							}
						}
						return true, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtNotOwnedBy, testUID), errWriteStore),
			},
		},
		"SuccessfulPropagateCreated": {
			reason: "We should return no error when propagated successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						return true, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				propagated: true,
			},
		},
		"CannotPropagateToUnowned": {
			reason: "We should return a proper error when attempted to update an unowned secret.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						for _, o := range wo {
							if err := o(context.Background(), &store.Secret{
								Data: map[string][]byte{
									"some-key": []byte("some-val"),
								},
							}, s); err != nil {
								return false, err
							}
						}
						return true, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				propagated: false,
				err:        errors.Wrap(errors.Errorf(errFmtNotOwnedBy, ""), errWriteStore),
			},
		},
		"SuccessfulPropagateUpdated": {
			reason: "We should return no error when propagated successfully by updating an already owned secret.",
			args: args{
				c: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						*obj.(*fake.StoreConfig) = fake.StoreConfig{
							ObjectMeta: metav1.ObjectMeta{
								Name: fakeConfig,
							},
							Config: v1.SecretStoreConfig{
								Type: &fakeStore,
							},
						}
						return nil
					},
					MockScheme: test.NewMockSchemeFn(resourcefake.SchemeWith(&fake.StoreConfig{})),
				},
				sb: fakeStoreBuilderFn(fake.SecretStore{
					ReadKeyValuesFn: func(ctx context.Context, n store.ScopedName, s *store.Secret) error {
						s.Metadata = &v1.ConnectionSecretMetadata{
							Labels: map[string]string{
								v1.LabelKeyOwnerUID: testUID,
							},
						}
						return nil
					},
					WriteKeyValuesFn: func(ctx context.Context, s *store.Secret, wo ...store.WriteOption) (bool, error) {
						for _, o := range wo {
							if err := o(context.Background(), &store.Secret{
								Metadata: &v1.ConnectionSecretMetadata{
									Labels: map[string]string{
										v1.LabelKeyOwnerUID: testUID,
									},
								},
								Data: map[string][]byte{
									"some-key": []byte("some-val"),
								},
							}, s); err != nil {
								return false, err
							}
						}
						return true, nil
					},
				}),
				from: &resourcefake.MockConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
				to: &resourcefake.MockLocalConnectionSecretOwner{
					ObjectMeta: metav1.ObjectMeta{
						UID: testUID,
					},
					To: &v1.PublishConnectionDetailsTo{
						SecretStoreConfigRef: &v1.Reference{
							Name: fakeConfig,
						},
					},
				},
			},
			want: want{
				propagated: true,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m := NewDetailsManager(tc.args.c, resourcefake.GVK(&fake.StoreConfig{}), WithStoreBuilder(tc.args.sb))

			got, err := m.PropagateConnection(context.Background(), tc.args.to, tc.args.from)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nm.PropagateConnection(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.propagated, got); diff != "" {
				t.Errorf("\nReason: %s\nm.PropagateConnection(...): -want propagated, +got propagated:\n%s", tc.reason, diff)
			}
		})
	}
}

func fakeStoreBuilderFn(ss fake.SecretStore) StoreBuilderFn {
	return func(_ context.Context, _ client.Client, tcfg *tls.Config, cfg v1.SecretStoreConfig) (Store, error) {
		if *cfg.Type == fakeStore {
			return &ss, nil
		}
		return nil, errors.Errorf(errFmtUnknownSecretStore, *cfg.Type)
	}
}
