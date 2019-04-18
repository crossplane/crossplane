/*
Copyright 2018 The Crossplane Authors.

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

package util

import (
	"context"
	"testing"

	"github.com/go-test/deep"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "coolNamespace"
	name      = "coolService"
	uid       = types.UID("definitely-a-uuid")
)

var (
	ctx       = context.Background()
	errorBoom = errors.New("boom")
	meta      = metav1.ObjectMeta{Namespace: namespace, Name: name, UID: uid}
	service   = &corev1.Service{
		ObjectMeta: meta,
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
)

type mockObjectKind struct {
	schema.ObjectKind
}

type weirdObject struct{}

func (o *weirdObject) GetObjectKind() schema.ObjectKind {
	return mockObjectKind{}
}

func (o *weirdObject) DeepCopyObject() runtime.Object {
	return o
}

func TestCreateOrUpdate(t *testing.T) {
	cases := []struct {
		name    string
		kube    client.Client
		obj     runtime.Object
		f       MutateFn
		wantErr error
	}{
		{
			name: "SuccessfulCreate",
			kube: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(nil),
			},
			obj: service,
			f:   func() error { return nil },
		},
		{
			name: "SuccessfulUpdate",
			kube: &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
					*obj.(*corev1.Service) = *service
					return nil
				},
				MockUpdate: test.NewMockUpdateFn(nil),
			},
			obj: service,
			f: func() error {
				service.Spec.Type = corev1.ServiceTypeClusterIP
				return nil
			},
		},
		{
			name: "NoopUpdate",
			kube: &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
					*obj.(*corev1.Service) = *service
					return nil
				},
			},
			obj: service,
			f:   func() error { return nil },
		},
		{
			name:    "MissingObjectMeta",
			kube:    &test.MockClient{},
			obj:     &weirdObject{},
			wantErr: errors.WithStack(errors.New("cannot get object key: object does not implement the Object interfaces")),
		},
		{
			name: "FailedGet",
			kube: &test.MockClient{
				MockGet: test.NewMockGetFn(errorBoom),
			},
			obj:     service,
			wantErr: errors.Wrap(errorBoom, "could not get object"),
		},
		{
			name: "FailedMutateForCreate",
			kube: &test.MockClient{
				MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
			},
			obj:     service,
			f:       func() error { return errorBoom },
			wantErr: errors.Wrap(errorBoom, "could not mutate object for creation"),
		},
		{
			name: "FailedCreate",
			kube: &test.MockClient{
				MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				MockCreate: test.NewMockCreateFn(errorBoom),
			},
			obj:     service,
			f:       func() error { return nil },
			wantErr: errors.Wrap(errorBoom, "could not create object"),
		},
		{
			name: "FailedMutateForUpdate",
			kube: &test.MockClient{
				MockGet: func(_ context.Context, _ client.ObjectKey, obj runtime.Object) error {
					*obj.(*corev1.Service) = *service
					return nil
				},
			},
			obj:     service,
			f:       func() error { return errorBoom },
			wantErr: errors.Wrap(errorBoom, "could not mutate object for update"),
		},
		{
			name: "FailedUpdate",
			kube: &test.MockClient{
				MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
					*obj.(*corev1.Service) = *service
					return nil
				},
				MockUpdate: test.NewMockUpdateFn(errorBoom),
			},
			obj: service,
			f: func() error {
				service.Spec.Type = corev1.ServiceTypeExternalName
				return nil
			},
			wantErr: errors.Wrap(errorBoom, "could not update object"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr := CreateOrUpdate(ctx, tc.kube, tc.obj, tc.f)

			if diff := deep.Equal(tc.wantErr, gotErr); diff != nil {
				t.Errorf("tc.rec.Reconcile(...): want error != got error:\n%s", diff)
			}
		})
	}
}
