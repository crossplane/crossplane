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

package composite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

type MockComposer struct {
	res CompositionResult
	err error
}

func (c *MockComposer) Compose(_ context.Context, _ resource.Composite, _ CompositionRequest) (CompositionResult, error) {
	return c.res, c.err
}

func TestFallbackComposer(t *testing.T) {
	errBoom := errors.New("boom")
	conns := managed.ConnectionDetails{"a": []byte("b")}

	type params struct {
		preferred Composer
		fallback  Composer
		fn        TriggerFn
	}
	type args struct {
		ctx context.Context
		xr  resource.Composite
		req CompositionRequest
	}
	type want struct {
		res CompositionResult
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"SimplePreferred": {
			reason: "We should call the preferred Composer if the TriggerFn returns false.",
			params: params{
				preferred: &MockComposer{res: CompositionResult{
					ConnectionDetails: conns,
				}},
				fallback: &MockComposer{res: CompositionResult{}},
				fn: func(ctx context.Context, xr resource.Composite, req CompositionRequest) (bool, error) {
					// Don't fall back.
					return false, nil
				},
			},
			want: want{
				res: CompositionResult{
					ConnectionDetails: conns,
				},
			},
		},
		"SimpleFallback": {
			reason: "We should call the fallback Composer if the TriggerFn returns true.",
			params: params{
				preferred: &MockComposer{res: CompositionResult{}},
				fallback: &MockComposer{res: CompositionResult{
					ConnectionDetails: conns,
				}},
				fn: func(ctx context.Context, xr resource.Composite, req CompositionRequest) (bool, error) {
					// Fall back.
					return true, nil
				},
			},
			want: want{
				res: CompositionResult{
					ConnectionDetails: conns,
				},
			},
		},
		"TriggerFnError": {
			reason: "We should return any error returned by the trigger function.",
			params: params{
				preferred: &MockComposer{res: CompositionResult{}},
				fallback:  &MockComposer{res: CompositionResult{}},
				fn: func(ctx context.Context, xr resource.Composite, req CompositionRequest) (bool, error) {
					return true, errBoom
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errTriggerFn),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewFallBackComposer(tc.params.preferred, tc.params.fallback, tc.params.fn)
			res, err := c.Compose(tc.args.ctx, tc.args.xr, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRender(...): -want, +got:\n%s", tc.reason, diff)
			}

			// We need to EquateErrors here for RenderErrors.
			if diff := cmp.Diff(tc.want.res, res, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nComposer(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFallBackForPatchAndTransform(t *testing.T) {
	type params struct {
		c client.Reader
	}
	type args struct {
		ctx context.Context
		xr  resource.Composite
		req CompositionRequest
	}
	type want struct {
		fallback bool
		err      error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"HasResources": {
			reason: "We should fallback if the supplied CompositionRevision has resource templates.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{}, // A resource.
							},
						},
					},
				},
			},
			want: want{
				fallback: true,
			},
		},
		"HasPipeline": {
			reason: "We not should fallback if the supplied CompositionRevision has only a Composition Function pipeline.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Pipeline: []v1.PipelineStep{
								{}, // A step.
							},
						},
					},
				},
			},
			want: want{
				fallback: false,
			},
		},
		"HasBoth": {
			reason: "We should fallback if the supplied CompositionRevision has both resource templates and a Composition Function pipeline.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{}, // A resource.
							},
							Pipeline: []v1.PipelineStep{
								{}, // A step.
							},
						},
					},
				},
			},
			want: want{
				fallback: true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := FallBackForPatchAndTransform(tc.params.c)
			fallback, err := fn(tc.args.ctx, tc.args.xr, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFallBackForPatchAndTransform(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.fallback, fallback); diff != "" {
				t.Errorf("\n%s\nFallBackForPatchAndTransform(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
