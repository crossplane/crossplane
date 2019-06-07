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

package kubernetes

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/compute/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func TestGKEClusterHandler_Find(t *testing.T) {
	type args struct {
		name types.NamespacedName
		c    client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Failed",
			args: args{
				name: types.NamespacedName{Namespace: "foo", Name: "bar"},
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return errors.New("test-get-error")
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.New("test-get-error"),
					"failed to retrieve %s: foo/bar", v1alpha1.GKEClusterKind),
			},
		},
		{
			name: "Success",
			args: args{
				name: types.NamespacedName{Namespace: "foo", Name: "bar"},
				c:    test.NewMockClient(),
			},
			want: want{
				res: &v1alpha1.GKECluster{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &GKEClusterHandler{}
			got, err := r.Find(tt.args.name, tt.args.c)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("GKEClusterHandler.Find() error = %v, want.err %v\n%s", err, tt.want.err, diff)
			}
			if diff := cmp.Diff(got, tt.want.res); diff != "" {
				t.Errorf("GKEClusterHandler.Find() = %v, want.res %v\n%s", got, tt.want.res, diff)
			}
		})
	}
}

func TestGKEClusterHandler_Provision(t *testing.T) {
	class := &corev1alpha1.ResourceClass{}
	claim := &computev1alpha1.KubernetesCluster{
		ObjectMeta: v1.ObjectMeta{
			UID: "test-claim-uid",
		},
	}
	createError := errors.New("test-cluster-create-error")
	type args struct {
		class *corev1alpha1.ResourceClass
		claim corev1alpha1.ResourceClaim
		c     client.Client
	}
	type want struct {
		err error
		res corev1alpha1.Resource
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "Success",
			args: args{
				class: class,
				claim: claim,
				c:     test.NewMockClient(),
			},
			want: want{
				res: &v1alpha1.GKECluster{
					ObjectMeta: v1.ObjectMeta{
						Labels:          map[string]string{labelProviderKey: labelProviderGCP},
						Namespace:       class.Namespace,
						Name:            "gke-test-claim-uid",
						OwnerReferences: []v1.OwnerReference{meta.AsOwner(meta.ReferenceTo(claim))},
					},
					Spec: v1alpha1.GKEClusterSpec{
						NumNodes: 1,
						Labels:   map[string]string{},
						Scopes:   []string{},
						ClassRef: meta.ReferenceTo(class),
						ClaimRef: meta.ReferenceTo(claim),
					},
				},
			},
		},
		{
			name: "Failure",
			args: args{
				class: class,
				claim: claim,
				c: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return createError
					},
				},
			},
			want: want{
				err: errors.Wrapf(createError,
					"failed to create cluster %s/%s", class.Namespace, "gke-"+claim.UID),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &GKEClusterHandler{}
			got, err := r.Provision(tt.args.class, tt.args.claim, tt.args.c)
			if diff := cmp.Diff(err, tt.want.err, test.EquateErrors()); diff != "" {
				t.Errorf("GKEClusterHandler.Provision() error = %v, want.err %v\n%s", err, tt.want.err, diff)
				return
			}
			if diff := cmp.Diff(tt.want.res, got); diff != "" {
				t.Errorf("GKEClusterHandler.Provision() -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGKEClusterHandler_SetBindStatus(t *testing.T) {
	name := types.NamespacedName{Namespace: "foo", Name: "bar"}

	getError := errors.New("test-get-error")
	getErrorNotFound := kerrors.NewNotFound(schema.GroupResource{}, name.String())
	updateError := errors.New("test-update-error")

	type args struct {
		name  types.NamespacedName
		c     client.Client
		bound bool
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Failure",
			args: args{
				name: name,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return getError
					},
				},
			},
			want: errors.Wrapf(getError, "failed to retrieve cluster %s", name),
		},
		{
			name: "FailureNotFoundBound",
			args: args{
				name: name,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return getErrorNotFound
					},
				},
				bound: true,
			},
			want: errors.Wrapf(getErrorNotFound, "failed to retrieve cluster %s", name),
		},
		{
			name: "FailureNotFoundNotBound",
			args: args{
				name: name,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return getErrorNotFound
					},
				},
			},
		},
		{
			name: "FailedToUpdate",
			args: args{
				name: name,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return updateError
					},
				},
			},
			want: errors.Wrapf(updateError, "failed to update cluster %s", name),
		},
		{
			name: "SuccessfulSetBound",
			args: args{
				name: name,
				c: &test.MockClient{
					MockGet: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
						return nil
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						cls, ok := obj.(*v1alpha1.GKECluster)
						if !ok {
							t.Errorf("GKEClusterHandler.SetBindStatus() unexpected object type: %T", obj)
						}
						if !cls.Status.IsBound() {
							t.Errorf("GKEClusterHandler.SetBindStatus() expected to be bound")
						}
						return nil
					},
				},
				bound: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := GKEClusterHandler{}
			err := r.SetBindStatus(tt.args.name, tt.args.c, tt.args.bound)
			if diff := cmp.Diff(err, tt.want, test.EquateErrors()); diff != "" {
				t.Errorf("GKEClusterHandler.SetBindStatus() error = %v, want %v\n%s", err, tt.want, diff)
			}
		})
	}
}
