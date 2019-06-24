/*
Copyright 2019 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    htcp://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace  = "coolns"
	name       = "cool"
	secretName = "coolsecret"
	uid        = types.UID("definitely-a-uuid")
)

var MockOwnerGVK = schema.GroupVersionKind{
	Group:   "cool",
	Version: "large",
	Kind:    "MockOwner",
}

type MockOwner struct {
	metav1.ObjectMeta
}

func (m *MockOwner) GetWriteConnectionSecretTo() corev1.LocalObjectReference {
	return corev1.LocalObjectReference{Name: secretName}
}

func (m *MockOwner) SetWriteConnectionSecretTo(_ corev1.LocalObjectReference) {}

func TestConnectionSecretFor(t *testing.T) {
	type args struct {
		o    ConnectionSecretOwner
		kind schema.GroupVersionKind
	}

	controller := true

	cases := map[string]struct {
		args args
		want *corev1.Secret
	}{
		"Success": {
			args: args{
				o: &MockOwner{ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					UID:       uid,
				}},
				kind: MockOwnerGVK,
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      secretName,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: MockOwnerGVK.GroupVersion().String(),
						Kind:       MockOwnerGVK.Kind,
						Name:       name,
						UID:        uid,
						Controller: &controller,
					}},
				},
				Data: map[string][]byte{},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConnectionSecretFor(tc.args.o, tc.args.kind)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ConnectionSecretFor(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMustCreateObject(t *testing.T) {
	type args struct {
		kind schema.GroupVersionKind
		oc   runtime.ObjectCreater
	}
	cases := map[string]struct {
		args args
		want runtime.Object
	}{
		"KindRegistered": {
			args: args{
				kind: MockGVK(&MockClaim{}),
				oc:   MockSchemeWith(&MockClaim{}),
			},
			want: &MockClaim{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MustCreateObject(tc.args.kind, tc.args.oc)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MustCreateObject(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIgnore(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		is  ErrorIs
		err error
	}
	cases := map[string]struct {
		args args
		want error
	}{
		"IgnoreError": {
			args: args{
				is:  func(err error) bool { return true },
				err: errBoom,
			},
			want: nil,
		},
		"PropagateError": {
			args: args{
				is:  func(err error) bool { return false },
				err: errBoom,
			},
			want: errBoom,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Ignore(tc.args.is, tc.args.err)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("Ignore(...): -want error, +got error:\n%s", diff)
			}
		})
	}
}

func TestResolveClassClaimValues(t *testing.T) {
	type args struct {
		classValue string
		claimValue string
	}

	type want struct {
		err   error
		value string
	}

	cases := map[string]struct {
		args
		want
	}{
		"ClassValueUnset": {
			args: args{claimValue: "cool"},
			want: want{value: "cool"},
		},
		"ClaimValueUnset": {
			args: args{classValue: "cool"},
			want: want{value: "cool"},
		},
		"IdenticalValues": {
			args: args{classValue: "cool", claimValue: "cool"},
			want: want{value: "cool"},
		},
		"ConflictingValues": {
			args: args{classValue: "lame", claimValue: "cool"},
			want: want{err: errors.New("claim value [cool] does not match class value [lame]")},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveClassClaimValues(tc.args.classValue, tc.args.claimValue)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("ResolveClassClaimValues(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.value, got); diff != "" {
				t.Errorf("ResolveClassClaimValues(...): -want, +got:\n%s", diff)
			}

		})
	}
}

func TestSetBindable(t *testing.T) {
	cases := map[string]struct {
		b    Bindable
		want v1alpha1.BindingPhase
	}{
		"BindableIsUnbindable": {
			b:    &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseUnbindable}},
			want: v1alpha1.BindingPhaseUnbound,
		},
		"BindableIsUnbound": {
			b:    &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseUnbound}},
			want: v1alpha1.BindingPhaseUnbound,
		},
		"BindableIsBound": {
			b:    &MockClaim{MockBindable: MockBindable{Phase: v1alpha1.BindingPhaseBound}},
			want: v1alpha1.BindingPhaseBound,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			SetBindable(tc.b)
			if diff := cmp.Diff(tc.want, tc.b.GetBindingPhase()); diff != "" {
				t.Errorf("SetBindable(...): -got, +want:\n%s", diff)
			}
		})
	}
}
