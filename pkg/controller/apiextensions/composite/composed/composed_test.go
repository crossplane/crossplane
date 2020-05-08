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

package composed

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

var (
	NopConfigure = ConfigureFn(func(_ resource.Composite, _ resource.Composed, _ v1alpha1.ComposedTemplate) error {
		return nil
	})
	NopOverlay = OverlayFn(func(_ resource.Composite, _ resource.Composed, _ v1alpha1.ComposedTemplate) error {
		return nil
	})
	NopFetcher = FetchFn(func(_ context.Context, _ resource.Composed, _ v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error) {
		return nil, nil
	})
)

func TestCompose(t *testing.T) {
	errBoom := errors.New("boom")

	// TODO(muvaf): fake.Composite and fake.Composed should have With* functions
	// for easier configurations.
	cp := &fake.Composite{
		ObjectMeta: metav1.ObjectMeta{
			Name: "composite",
		},
	}
	cd := &fake.Composed{
		ObjectMeta: metav1.ObjectMeta{
			Name: "composed",
		},
	}
	cd.SetConditions(runtimev1alpha1.Available())
	conn := managed.ConnectionDetails{
		"cool": []byte("data"),
	}

	boundCD := cd.DeepCopyObject().(*fake.Composed)
	meta.AddOwnerReference(boundCD, meta.AsController(meta.ReferenceTo(cp, cp.GetObjectKind().GroupVersionKind())))

	type args struct {
		composer *Composer
		cp       resource.Composite
		cd       resource.Composed
		t        v1alpha1.ComposedTemplate
	}
	type want struct {
		err error
		obs Observation
		cd  resource.Composed
	}

	cases := map[string]struct {
		reason string
		args
		want
	}{
		"ConfigureFailed": {
			reason: "Failure of configuration should return error",
			args: args{
				composer: NewComposer(&test.MockClient{},
					WithConfigurator(ConfigureFn(func(_ resource.Composite, _ resource.Composed, _ v1alpha1.ComposedTemplate) error {
						return errBoom
					}))),
				cd: &fake.Composed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errConfigure),
			},
		},
		"OverlayFailed": {
			reason: "Failure of overlay should return error",
			args: args{
				composer: NewComposer(&test.MockClient{},
					WithConfigurator(NopConfigure),
					WithOverlayApplicator(OverlayFn(func(_ resource.Composite, _ resource.Composed, _ v1alpha1.ComposedTemplate) error {
						return errBoom
					}))),
				cd: &fake.Composed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errOverlay),
			},
		},
		"FetchFailed": {
			reason: "Failure of fetching connection details should return error",
			args: args{
				composer: NewComposer(&test.MockClient{},
					WithConfigurator(NopConfigure),
					WithOverlayApplicator(NopOverlay),
					WithConnectionDetailFetcher(FetchFn(func(_ context.Context, _ resource.Composed, _ v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, errBoom
					}))),
				cd: &fake.Composed{},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchSecret),
			},
		},
		"ApplyFailed": {
			reason: "Failure of apply should return error",
			args: args{
				composer: NewComposer(&test.MockClient{},
					WithConfigurator(NopConfigure),
					WithOverlayApplicator(NopOverlay),
					WithConnectionDetailFetcher(NopFetcher),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{},
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return errBoom
						}),
					})),
				cd: &fake.Composed{},
				cp: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errApply),
			},
		},
		"Success": {
			reason: "Observation should include the right information",
			args: args{
				composer: NewComposer(nil,
					WithConfigurator(NopConfigure),
					WithOverlayApplicator(NopOverlay),
					WithConnectionDetailFetcher(FetchFn(func(_ context.Context, _ resource.Composed, _ v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return conn, nil
					})),
					WithClientApplicator(resource.ClientApplicator{
						Client: test.NewMockClient(),
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					})),
				cd: cd,
				cp: &fake.Composite{},
			},
			want: want{
				obs: Observation{
					Ref:               *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind()),
					ConnectionDetails: conn,
					Ready:             true,
				},
				cd: boundCD,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			obs, err := tc.args.composer.Compose(context.Background(), tc.args.cp, tc.args.cd, tc.args.t)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.obs, obs); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}

}
