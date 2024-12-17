/*
Copyright 2024 The Crossplane Authors.

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

package job

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	crossapiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	kfake "k8s.io/client-go/kubernetes/fake"
	restfake "k8s.io/client-go/rest/fake"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type FakeExtendedCoreV1 struct {
	typedcorev1.CoreV1Interface
}

func (c *FakeExtendedCoreV1) RESTClient() rest.Interface {
	return &restfake.RESTClient{}
}

type FakeExtendedClientset struct {
	*kfake.Clientset
}

func (f *FakeExtendedClientset) CoreV1() typedcorev1.CoreV1Interface {
	return &FakeExtendedCoreV1{f.Clientset.CoreV1()}
}

func NewFakeClientset(objs ...runtime.Object) kubernetes.Interface {
	return &FakeExtendedClientset{kfake.NewClientset(objs...)}
}

func TestCompositionRevisionCleanupJob(t *testing.T) {

	comp := &crossapiextensionsv1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cool-composition",
		},
	}

	rev2 := &crossapiextensionsv1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				crossapiextensionsv1.LabelCompositionName: comp.GetName(),
			},
			Name: comp.GetName() + "-2",
		},
		Spec: crossapiextensionsv1.CompositionRevisionSpec{Revision: 2},
	}

	rev1 := &crossapiextensionsv1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				crossapiextensionsv1.LabelCompositionName: comp.GetName(),
			},
			Name: comp.GetName() + "-1",
		},
		Spec: crossapiextensionsv1.CompositionRevisionSpec{Revision: 1},
	}

	objects := []runtime.Object{
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "ns1"},
		},
	}

	type args struct {
		log              logging.Logger
		k8sClient        kubernetes.Interface
		crossplaneClient client.Client
		Ctx              context.Context
		ItemsToKeep      map[string]struct{}
		KeepTopNItems    int
	}
	type want struct {
		processedCount int
		err            error
	}
	cases := map[string]struct {
		args
		want
	}{
		"SuccessWithNoKeepingItemsAndKeepOneRevision": {
			args: args{
				log:       logging.NewNopLogger(),
				k8sClient: NewFakeClientset(objects...),
				crossplaneClient: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*crossapiextensionsv1.CompositionRevisionList) = crossapiextensionsv1.CompositionRevisionList{
							Items: []crossapiextensionsv1.CompositionRevision{
								*rev2,
								*rev1,
							},
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
				},

				Ctx:           context.Background(),
				ItemsToKeep:   map[string]struct{}{},
				KeepTopNItems: 1,
			},
			want: want{
				processedCount: 1,
				err:            nil,
			},
		},
		"SuccessWithKeepingItemsAndKeepOneRevision": {
			args: args{
				log:       logging.NewNopLogger(),
				k8sClient: NewFakeClientset(objects...),
				crossplaneClient: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*crossapiextensionsv1.CompositionRevisionList) = crossapiextensionsv1.CompositionRevisionList{
							Items: []crossapiextensionsv1.CompositionRevision{
								*rev2,
								*rev1,
							},
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(nil),
				},

				Ctx: context.Background(),
				ItemsToKeep: map[string]struct{}{
					comp.GetName(): struct{}{},
				},
				KeepTopNItems: 1,
			},
			want: want{
				processedCount: 0,
				err:            nil,
			},
		},
		"SuccessWithKeepingItemsAndKeepThreeRevisions": {
			args: args{
				log:       logging.NewNopLogger(),
				k8sClient: NewFakeClientset(objects...),
				crossplaneClient: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*crossapiextensionsv1.CompositionRevisionList) = crossapiextensionsv1.CompositionRevisionList{
							Items: []crossapiextensionsv1.CompositionRevision{
								*rev2,
								*rev1,
							},
						}
						return nil
					}),
				},

				Ctx:           context.Background(),
				ItemsToKeep:   map[string]struct{}{},
				KeepTopNItems: 3,
			},
			want: want{
				processedCount: 0,
				err:            nil,
			},
		},
		"FailWithErrorOnDelete": {
			args: args{
				log:       logging.NewNopLogger(),
				k8sClient: NewFakeClientset(objects...),
				crossplaneClient: &test.MockClient{
					MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
						*obj.(*crossapiextensionsv1.CompositionRevisionList) = crossapiextensionsv1.CompositionRevisionList{
							Items: []crossapiextensionsv1.CompositionRevision{
								*rev2,
								*rev1,
							},
						}
						return nil
					}),
					MockDelete: test.NewMockDeleteFn(errors.New("error")),
				},

				Ctx:           context.Background(),
				ItemsToKeep:   map[string]struct{}{},
				KeepTopNItems: 1,
			},
			want: want{
				processedCount: 0,
				err:            errors.Wrap(errors.New("error"), errDeleteCompositionRev),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			job := NewCompositionRevisionCleanupJob(tc.args.log, tc.args.k8sClient, tc.args.crossplaneClient)
			processedCount, err := job.Run(tc.args.Ctx, tc.args.ItemsToKeep, tc.args.KeepTopNItems)
			if diff := cmp.Diff(tc.want.processedCount, processedCount, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
