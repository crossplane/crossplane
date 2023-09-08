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

package providerconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// This can't live in fake, because it would cause an import cycle due to
// GetItems returning managed.ProviderConfigUsage.
type ProviderConfigUsageList struct { //nolint:musttag // This is a fake implementation to be used in unit tests only.
	client.ObjectList
	Items []resource.ProviderConfigUsage
}

func (p *ProviderConfigUsageList) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (p *ProviderConfigUsageList) DeepCopyObject() runtime.Object {
	out := &ProviderConfigUsageList{}
	j, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

func (p *ProviderConfigUsageList) GetItems() []resource.ProviderConfigUsage {
	return p.Items
}

func TestReconciler(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	uid := types.UID("so-unique")
	ctrl := true

	type args struct {
		m  manager.Manager
		of resource.ProviderConfigKinds
	}

	type want struct {
		result reconcile.Result
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetProviderConfigError": {
			reason: "Errors getting a provider config should be returned",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    errors.Wrap(errBoom, errGetPC),
			},
		},
		"ProviderConfigNotFound": {
			reason: "We should return without requeueing if the provider config no longer exists",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{},
				err:    nil,
			},
		},
		"ListProviderConfigUsageError": {
			reason: "We should requeue after a short wait if we encounter an error listing provider config usages",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:  test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"DeleteProviderConfigUsageError": {
			reason: "We should requeue after a short wait if we encounter an error deleting a provider config usage",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*ProviderConfigUsageList)
							l.Items = []resource.ProviderConfigUsage{
								&fake.ProviderConfigUsage{},
							}
							return nil
						}),
						MockDelete: test.NewMockDeleteFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"BlockDeleteWhileInUse": {
			reason: "We should return without requeueing if the provider config is still in use",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)
							pc.SetUID(uid)
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*ProviderConfigUsageList)
							l.Items = []resource.ProviderConfigUsage{
								&fake.ProviderConfigUsage{
									ObjectMeta: metav1.ObjectMeta{
										OwnerReferences: []metav1.OwnerReference{{
											UID:        uid,
											Controller: &ctrl,
										}},
									},
								},
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should requeue after a short wait if we encounter an error while removing our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)
							return nil
						}),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulDelete": {
			reason: "We should return without requeueing when we successfully remove our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							pc := obj.(*fake.ProviderConfig)
							pc.SetDeletionTimestamp(&now)
							return nil
						}),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
		"AddFinalizerError": {
			reason: "We should requeue after a short wait if we encounter an error while adding our finalizer",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    test.NewMockGetFn(nil),
						MockList:   test.NewMockListFn(nil),
						MockUpdate: test.NewMockUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"UpdateStatusError": {
			reason: "We return errors encountered while updating our status",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
				err:    errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"SuccessfulSetUsers": {
			reason: "We should return without requeuing if we successfully update our user count",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.ProviderConfig{}, &ProviderConfigUsageList{}),
				},
				of: resource.ProviderConfigKinds{
					Config:    fake.GVK(&fake.ProviderConfig{}),
					UsageList: fake.GVK(&ProviderConfigUsageList{}),
				},
			},
			want: want{
				result: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.of)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
