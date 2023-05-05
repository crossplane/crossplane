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
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	computeresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	iov1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	fnpbv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	env "github.com/crossplane/crossplane/internal/controller/apiextensions/composite/environment"
	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestPTFCompose(t *testing.T) {
	errBoom := errors.New("boom")
	details := managed.ConnectionDetails{"a": []byte("b")}

	unmarshalable := kunstructured.Unstructured{Object: map[string]interface{}{
		"you-cant-marshal-a-channel": make(chan<- string),
	}}
	_, errUnmarshalableXR := json.Marshal(&composite.Unstructured{Unstructured: unmarshalable})

	type params struct {
		kube client.Client
		o    []PTFComposerOption
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
		"FetchConnectionError": {
			reason: "We should return any error encountered while fetching the XR's connection details.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchXRConnectionDetails),
			},
		},
		"GetComposedResourcesError": {
			reason: "We should return any error encountered while getting the XR's existing composed resources.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetExistingCDs),
			},
		},
		"FunctionIOObservedError": {
			reason: "We should return any error encountered while building a FunctionIO observed object.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
				},
			},
			args: args{
				xr: &composite.Unstructured{Unstructured: unmarshalable},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errUnmarshalableXR, errMarshalXR), errBuildFunctionIOObserved),
			},
		},
		"PatchAndTransformError": {
			reason: "We should return any error encountered while running patches and transforms.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errPatchAndTransform),
			},
		},
		"FunctionIODesiredError": {
			reason: "We should return any error encountered while building a FunctionIO desired object.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						// After FunctionIOObserved has been called we replace
						// our XR with one that can't be marshalled by
						// FunctionIODesired.
						s.Composite = &composite.Unstructured{Unstructured: unmarshalable}
						return nil
					})),
				},
			},
			args: args{
				// We start with an XR that can be marshalled (i.e. by
				// FunctionIOObserved).
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errUnmarshalableXR, errMarshalXR), errBuildFunctionIODesired),
			},
		},
		"RunFunctionPipelineError": {
			reason: "We should return any error encountered while running the Composition Function pipeline.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errRunFunctionPipeline),
			},
		},
		"DeleteComposedResourcesError": {
			reason: "We should return any error encountered while deleting undesired composed resources.",
			params: params{
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteUndesiredCDs),
			},
		},
		"ApplyCompositeError": {
			reason: "We should return any error encountered while applying the XR.",
			params: params{
				kube: &test.MockClient{
					// We test through the ClientApplicator - Get is called by Apply.
					MockGet: test.NewMockGetFn(errBoom),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, "cannot get object"), errApplyXR),
			},
		},
		"DoNotApplyFailedRenders": {
			reason: "We should skip applying a composed resource if we failed to render its P&T template.",
			params: params{
				kube: &test.MockClient{
					// We test through the ClientApplicator - Get is called by Apply.
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						// Only return an error if this is a composed resource.
						if _, ok := obj.(*fake.Composed); ok {
							// We don't want to return this - if we did it would
							// imply we called Apply.
							return errBoom
						}
						return nil
					}),
					// The ClientApplicator will call Update for the XR.
					MockPatch: test.NewMockPatchFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						cds := ComposedResourceStates{
							"cool-resource": ComposedResourceState{
								ComposedResource: ComposedResource{
									ResourceName: "cool-resource",
								},
								Resource:          &fake.Composed{},
								Template:          &v1.ComposedTemplate{},
								TemplateRenderErr: errBoom,
							},
						}
						return cds, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				res: CompositionResult{Composed: []ComposedResource{{ResourceName: "cool-resource"}}},
				err: nil,
			},
		},
		"ApplyComposedError": {
			reason: "We should return any error encountered while applying a composed resource.",
			params: params{
				kube: &test.MockClient{
					// We test through the ClientApplicator - Get is called by Apply.
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						// Only return an error if this is a composed resource.
						if _, ok := obj.(*fake.Composed); ok {
							return errBoom
						}
						return nil
					}),
					// The ClientApplicator will call Update for the XR.
					MockPatch: test.NewMockPatchFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						cds := ComposedResourceStates{
							"cool-resource": ComposedResourceState{
								ComposedResource: ComposedResource{
									ResourceName: "cool-resource",
								},
								Resource: &fake.Composed{},
								Template: &v1.ComposedTemplate{},
							},
						}
						return cds, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errBoom, "cannot get object"), errFmtApplyCD, "cool-resource"),
			},
		},
		"ObserveComposedResourcesError": {
			reason: "We should return any error encountered while observing a composed resource.",
			params: params{
				kube: &test.MockClient{
					// These are both called by Apply.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						return nil, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, s *PTFCompositionState) error {
						return errBoom
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errBoom, errObserveCDs),
			},
		},
		"Success": {
			reason: "",
			params: params{
				kube: &test.MockClient{
					// These are both called by Apply.
					MockGet:   test.NewMockGetFn(nil),
					MockPatch: test.NewMockPatchFn(nil),
				},
				o: []PTFComposerOption{
					WithCompositeConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
						return details, nil
					})),
					WithComposedResourceGetter(ComposedResourceGetterFn(func(ctx context.Context, xr resource.Composite) (ComposedResourceStates, error) {
						cds := ComposedResourceStates{
							"cool-resource": ComposedResourceState{
								ComposedResource: ComposedResource{
									ResourceName: "cool-resource",
								},
								Resource: &fake.Composed{},
							},
						}
						return cds, nil
					})),
					WithPatchAndTransformer(PatchAndTransformerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState) error {
						return nil
					})),
					WithFunctionPipelineRunner(FunctionPipelineRunnerFn(func(ctx context.Context, req CompositionRequest, s *PTFCompositionState, o iov1alpha1.Observed, d iov1alpha1.Desired) error {
						return nil
					})),
					WithComposedResourceDeleter(ComposedResourceDeleterFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
					WithComposedResourceObserver(ComposedResourceObserverFn(func(ctx context.Context, s *PTFCompositionState) error {
						return nil
					})),
				},
			},
			args: args{
				xr: &fake.Composite{},
			},
			want: want{
				res: CompositionResult{
					Composed: []ComposedResource{{
						ResourceName: "cool-resource",
					}},
					ConnectionDetails: details,
				},
				err: nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			c := NewPTFComposer(tc.params.kube, tc.params.o...)
			res, err := c.Compose(tc.args.ctx, tc.args.xr, tc.args.req)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.res, res, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nCompose(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGetComposedResources(t *testing.T) {
	errBoom := errors.New("boom")
	details := managed.ConnectionDetails{"a": []byte("b")}

	type params struct {
		c client.Reader
		f managed.ConnectionDetailsFetcher
	}

	type args struct {
		ctx context.Context
		xr  resource.Composite
	}

	type want struct {
		cds ComposedResourceStates
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"UnnamedRef": {
			reason: "We should skip any resource references without names.",
			params: params{
				c: &test.MockClient{
					// We should continue past the unnamed reference and not hit
					// this error.
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								APIVersion: "example.org/v1",
								Kind:       "Unnamed",
							},
						},
					},
				},
			},
		},
		"ComposedResourceNotFound": {
			reason: "We should skip any resources that are not found.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
		},
		"GetComposedResourceError": {
			reason: "We should return any error we encounter while getting a composed resource.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetComposed),
			},
		},
		"UncontrolledComposedResource": {
			reason: "We should skip any composed resources our XR doesn't control.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						_ = meta.AddControllerReference(obj, metav1.OwnerReference{
							UID:        types.UID("someone-else"),
							Controller: pointer.Bool(true),
						})

						return nil
					}),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"AnonymousComposedResourceError": {
			reason: "We should return an error if we encounter an (unsupported) anonymous composed resource.",
			params: params{
				c: &test.MockClient{
					// We 'return' an empty resource with no annotations.
					MockGet: test.NewMockGetFn(nil),
				},
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{Name: "cool-resource"},
						},
					},
				},
			},
			want: want{
				err: errors.New(errAnonymousCD),
			},
		},
		"FetchConnectionDetailsError": {
			reason: "We should return an error if we can't fetch composed resource connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return nil, errBoom
				}),
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								Kind: "Broken",
								Name: "cool-resource-42",
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtFetchCDConnectionDetails, "cool-resource", "Broken", "cool-resource-42"),
			},
		},
		"Success": {
			reason: "We should return any composed resources and their connection details.",
			params: params{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.SetName("cool-resource-42")
						SetCompositionResourceName(obj, "cool-resource")
						return nil
					}),
				},
				f: ConnectionDetailsFetcherFn(func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
					return details, nil
				}),
			},
			args: args{
				xr: &fake.Composite{
					ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
						Refs: []corev1.ObjectReference{
							{
								APIVersion: "example.org/v1",
								Kind:       "Composed",
								Name:       "cool-resource-42",
							},
						},
					},
				},
			},
			want: want{
				cds: ComposedResourceStates{
					"cool-resource": ComposedResourceState{
						ComposedResource: ComposedResource{
							ResourceName: "cool-resource",
						},
						ConnectionDetails: details,
						Resource: func() resource.Composed {
							cd := composed.New()
							cd.SetAPIVersion("example.org/v1")
							cd.SetKind("Composed")
							cd.SetName("cool-resource-42")
							SetCompositionResourceName(cd, "cool-resource")
							return cd
						}(),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			g := NewExistingComposedResourceGetter(tc.params.c, tc.params.f)
			cds, err := g.GetComposedResources(tc.args.ctx, tc.args.xr)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cds, cds, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nGetComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFunctionIOObserved(t *testing.T) {
	unmarshalable := kunstructured.Unstructured{Object: map[string]interface{}{
		"you-cant-marshal-a-channel": make(chan<- string),
	}}
	_, errUnmarshalableXR := json.Marshal(&composite.Unstructured{Unstructured: unmarshalable})
	_, errUnmarshalableCD := json.Marshal(&composed.Unstructured{Unstructured: unmarshalable})

	type args struct {
		s *PTFCompositionState
	}
	type want struct {
		o   iov1alpha1.Observed
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MarshalCompositeError": {
			reason: "We should return an error if we can't marshal the XR.",
			args: args{
				s: &PTFCompositionState{
					Composite: &composite.Unstructured{Unstructured: unmarshalable},
				},
			},
			want: want{
				err: errors.Wrap(errUnmarshalableXR, errMarshalXR),
			},
		},
		"MarshalComposedError": {
			reason: "We should return an error if we can't marshal a composed resource.",
			args: args{
				s: &PTFCompositionState{
					Composite: composite.New(),
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							Resource: &composed.Unstructured{Unstructured: unmarshalable},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errUnmarshalableCD, errMarshalCD),
			},
		},
		"Success": {
			reason: "We should successfully build a FunctionIO Observed struct from our state.",
			args: args{
				s: &PTFCompositionState{
					Composite: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "Composite",
					})),
					ConnectionDetails: managed.ConnectionDetails{"a": []byte("b")},
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{ResourceName: "cool-resource"},
							Resource: composed.New(composed.FromReference(corev1.ObjectReference{
								APIVersion: "example.org/v2",
								Kind:       "Composed",
								Name:       "cool-resource-42",
							})),
							ConnectionDetails: managed.ConnectionDetails{"c": []byte("d")},
						},
					},
				},
			},
			want: want{
				o: iov1alpha1.Observed{
					Composite: iov1alpha1.ObservedComposite{
						Resource: runtime.RawExtension{
							Raw: []byte(`{"apiVersion":"example.org/v1","kind":"Composite"}`),
						},
						ConnectionDetails: []iov1alpha1.ExplicitConnectionDetail{{
							Name:  "a",
							Value: "b",
						}},
					},
					Resources: []iov1alpha1.ObservedResource{{
						Name: "cool-resource",
						Resource: runtime.RawExtension{
							Raw: []byte(`{"apiVersion":"example.org/v2","kind":"Composed","metadata":{"name":"cool-resource-42"}}`),
						},
						ConnectionDetails: []iov1alpha1.ExplicitConnectionDetail{{
							Name:  "c",
							Value: "d",
						}},
					}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := FunctionIOObserved(tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFunctionIOObserved(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, o, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFunctionIOObserved(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPatchAndTransform(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		composite Renderer
		composed  Renderer
	}

	type args struct {
		ctx context.Context
		req CompositionRequest
		s   *PTFCompositionState
	}

	type want struct {
		s   *PTFCompositionState
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ComposedTemplatesError": {
			reason: "We should return any error encountered while inlining a Composition's PatchSets.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{
								Patches: []v1.Patch{{
									// This reference to a non-existent patchset
									// triggers the error.
									Type:         v1.PatchTypePatchSet,
									PatchSetName: pointer.String("nonexistent-patchset"),
								}},
							}},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.Errorf(errFmtUndefinedPatchSet, "nonexistent-patchset"), errInline),
			},
		},
		// TODO(negz): Test handling of ApplyEnvironmentPatch errors.
		"CompositeRenderError": {
			reason: "We should return any error encountered while rendering an XR.",
			params: params{
				composite: RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error {
					return errBoom
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{
									Name: pointer.String("cool-resource"),
								},
							},
						},
					},
				},
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						// Corresponds to the ComposedTemplate above. The
						// resource must exist in order for the code to try
						// render the XR from it.
						"cool-resource": ComposedResourceState{
							Resource: func() *composed.Unstructured {
								r := composed.New()
								r.SetKind("Broken")
								r.SetName("cool-resource-42")
								return r
							}(),
						},
					},
				},
			},
			want: want{
				// We expect the state to be unchanged.
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							Resource: func() *composed.Unstructured {
								r := composed.New()
								r.SetKind("Broken")
								r.SetName("cool-resource-42")
								return r
							}(),
						},
					},
				},
				err: errors.Wrapf(errBoom, errFmtRenderXR, "cool-resource", "Broken", "cool-resource-42"),
			},
		},
		"ComposedRenderError": {
			reason: "We should include any error encountered while rendering a composed resource in our state.",
			params: params{
				composite: RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error {
					return nil
				}),
				composed: RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate, env *env.Environment) error {
					return errBoom
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{
								{
									Name: pointer.String("cool-resource"),
								},
							},
						},
					},
				},
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						// Corresponds to the ComposedTemplate above. The
						// resource must exist in order for the code to try
						// render the XR from it.
						"cool-resource": ComposedResourceState{
							Resource: func() *composed.Unstructured {
								r := composed.New()
								r.SetKind("Broken")
								r.SetName("cool-resource-42")
								return r
							}(),
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: func() *composed.Unstructured {
								r := composed.New()
								r.SetKind("Broken")
								r.SetName("cool-resource-42")
								return r
							}(),
							Template: &v1.ComposedTemplate{
								Name: pointer.String("cool-resource"),
							},
							TemplateRenderErr: errBoom,
						},
					},
					Events: []event.Event{
						event.Warning(reasonCompose, errors.Wrapf(errBoom, errFmtResourceName, "cool-resource")),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			pt := NewXRCDPatchAndTransformer(tc.params.composite, tc.params.composed)
			err := pt.PatchAndTransform(tc.args.ctx, tc.args.req, tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			// We need to EquateErrors here for the TemplateRenderError in state.
			if diff := cmp.Diff(tc.want.s, tc.args.s, cmpopts.EquateEmpty(), test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFunctionIODesired(t *testing.T) {
	unmarshalable := kunstructured.Unstructured{Object: map[string]interface{}{
		"you-cant-marshal-a-channel": make(chan<- string),
	}}
	_, errUnmarshalableXR := json.Marshal(&composite.Unstructured{Unstructured: unmarshalable})
	_, errUnmarshalableCD := json.Marshal(&composed.Unstructured{Unstructured: unmarshalable})

	type args struct {
		s *PTFCompositionState
	}
	type want struct {
		o   iov1alpha1.Desired
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MarshalCompositeError": {
			reason: "We should return an error if we can't marshal the XR.",
			args: args{
				s: &PTFCompositionState{
					Composite: &composite.Unstructured{Unstructured: unmarshalable},
				},
			},
			want: want{
				err: errors.Wrap(errUnmarshalableXR, errMarshalXR),
			},
		},
		"MarshalComposedError": {
			reason: "We should return an error if we can't marshal a composed resource.",
			args: args{
				s: &PTFCompositionState{
					Composite: composite.New(),
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							Template: &v1.ComposedTemplate{},
							Resource: &composed.Unstructured{Unstructured: unmarshalable},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errUnmarshalableCD, errMarshalCD),
			},
		},
		"Success": {
			reason: "We should successfully build a FunctionIO Desired struct from our state.",
			args: args{
				s: &PTFCompositionState{
					Composite: composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{
						Group:   "example.org",
						Version: "v1",
						Kind:    "Composite",
					})),
					ConnectionDetails: managed.ConnectionDetails{"a": []byte("b")},
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{ResourceName: "cool-resource"},
							Template:         &v1.ComposedTemplate{},
							Resource: composed.New(composed.FromReference(corev1.ObjectReference{
								APIVersion: "example.org/v2",
								Kind:       "Composed",
								Name:       "cool-resource-42",
							})),
						},
					},
				},
			},
			want: want{
				o: iov1alpha1.Desired{
					Composite: iov1alpha1.DesiredComposite{
						Resource: runtime.RawExtension{
							Raw: []byte(`{"apiVersion":"example.org/v1","kind":"Composite"}`),
						},
						ConnectionDetails: []iov1alpha1.ExplicitConnectionDetail{{
							Name:  "a",
							Value: "b",
						}},
					},
					Resources: []iov1alpha1.DesiredResource{{
						Name: "cool-resource",
						Resource: runtime.RawExtension{
							Raw: []byte(`{"apiVersion":"example.org/v2","kind":"Composed","metadata":{"name":"cool-resource-42"}}`),
						},
					}},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			o, err := FunctionIODesired(tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFunctionIODesired(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.o, o, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nFunctionIODesired(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunctionPipeline(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		c ContainerFunctionRunner
	}

	type args struct {
		ctx context.Context
		req CompositionRequest
		s   *PTFCompositionState
		o   iov1alpha1.Observed
		d   iov1alpha1.Desired
	}

	type want struct {
		s   *PTFCompositionState
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"UnsupportedFunctionTypeError": {
			reason: "We should return an error if asked to run an unsupported function type.",
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionType("wat"),
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtUnsupportedFnType, "wat"), errFmtRunFn, "cool-fn"),
			},
		},
		"RunContainerFunctionError": {
			reason: "We should return an error if we can't run a containerized function.",
			params: params{
				c: ContainerFunctionRunnerFn(func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
					return nil, errBoom
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionTypeContainer,
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errBoom, errFmtRunFn, "cool-fn"),
			},
		},
		"ResultError": {
			reason: "We should return the first result of Error severity.",
			params: params{
				c: ContainerFunctionRunnerFn(func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
					return &iov1alpha1.FunctionIO{
						Results: []iov1alpha1.Result{
							{
								Severity: iov1alpha1.SeverityFatal,
								Message:  errBoom.Error(),
							},
						},
					}, nil
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionTypeContainer,
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFatalResult),
			},
		},
		"ParseCompositeError": {
			reason: "We should return an error if we can't unmarshal the desired XR.",
			params: params{
				c: ContainerFunctionRunnerFn(func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
					return &iov1alpha1.FunctionIO{
						Desired: iov1alpha1.Desired{
							Composite: iov1alpha1.DesiredComposite{
								Resource: runtime.RawExtension{
									// This triggers the unmarshal error.
									Raw: []byte("}"),
								},
							},
						},
					}, nil
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionTypeContainer,
								},
							},
						},
					},
				},
			},
			want: want{
				err: errors.Wrap(json.Unmarshal([]byte("}"), nil), errUnmarshalDesiredXR),
			},
		},
		"ParseComposedError": {
			reason: "We should return an error if we can't unmarshal a desired composed resource.",
			params: params{
				c: ContainerFunctionRunnerFn(func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
					return &iov1alpha1.FunctionIO{
						Desired: iov1alpha1.Desired{
							Composite: iov1alpha1.DesiredComposite{
								Resource: runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"a/v1","kind":"XR"}`),
								},
							},
							Resources: []iov1alpha1.DesiredResource{
								{
									Name: "cool-resource",
									Resource: runtime.RawExtension{
										// This triggers the unmarshal error.
										Raw: []byte("}"),
									},
								},
							},
						},
					}, nil
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionTypeContainer,
								},
							},
						},
					},
				},
				s: &PTFCompositionState{},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: func() *composite.Unstructured {
						xr := composite.New()
						xr.SetAPIVersion("a/v1")
						xr.SetKind("XR")
						return xr
					}(),
				},
				err: errors.Wrapf(errors.Wrap(json.Unmarshal([]byte("}"), nil), errUnmarshalDesiredCD), errFmtParseDesiredCD, "cool-resource"),
			},
		},
		"Success": {
			reason: "We should update our CompositionState with the results of the pipeline.",
			params: params{
				c: ContainerFunctionRunnerFn(func(ctx context.Context, fnio *iov1alpha1.FunctionIO, fn *v1.ContainerFunction) (*iov1alpha1.FunctionIO, error) {
					return &iov1alpha1.FunctionIO{
						Desired: iov1alpha1.Desired{
							Composite: iov1alpha1.DesiredComposite{
								Resource: runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"a/v1","kind":"XR"}`),
								},
								ConnectionDetails: []v1alpha1.ExplicitConnectionDetail{
									{
										Name:  "a",
										Value: "b",
									},
								},
							},
							// Note we test ParseDesiredResource separately.
							Resources: []iov1alpha1.DesiredResource{
								{
									Name: "cool-resource",
									Resource: runtime.RawExtension{
										Raw: []byte(`{"apiVersion":"a/v1","kind":"Composed"}`),
									},
								},
							},
						},
						Results: []iov1alpha1.Result{
							{
								Severity: iov1alpha1.SeverityWarning,
								Message:  "oh no",
							},
							{
								Severity: iov1alpha1.SeverityNormal,
								Message:  "good stuff",
							},
						},
					}, nil
				}),
			},
			args: args{
				req: CompositionRequest{
					Revision: &v1.CompositionRevision{
						Spec: v1.CompositionRevisionSpec{
							Functions: []v1.Function{
								{
									Name: "cool-fn",
									Type: v1.FunctionTypeContainer,
								},
							},
						},
					},
				},
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: func() *composite.Unstructured {
						xr := composite.New()
						xr.SetAPIVersion("a/v1")
						xr.SetKind("XR")
						return xr
					}(),
					ConnectionDetails: managed.ConnectionDetails{"a": []byte("b")},
					ComposedResources: ComposedResourceStates{
						// We don't need to test that ParseDesiredResource works
						// here - we do that elsewhere - so just call it.
						"cool-resource": func() ComposedResourceState {
							dr := iov1alpha1.DesiredResource{
								Name: "cool-resource",
								Resource: runtime.RawExtension{
									Raw: []byte(`{"apiVersion":"a/v1","kind":"Composed"}`),
								},
							}
							xr := composite.New()
							xr.SetAPIVersion("a/v1")
							xr.SetKind("XR")
							cd, _ := ParseDesiredResource(dr, xr)
							return cd
						}(),
					},
					Events: []event.Event{
						event.Warning(reasonCompose, errors.New("oh no")),
						event.Normal(reasonCompose, "good stuff"),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			r := NewFunctionPipeline(tc.params.c)
			err := r.RunFunctionPipeline(tc.args.ctx, tc.args.req, tc.args.s, tc.args.o, tc.args.d)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRunFunctionPipeline(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.s, tc.args.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRunFunctionPipeline(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

type MockFunctionServer struct {
	fnpbv1alpha1.UnimplementedContainerizedFunctionRunnerServiceServer

	rsp *fnpbv1alpha1.RunFunctionResponse
	err error
}

func (s *MockFunctionServer) RunFunction(context.Context, *fnpbv1alpha1.RunFunctionRequest) (*fnpbv1alpha1.RunFunctionResponse, error) {
	return s.rsp, s.err
}

func TestRunFunction(t *testing.T) {
	errBoom := errors.New("boom")

	fnio := &iov1alpha1.FunctionIO{
		Desired: iov1alpha1.Desired{
			Resources: []v1alpha1.DesiredResource{
				{Name: "cool-resource"},
			},
		},
	}
	fnioyaml, _ := yaml.Marshal(fnio)

	type params struct {
		server fnpbv1alpha1.ContainerizedFunctionRunnerServiceServer
	}

	type args struct {
		ctx  context.Context
		fnio *iov1alpha1.FunctionIO
		fn   *v1.ContainerFunction
	}

	type want struct {
		fnio *iov1alpha1.FunctionIO
		err  error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"RunFunctionError": {
			reason: "We should return an error if we can't make an RPC call to run the function.",
			params: params{
				server: &MockFunctionServer{err: errBoom},
			},
			args: args{
				ctx:  context.Background(),
				fnio: &iov1alpha1.FunctionIO{},
				fn:   &v1.ContainerFunction{},
			},
			want: want{
				err: errors.Wrap(status.Errorf(codes.Unknown, errBoom.Error()), errRunFnContainer),
			},
		},
		"RunFunctionSuccess": {
			reason: "We should return the same FunctionIO our server returned.",
			params: params{
				server: &MockFunctionServer{
					rsp: &fnpbv1alpha1.RunFunctionResponse{
						Output: fnioyaml,
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				fnio: &iov1alpha1.FunctionIO{},
				fn:   &v1.ContainerFunction{},
			},
			want: want{
				fnio: fnio,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			// Listen on a random port.
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}

			wg := &sync.WaitGroup{}
			wg.Add(1)
			go func() {
				s := grpc.NewServer()
				fnpbv1alpha1.RegisterContainerizedFunctionRunnerServiceServer(s, tc.params.server)
				_ = s.Serve(lis)
				wg.Done()
			}()

			// Tell the function to connect to our mock server.
			tc.args.fn.Runner = &v1.ContainerFunctionRunner{
				Endpoint: pointer.String(lis.Addr().String()),
			}

			xfnRunner := &DefaultCompositeFunctionRunner{}

			fnio, err := xfnRunner.RunFunction(tc.args.ctx, tc.args.fnio, tc.args.fn)

			_ = lis.Close() // This should terminate the goroutine above.
			wg.Wait()

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRunFunction(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.fnio, fnio); diff != "" {
				t.Errorf("\n%s\nRunFunction(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestParseDesiredResource(t *testing.T) {

	cd := composed.New()
	cd.SetAPIVersion("a/v1")
	cd.SetKind("Composed")
	cd.SetName("composed")

	desired := iov1alpha1.DesiredResource{
		Name: "cool-resource",
		Resource: runtime.RawExtension{
			Raw: func() []byte {
				j, _ := json.Marshal(cd)
				return j
			}(),
		},
	}

	type args struct {
		dr    iov1alpha1.DesiredResource
		owner resource.Object
	}

	type want struct {
		cd  ComposedResourceState
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnmarshalResourceError": {
			reason: "We should return an error if we can't unmarshal our composed resource.",
			args: args{
				dr: iov1alpha1.DesiredResource{
					Resource: runtime.RawExtension{
						Raw: []byte("}"),
					},
				},
			},
			want: want{
				err: errors.Wrap(json.Unmarshal([]byte("}"), nil), errUnmarshalDesiredCD),
			},
		},
		"SetControllerRefError": {
			reason: "We should return an error if we can't set a controller reference on our composed resource.",
			args: args{
				dr: iov1alpha1.DesiredResource{
					Resource: runtime.RawExtension{
						Raw: func() []byte {
							u := cd.DeepCopy()
							cd := &composed.Unstructured{Unstructured: *u}
							meta.AddOwnerReference(cd, meta.AsController(&xpv1.TypedReference{
								UID:  types.UID("someone-else"),
								Kind: "Composite",
								Name: "someone-else",
							}))
							j, _ := json.Marshal(cd)
							return j
						}(),
					},
				},
				owner: &fake.Composite{},
			},
			want: want{
				err: errors.Wrap(errors.New("composed is already controlled by Composite someone-else (UID someone-else)"), errSetControllerRef),
			},
		},
		"Success": {
			reason: "We should successfully translate a desired resource into composed resource state.",
			args: args{
				dr: desired,
				owner: &fake.Composite{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("owner"),
						Labels: map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "pfx",
							xcrd.LabelKeyClaimName:             "cool-claim",
							xcrd.LabelKeyClaimNamespace:        "default",
						},
					},
				},
			},
			want: want{
				cd: ComposedResourceState{
					ComposedResource: ComposedResource{
						ResourceName: "cool-resource",
					},
					Desired: &desired,
					Resource: func() *composed.Unstructured {
						u := cd.DeepCopy()
						cd := &composed.Unstructured{Unstructured: *u}
						SetCompositionResourceName(cd, "cool-resource")
						cd.SetOwnerReferences([]metav1.OwnerReference{{
							UID:                types.UID("owner"),
							Controller:         pointer.Bool(true),
							BlockOwnerDeletion: pointer.Bool(true),
						}})
						cd.SetLabels(map[string]string{
							xcrd.LabelKeyNamePrefixForComposed: "pfx",
							xcrd.LabelKeyClaimName:             "cool-claim",
							xcrd.LabelKeyClaimNamespace:        "default",
						})
						return cd
					}(),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			cd, err := ParseDesiredResource(tc.args.dr, tc.args.owner)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nParseDesiredResource(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cd, cd); diff != "" {
				t.Errorf("\n%s\nParseDesiredResource(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestImagePullConfig(t *testing.T) {

	always := corev1.PullAlways
	never := corev1.PullNever
	ifNotPresent := corev1.PullIfNotPresent

	cases := map[string]struct {
		reason  string
		fn      *v1.ContainerFunction
		want    *fnpbv1alpha1.ImagePullConfig
		wantErr error
	}{
		"NoImagePullPolicy": {
			reason:  "We should return an empty config if there's no ImagePullPolicy.",
			fn:      &v1.ContainerFunction{},
			want:    &fnpbv1alpha1.ImagePullConfig{},
			wantErr: nil,
		},
		"PullAlways": {
			reason: "We should correctly map PullAlways.",
			fn: &v1.ContainerFunction{
				ImagePullPolicy: &always,
			},
			want: &fnpbv1alpha1.ImagePullConfig{
				PullPolicy: fnpbv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_ALWAYS,
			},
			wantErr: nil,
		},
		"PullNever": {
			reason: "We should correctly map PullNever.",
			fn: &v1.ContainerFunction{
				ImagePullPolicy: &never,
			},
			want: &fnpbv1alpha1.ImagePullConfig{
				PullPolicy: fnpbv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_NEVER,
			},
			wantErr: nil,
		},
		"PullIfNotPresent": {
			reason: "We should correctly map PullIfNotPresent.",
			fn: &v1.ContainerFunction{
				ImagePullPolicy: &ifNotPresent,
			},
			want: &fnpbv1alpha1.ImagePullConfig{
				PullPolicy: fnpbv1alpha1.ImagePullPolicy_IMAGE_PULL_POLICY_IF_NOT_PRESENT,
			},
			wantErr: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ImagePullConfig(tc.fn, nil)

			if diff := cmp.Diff(tc.wantErr, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nImagePullConfig(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nImagePullConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestRunFunctionConfig(t *testing.T) {

	cpu := computeresource.MustParse("1")
	mem := computeresource.MustParse("256Mi")

	isolated := v1.ContainerFunctionNetworkPolicyIsolated
	runner := v1.ContainerFunctionNetworkPolicyRunner

	cases := map[string]struct {
		reason string
		fn     *v1.ContainerFunction
		want   *fnpbv1alpha1.RunFunctionConfig
	}{
		"EmptyConfig": {
			reason: "An empty config should be returned when there is no run-related configuration.",
			fn:     &v1.ContainerFunction{},
			want:   &fnpbv1alpha1.RunFunctionConfig{},
		},
		"Resources": {
			reason: "All resource quantities should be included in the RunFunctionConfig",
			fn: &v1.ContainerFunction{
				Resources: &v1.ContainerFunctionResources{
					Limits: &v1.ContainerFunctionResourceLimits{
						CPU:    &cpu,
						Memory: &mem,
					},
				},
			},
			want: &fnpbv1alpha1.RunFunctionConfig{
				Resources: &fnpbv1alpha1.ResourceConfig{
					Limits: &fnpbv1alpha1.ResourceLimits{
						Cpu:    cpu.String(),
						Memory: mem.String(),
					},
				},
			},
		},
		"IsolatedNetwork": {
			reason: "The isolated network policy should be returned.",
			fn: &v1.ContainerFunction{
				Network: &v1.ContainerFunctionNetwork{
					Policy: &isolated,
				},
			},
			want: &fnpbv1alpha1.RunFunctionConfig{
				Network: &fnpbv1alpha1.NetworkConfig{
					Policy: fnpbv1alpha1.NetworkPolicy_NETWORK_POLICY_ISOLATED,
				},
			},
		},
		"RunnerNetwork": {
			reason: "The runner network policy should be returned.",
			fn: &v1.ContainerFunction{
				Network: &v1.ContainerFunctionNetwork{
					Policy: &runner,
				},
			},
			want: &fnpbv1alpha1.RunFunctionConfig{
				Network: &fnpbv1alpha1.NetworkConfig{
					Policy: fnpbv1alpha1.NetworkPolicy_NETWORK_POLICY_RUNNER,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			got := RunFunctionConfig(tc.fn)

			if diff := cmp.Diff(tc.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("\n%s\nRunFunctionConfig(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeleteComposedResources(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		client client.Writer
	}

	type args struct {
		ctx context.Context
		s   *PTFCompositionState
	}

	type want struct {
		s   *PTFCompositionState
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"HasDesired": {
			reason: "Desired resources should not be deleted.",
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"desired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "desired-resource",
							},
							Desired: &iov1alpha1.DesiredResource{},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"desired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "desired-resource",
							},
							Desired: &iov1alpha1.DesiredResource{},
						},
					},
				},
			},
		},
		"NeverCreated": {
			reason: "Resources that were never created should be deleted from state, but not the API server.",
			params: params{
				client: &test.MockClient{
					// We know Delete wasn't called because it's a nil function
					// and would thus panic if it was.
				},
			},
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"undesired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{},
							Resource:         &fake.Composed{},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{},
				},
			},
		},
		"UncontrolledResource": {
			reason: "Resources the XR doesn't control should be deleted from state, but not the API server.",
			params: params{
				client: &test.MockClient{
					// We know Delete wasn't called because it's a nil function
					// and would thus panic if it was.
				},
			},
			args: args{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{
						"undesired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									// This resource exists in the API server.
									CreationTimestamp: metav1.Now(),
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{},
				},
			},
		},
		"DeleteError": {
			reason: "We should return any error encountered deleting the resource.",
			params: params{
				client: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(errBoom),
				},
			},
			args: args{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{
						"undesired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "undesired-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									// This resource exists in the API server.
									CreationTimestamp: metav1.Now(),

									// This resource is controlled by the XR.
									OwnerReferences: []metav1.OwnerReference{{
										Controller: pointer.Bool(true),
										UID:        "cool-xr",
									}},
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{},
				},
				err: errors.Wrapf(errBoom, errFmtDeleteCD, "undesired-resource", "", ""),
			},
		},
		"Success": {
			reason: "We should successfully delete the resource from the API server and state.",
			params: params{
				client: &test.MockClient{
					MockDelete: test.NewMockDeleteFn(nil),
				},
			},
			args: args{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{
						"undesired-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "undesired-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									// This resource exists in the API server.
									CreationTimestamp: metav1.Now(),

									// This resource is controlled by the XR.
									OwnerReferences: []metav1.OwnerReference{{
										Controller: pointer.Bool(true),
										UID:        "cool-xr",
									}},
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ObjectMeta: metav1.ObjectMeta{
							UID: "cool-xr",
						},
					},
					ComposedResources: ComposedResourceStates{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			d := NewUndesiredComposedResourceDeleter(tc.params.client)
			err := d.DeleteComposedResources(tc.args.ctx, tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDeleteComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.s, tc.args.s); diff != "" {
				t.Errorf("\n%s\nDeleteComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestUpdateResourceRefs(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		s *PTFCompositionState
	}

	type want struct {
		s *PTFCompositionState
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RenderError": {
			reason: "We shouldn't record references to resources that were never created and failed to render.",
			args: args{
				s: &PTFCompositionState{
					Composite: &fake.Composite{},
					ComposedResources: ComposedResourceStates{
						"never-created": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-42",
								},
							},
							TemplateRenderErr: errBoom,
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
							Refs: []corev1.ObjectReference{},
						},
					},
					ComposedResources: ComposedResourceStates{
						"never-created": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-42",
								},
							},
							TemplateRenderErr: errBoom,
						},
					},
				},
			},
		},
		"Success": {
			reason: "We should return a consistently ordered set of references.",
			args: args{
				s: &PTFCompositionState{
					Composite: &fake.Composite{},
					ComposedResources: ComposedResourceStates{
						"never-created-c": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-c-42",
								},
							},
						},
						"never-created-b": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-b-42",
								},
							},
						},
						"never-created-a": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-a-42",
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					Composite: &fake.Composite{
						ComposedResourcesReferencer: fake.ComposedResourcesReferencer{
							Refs: []corev1.ObjectReference{
								{Name: "never-created-a-42"},
								{Name: "never-created-b-42"},
								{Name: "never-created-c-42"},
							},
						},
					},
					ComposedResources: ComposedResourceStates{
						"never-created-c": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-c-42",
								},
							},
						},
						"never-created-b": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-b-42",
								},
							},
						},
						"never-created-a": ComposedResourceState{
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "never-created-a-42",
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			UpdateResourceRefs(tc.args.s)

			// We need to EquateErrors here for the TemplateRenderError in state.
			if diff := cmp.Diff(tc.want.s, tc.args.s, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nUpdateResourceRefs(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestReadinessObserver(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		c ReadinessChecker
	}

	type args struct {
		ctx context.Context
		s   *PTFCompositionState
	}

	type want struct {
		s   *PTFCompositionState
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"IsReadyError": {
			reason: "We should return any error encountered checking readiness.",
			params: params{
				c: ReadinessCheckerFn(func(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error) {
					return false, errBoom
				}),
			},
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-resource-42",
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-resource-42",
								},
							},
						},
					},
				},
				err: errors.Wrapf(errBoom, errFmtReadiness, "cool-resource", "", "cool-resource-42"),
			},
		},
		"Ready": {
			reason: "We should record whether the composed resource is ready.",
			params: params{
				c: ReadinessCheckerFn(func(ctx context.Context, o ConditionedObject, rc ...ReadinessCheck) (ready bool, err error) {
					return true, nil
				}),
			},
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
								Ready:        true,
							},
							Resource: &fake.Composed{},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			o := NewReadinessObserver(tc.params.c)
			err := o.ObserveComposedResources(tc.args.ctx, tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.s, tc.args.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestConnectionDetailsObserver(t *testing.T) {
	errBoom := errors.New("boom")

	type params struct {
		e ConnectionDetailsExtractor
	}

	type args struct {
		ctx context.Context
		s   *PTFCompositionState
	}

	type want struct {
		s   *PTFCompositionState
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ExtractConnectionError": {
			reason: "We should return any error encountered extracting connetion details.",
			params: params{
				e: ConnectionDetailsExtractorFn(func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
					return nil, errBoom
				}),
			},
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-resource-42",
								},
							},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{
								ObjectMeta: metav1.ObjectMeta{
									Name: "cool-resource-42",
								},
							},
						},
					},
				},
				err: errors.Wrapf(errBoom, errFmtExtractConnectionDetails, "cool-resource", "", "cool-resource-42"),
			},
		},
		"Success": {
			reason: "We should record the extracted connection details.",
			params: params{
				e: ConnectionDetailsExtractorFn(func(cd resource.Composed, conn managed.ConnectionDetails, cfg ...ConnectionDetailExtractConfig) (managed.ConnectionDetails, error) {
					return managed.ConnectionDetails{"a": []byte("b")}, nil
				}),
			},
			args: args{
				s: &PTFCompositionState{
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{},
						},
					},
				},
			},
			want: want{
				s: &PTFCompositionState{
					ConnectionDetails: managed.ConnectionDetails{"a": []byte("b")},
					ComposedResources: ComposedResourceStates{
						"cool-resource": ComposedResourceState{
							ComposedResource: ComposedResource{
								ResourceName: "cool-resource",
							},
							Resource: &fake.Composed{},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			o := NewConnectionDetailsObserver(tc.params.e)
			err := o.ObserveComposedResources(tc.args.ctx, tc.args.s)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.s, tc.args.s, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nObserveComposedResources(...): -want, +got:\n%s", tc.reason, diff)
			}

		})
	}
}
