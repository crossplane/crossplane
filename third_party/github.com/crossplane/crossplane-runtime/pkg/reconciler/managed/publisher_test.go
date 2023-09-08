/*
Copyright 2019 The Crossplane Authors.

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

package managed

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var (
	_ ConnectionPublisher = &APISecretPublisher{}
	_ ConnectionPublisher = PublisherChain{}
)

func TestPublisherChain(t *testing.T) {
	type args struct {
		ctx context.Context
		mg  resource.Managed
		c   ConnectionDetails
	}

	type want struct {
		err       error
		published bool
	}

	errBoom := errors.New("boom")

	cases := map[string]struct {
		p    ConnectionPublisher
		args args
		want want
	}{
		"EmptyChain": {
			p: PublisherChain{},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
		},
		"SuccessfulPublisher": {
			p: PublisherChain{
				ConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error) {
						return true, nil
					},
					UnpublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
			want: want{
				published: true,
			},
		},
		"PublisherReturnsError": {
			p: PublisherChain{
				ConnectionPublisherFns{
					PublishConnectionFn: func(_ context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) (bool, error) {
						return false, errBoom
					},
					UnpublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c ConnectionDetails) error {
						return nil
					},
				},
			},
			args: args{
				ctx: context.Background(),
				mg:  &fake.Managed{},
				c:   ConnectionDetails{},
			},
			want: want{
				err: errBoom,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotErr := tc.p.PublishConnection(tc.args.ctx, tc.args.mg, tc.args.c)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.published, got); diff != "" {
				t.Errorf("Publish(...): -wantPublished, +gotPublished:\n%s", diff)
			}
		})
	}
}

func TestDisabledSecretStorePublish(t *testing.T) {
	type args struct {
		mg resource.Managed
	}
	type want struct {
		published bool
		err       error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"APINotUsedNoError": {
			args: args{
				mg: &fake.Managed{},
			},
		},
		"APIUsedProperError": {
			args: args{
				mg: &fake.Managed{
					ConnectionDetailsPublisherTo: fake.ConnectionDetailsPublisherTo{
						To: &xpv1.PublishConnectionDetailsTo{},
					},
				},
			},
			want: want{
				err: errors.New(errSecretStoreDisabled),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &DisabledSecretStoreManager{}
			got, gotErr := ss.PublishConnection(context.Background(), tc.args.mg, nil)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.published, got); diff != "" {
				t.Errorf("Publish(...): -wantPublished, +gotPublished:\n%s", diff)
			}
		})
	}
}

func TestDisabledSecretStoreUnpublish(t *testing.T) {
	type args struct {
		mg resource.Managed
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"APINotUsedNoError": {
			args: args{
				mg: &fake.Managed{},
			},
		},
		"APIUsedProperError": {
			args: args{
				mg: &fake.Managed{
					ConnectionDetailsPublisherTo: fake.ConnectionDetailsPublisherTo{
						To: &xpv1.PublishConnectionDetailsTo{},
					},
				},
			},
			want: want{
				err: errors.New(errSecretStoreDisabled),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ss := &DisabledSecretStoreManager{}
			gotErr := ss.UnpublishConnection(context.Background(), tc.args.mg, nil)
			if diff := cmp.Diff(tc.want.err, gotErr, test.EquateErrors()); diff != "" {
				t.Errorf("Publish(...): -want, +got:\n%s", diff)
			}
		})
	}
}
