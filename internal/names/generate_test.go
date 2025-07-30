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

package names

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/smithy-go/ptr"
	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/internal/xcrd"
)

func TestGenerateName(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		ctx context.Context
		cd  resource.Composed
	}

	type want struct {
		cd  resource.Composed
		err error
	}

	cases := map[string]struct {
		reason string
		client client.Client
		args
		want
	}{
		"SkipGenerateNamedResources": {
			reason: "We should not try naming a resource that already have a name",
			// We must be returning early, or else we'd hit this error.
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					Name: "already-has-a-cool-name",
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					Name: "already-has-a-cool-name",
				}},
				err: nil,
			},
		},
		"SkipGenerateNameForResourcesWithoutGenerateName": {
			reason: "We should not try to name resources that don't have a generate name (though that should never happen)",
			// We must be returning early, or else we'd hit this error.
			client: &test.MockClient{MockCreate: test.NewMockCreateFn(errBoom)},
			args: args{
				cd: &fake.Composed{}, // Conspicously missing a generate name.
			},
			want: want{
				cd:  &fake.Composed{},
				err: nil,
			},
		},
		"NameGeneratorClientError": {
			reason: "Client error finding a free name for a composed resource",
			client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
				err: errBoom,
			},
		},
		"Success": {
			reason: "Name is found on first try",
			client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "CoolResource"}, "cool-resource-42"))},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					Name:         "cool-resource-42",
				}},
			},
		},
		"SuccessMissingOwner": {
			reason: "If no owner, use the random name generator",
			client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "CoolResource"}, "cool-resource-42"))},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "pipeline-name-of-cool-resource",
					},
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					Name:         "cool-resource-42",
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "pipeline-name-of-cool-resource",
					},
				}},
			},
		},
		"SuccessAfterConflict": {
			reason: "Name is found on second try",
			client: &test.MockClient{MockGet: func(_ context.Context, key client.ObjectKey, _ client.Object) error {
				if key.Name == "cool-resource-42" {
					return nil
				}
				return kerrors.NewNotFound(schema.GroupResource{Resource: "CoolResource"}, key.Name)
			}},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					Name:         "cool-resource-43",
				}},
			},
		},
		"AlwaysConflict": {
			reason: "Name cannot be found",
			client: &test.MockClient{MockGet: test.NewMockGetFn(nil)},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
				}},
				err: errors.New(errGenerateName),
			},
		},
		"SuccessCompositeTruncated": {
			reason: "Is annotated and owned should use ChildName and not be random, but if all this is too long, it will be shortened",
			client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "CoolResource"}, "cool-resource-42"))},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-with-a-really-long-name-that-can-not-fit-all-in-one-place-",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "Foo/v1",
						Kind:       "Bar",
						Name:       "parent",
						UID:        "75e4a668-035f-4ce8-8c45-f4d3ac850155",
						Controller: ptr.Bool(true),
					}},
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "pipeline-name-of-cool-resource",
					},
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-with-a-really-long-name-that-can-not-fit-all-in-one-place-",
					Name:         "cool-resource-with-a-really-lonab455e7d35d099adea15e918d64db893",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "Foo/v1",
						Kind:       "Bar",
						Name:       "parent",
						UID:        "75e4a668-035f-4ce8-8c45-f4d3ac850155",
						Controller: ptr.Bool(true),
					}},
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "pipeline-name-of-cool-resource",
					},
				}},
			},
		},
		"SuccessComposite": {
			reason: "Is annotated and owned should use ChildName and not be random",
			client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{Resource: "CoolResource"}, "cool-resource-42"))},
			args: args{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "Foo/v1",
						Kind:       "Bar",
						Name:       "parent",
						UID:        "75e4a668-035f-4ce8-8c45-f4d3ac850155",
						Controller: ptr.Bool(true),
					}},
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "kid1",
					},
				}},
			},
			want: want{
				cd: &fake.Composed{ObjectMeta: metav1.ObjectMeta{
					GenerateName: "cool-resource-",
					Name:         "cool-resource-f4d3ac850155-kid1",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "Foo/v1",
						Kind:       "Bar",
						Name:       "parent",
						UID:        "75e4a668-035f-4ce8-8c45-f4d3ac850155",
						Controller: ptr.Bool(true),
					}},
					Annotations: map[string]string{
						xcrd.AnnotationKeyCompositionResourceName: "kid1",
					},
				}},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &nameGenerator{reader: tc.client, namer: &mockNameGenerator{last: 41}}

			err := r.GenerateName(tc.ctx, tc.args.cd)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nDryRunRender(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.cd, tc.args.cd); diff != "" {
				t.Errorf("\n%s\nDryRunRender(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type mockNameGenerator struct {
	last int
}

func (m *mockNameGenerator) GenerateName(prefix string) string {
	m.last++
	return prefix + strconv.Itoa(m.last)
}
