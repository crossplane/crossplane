/*
Copyright 2021 The Crossplane Authors.

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

package initializer

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestDefaultManagedResourceActivationPolicy(t *testing.T) {
	type args struct {
		activations []v1alpha1.ActivationPolicy
		kube        client.Client
	}

	type want struct {
		err   error
		nilFn bool
	}

	cases := map[string]struct {
		args
		want
	}{
		"NoActivation": {
			args: args{},
			want: want{
				nilFn: true,
			},
		},
		"FailedToCreate": {
			args: args{
				activations: []v1alpha1.ActivationPolicy{"*"},
				kube: &test.MockClient{
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						mrap, ok := obj.(*v1alpha1.ManagedResourceActivationPolicy)
						if !ok {
							t.Errorf("Expected ManagedResourceActivationPolicy, got %T", obj)
							return nil
						}
						expectedActivations := []v1alpha1.ActivationPolicy{"*"}
						if diff := cmp.Diff(expectedActivations, mrap.Spec.Activations); diff != "" {
							t.Errorf("Activations mismatch (-want +got):\n%s", diff)
						}
						return errBoom
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, "cannot create ManagedResourceActivationPolicy \"default\""),
			},
		},
		"SuccessCreated": {
			args: args{
				activations: []v1alpha1.ActivationPolicy{"policy1", "policy2", "*.example.com"},
				kube: &test.MockClient{
					MockCreate: func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
						mrap, ok := obj.(*v1alpha1.ManagedResourceActivationPolicy)
						if !ok {
							t.Errorf("Expected ManagedResourceActivationPolicy, got %T", obj)
							return nil
						}
						expectedActivations := []v1alpha1.ActivationPolicy{"policy1", "policy2", "*.example.com"}
						if diff := cmp.Diff(expectedActivations, mrap.Spec.Activations); diff != "" {
							t.Errorf("Activations mismatch (-want +got):\n%s", diff)
						}
						return nil
					},
				},
			},
		},
		"SuccessAlreadyExists": {
			args: args{
				activations: []v1alpha1.ActivationPolicy{"*"},
				kube: &test.MockClient{
					MockCreate: func(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
						return kerrors.NewAlreadyExists(schema.GroupResource{}, "default")
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fn := DefaultManagedResourceActivationPolicy(tc.activations...)
			if fn == nil {
				if !tc.nilFn {
					t.Errorf("\n%s\nUnexpected nil function", name)
				}
				return
			}
			err := fn(context.TODO(), tc.kube)
			if diff := cmp.Diff(tc.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(...): -want err, +got err:\n%s", name, diff)
			}
		})
	}
}
