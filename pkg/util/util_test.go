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
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-test/deep"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/test"
)

// TestApplySecretError applying a secret and return error
func TestApplySecretError(t *testing.T) {
	g := NewGomegaWithT(t)
	mk := fake.NewSimpleClientset()

	mk.PrependReactor("get", "secrets", func(action ting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("test-error")
	})
	ex, err := ApplySecret(mk, &corev1.Secret{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("test-error"))
	g.Expect(ex).To(BeNil())
	a := mk.Actions()
	g.Expect(len(a)).To(Equal(1))
	g.Expect(a[0].GetVerb()).To(Equal("get"))
}

// TestApplySecretCreate applying a secret that does not exist - expected action: create
func TestApplySecretCreate(t *testing.T) {
	g := NewGomegaWithT(t)
	mk := fake.NewSimpleClientset()

	cs := &corev1.Secret{}
	ex, err := ApplySecret(mk, cs)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ex).NotTo(BeNil())

	a := mk.Actions()
	g.Expect(len(a)).To(Equal(2))
	g.Expect(a[0].GetVerb()).To(Equal("get"))
	g.Expect(a[1].GetVerb()).To(Equal("create"))

	mk.ClearActions()
}

// TestApplySecretUpdate applying a secret that already exists - expected action: update
func TestApplySecretUpdate(t *testing.T) {
	g := NewGomegaWithT(t)

	cs := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
	}

	mk := fake.NewSimpleClientset(cs)

	ex, err := ApplySecret(mk, cs)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ex).NotTo(BeNil())

	a := mk.Actions()
	g.Expect(len(a)).To(Equal(2))
	g.Expect(a[0].GetVerb()).To(Equal("get"))
	g.Expect(a[1].GetVerb()).To(Equal("update"))
}

func TestSecretData(t *testing.T) {
	g := NewGomegaWithT(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "bar",
		},
		Data: map[string][]byte{
			"test-key": []byte("test-data"),
		},
	}

	kube := fake.NewSimpleClientset(secret)

	// test data key is found
	key := corev1.SecretKeySelector{
		Key: "test-key",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
	}
	data, err := SecretData(kube, secret.Namespace, key)
	g.Expect(data).To(Equal(secret.Data["test-key"]))
	g.Expect(err).NotTo(HaveOccurred())

	// test data key is not found
	key = corev1.SecretKeySelector{
		Key: "test-key-bad",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
	}
	data, err = SecretData(kube, secret.Namespace, key)
	g.Expect(data).To(BeNil())
	g.Expect(err).To(HaveOccurred())

	// test secret is not found
	key = corev1.SecretKeySelector{
		Key: "test-key",
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "wrong-secret-name",
		},
	}
	data, err = SecretData(kube, secret.Namespace, key)
	g.Expect(data).To(BeNil())
	g.Expect(err).To(HaveOccurred())
}

func TestObjectReference(t *testing.T) {
	g := NewGomegaWithT(t)
	api := "test-api"
	kind := "test-kind"
	om := metav1.ObjectMeta{
		Namespace:       "test-namespace",
		Name:            "test-name",
		ResourceVersion: "test-resource-version",
		UID:             "test-uid",
	}

	ex := &corev1.ObjectReference{
		APIVersion:      api,
		Kind:            kind,
		Namespace:       om.Namespace,
		Name:            om.Name,
		ResourceVersion: om.ResourceVersion,
		UID:             om.UID,
	}

	g.Expect(ObjectReference(om, api, kind)).To(Equal(ex))
}

func TestIfEmptyString(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(IfEmptyString("", "foo")).To(Equal("foo"))
	g.Expect(IfEmptyString("foo", "bar")).To(Equal("foo"))
	g.Expect(IfEmptyString("foo", "foo")).To(Equal("foo"))
	g.Expect(IfEmptyString("", "")).To(BeEmpty())
}

func TestGenerateName(t *testing.T) {
	g := NewGomegaWithT(t)

	name := GenerateName("foo")
	g.Expect(name).Should(MatchRegexp("foo-[a-zA-z0-9]{5}"))

	// 247 chars, the max allowed (should not be truncated)
	name = GenerateName("CnYC4iprdKJhGNWmG4mAjX4BgiLAzQx1p6CbZVA0mqtVN81FOX0UFkf7IqEDDio24C2nOuqiXcIZziBUJEoynoihLiGS68ZxnQzro3oHF7XNWFwWZBTf5ij52pg5F7qjcsnvZmMC4Qui4c5j8m60G2F6m9MZk6EYw68mXj5PbiB93PD9bnJYdWgkLV3MFy4LJYUM3AbpiLvjVDZRrjoS2s3mLKB3mOIM8pIY0qPI5CqknsYsWWQck9k")
	g.Expect(name).Should(MatchRegexp("CnYC4iprdKJhGNWmG4mAjX4BgiLAzQx1p6CbZVA0mqtVN81FOX0UFkf7IqEDDio24C2nOuqiXcIZziBUJEoynoihLiGS68ZxnQzro3oHF7XNWFwWZBTf5ij52pg5F7qjcsnvZmMC4Qui4c5j8m60G2F6m9MZk6EYw68mXj5PbiB93PD9bnJYdWgkLV3MFy4LJYUM3AbpiLvjVDZRrjoS2s3mLKB3mOIM8pIY0qPI5CqknsYsWWQck9k-[a-zA-z0-9]{5}"))

	// 248 chars, 1 over the max allowed (should get its last char truncated)
	name = GenerateName("CnYC4iprdKJhGNWmG4mAjX4BgiLAzQx1p6CbZVA0mqtVN81FOX0UFkf7IqEDDio24C2nOuqiXcIZziBUJEoynoihLiGS68ZxnQzro3oHF7XNWFwWZBTf5ij52pg5F7qjcsnvZmMC4Qui4c5j8m60G2F6m9MZk6EYw68mXj5PbiB93PD9bnJYdWgkLV3MFy4LJYUM3AbpiLvjVDZRrjoS2s3mLKB3mOIM8pIY0qPI5CqknsYsWWQck9kZ")
	g.Expect(name).Should(MatchRegexp("CnYC4iprdKJhGNWmG4mAjX4BgiLAzQx1p6CbZVA0mqtVN81FOX0UFkf7IqEDDio24C2nOuqiXcIZziBUJEoynoihLiGS68ZxnQzro3oHF7XNWFwWZBTf5ij52pg5F7qjcsnvZmMC4Qui4c5j8m60G2F6m9MZk6EYw68mXj5PbiB93PD9bnJYdWgkLV3MFy4LJYUM3AbpiLvjVDZRrjoS2s3mLKB3mOIM8pIY0qPI5CqknsYsWWQck9k-[a-zA-z0-9]{5}"))
}

func TestApply(t *testing.T) {
	ctx := context.TODO()
	testError := errors.New("test-error")
	type args struct {
		kube client.Client
		o    runtime.Object
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "create failed: other",
			args: args{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return testError
					},
				},
			},
			want: testError,
		},
		{
			name: "create failed: already exists",
			args: args{
				kube: &test.MockClient{
					MockCreate: func(ctx context.Context, obj runtime.Object) error {
						return kerrors.NewAlreadyExists(schema.GroupResource{}, "foo")
					},
					MockUpdate: func(ctx context.Context, obj runtime.Object) error {
						return nil
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Apply(ctx, tt.args.kube, tt.args.o)
			if diff := deep.Equal(err, tt.want); diff != nil {
				t.Errorf("Apply() error = %v, want %v\n%s", err, tt.want, diff)
			}
		})
	}
}
