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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
