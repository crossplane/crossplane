/*
Copyright 2025 The Crossplane Authors.

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

package usage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	legacy "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	"github.com/crossplane/crossplane/apis/protection/v1beta1"
	"github.com/crossplane/crossplane/internal/protection"
)

type IndexFieldFn func(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error

func (fn IndexFieldFn) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return fn(ctx, obj, field, extractValue)
}

func TestFindUsageOf(t *testing.T) {
	type params struct {
		r  client.Reader
		fi client.FieldIndexer
	}
	type args struct {
		ctx context.Context
		o   Object
	}
	type want struct {
		u   []protection.Usage
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"ListUsageError": {
			reason: "We should return an error if we can't list protection.crossplane.io Usages.",
			params: params{
				r: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						if _, ok := obj.(*v1beta1.UsageList); ok {
							return errors.New("boom")
						}
						return nil
					}),
				},
				fi: IndexFieldFn(func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
					return nil
				}),
			},
			args: args{
				o: &unstructured.Unstructured{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListClusterUsageError": {
			reason: "We should return an error if we can't list protection.crossplane.io ClusterUsages.",
			params: params{
				r: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						if _, ok := obj.(*v1beta1.ClusterUsageList); ok {
							return errors.New("boom")
						}
						return nil
					}),
				},
				fi: IndexFieldFn(func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
					return nil
				}),
			},
			args: args{
				o: &unstructured.Unstructured{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListLegacyUsageError": {
			reason: "We should return an error if we can't list apiextensions.crossplane.io Usages.",
			params: params{
				r: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						if _, ok := obj.(*legacy.UsageList); ok { //nolint:staticcheck // Deprecated but still supported.
							return errors.New("boom")
						}
						return nil
					}),
				},
				fi: IndexFieldFn(func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
					return nil
				}),
			},
			args: args{
				o: &unstructured.Unstructured{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We should return any type of usages we found.",
			params: params{
				r: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						switch l := obj.(type) {
						case *v1beta1.UsageList:
							l.Items = []v1beta1.Usage{{ObjectMeta: metav1.ObjectMeta{Name: "usage"}}}
						case *v1beta1.ClusterUsageList:
							l.Items = []v1beta1.ClusterUsage{{ObjectMeta: metav1.ObjectMeta{Name: "cluster-usage"}}}
						case *legacy.UsageList: //nolint:staticcheck // Deprecated but still supported.
							l.Items = []legacy.Usage{{ObjectMeta: metav1.ObjectMeta{Name: "legacy-usage"}}} //nolint:staticcheck // See above.
						default:
							return errors.New("shouldn't happen")
						}
						return nil
					}),
				},
				fi: IndexFieldFn(func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
					return nil
				}),
			},
			args: args{
				o: &unstructured.Unstructured{},
			},
			want: want{
				u: []protection.Usage{
					&v1beta1.Usage{ObjectMeta: metav1.ObjectMeta{Name: "usage"}},
					&v1beta1.ClusterUsage{ObjectMeta: metav1.ObjectMeta{Name: "cluster-usage"}},
					&legacy.Usage{ObjectMeta: metav1.ObjectMeta{Name: "legacy-usage"}}, //nolint:staticcheck // Deprecated but still supported.
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := NewFinder(tc.params.r, tc.params.fi)
			if err != nil {
				t.Fatal(err)
			}

			u, err := f.FindUsageOf(tc.args.ctx, tc.args.o)
			if diff := cmp.Diff(tc.want.u, u); diff != "" {
				t.Errorf("%s\nf.FindUsageOf(...): -want u, +got u:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.FindUsageOf(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
