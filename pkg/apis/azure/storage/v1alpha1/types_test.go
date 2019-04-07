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

package v1alpha1

import (
	"log"
	"testing"

	"github.com/go-test/deep"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/test"
)

const (
	namespace = "default"
	name      = "test-instance"
)

var (
	cfg *rest.Config
	c   client.Client
)

func TestMain(m *testing.M) {
	err := SchemeBuilder.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Fatal(err)
	}

	t := test.NewEnv(namespace, test.CRDs())
	cfg = t.Start()

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	t.StopAndExit(m.Run())
}

func TestAzureStorageAccount(t *testing.T) {
	key := types.NamespacedName{Name: name, Namespace: namespace}
	created := &Account{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: AccountSpec{
			ResourceGroupName:  "test-group",
			StorageAccountName: "test-name",
			StorageAccountSpec: &StorageAccountSpec{},
		},
	}
	g := gomega.NewGomegaWithT(t)

	// Test Create
	fetched := &Account{}
	g.Expect(c.Create(context.TODO(), created)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(created))

	// Test Updating the Labels
	updated := fetched.DeepCopy()
	updated.Labels = map[string]string{"hello": "world"}
	g.Expect(c.Update(context.TODO(), updated)).NotTo(gomega.HaveOccurred())

	g.Expect(c.Get(context.TODO(), key, fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(fetched).To(gomega.Equal(updated))

	// Test Delete
	g.Expect(c.Delete(context.TODO(), fetched)).NotTo(gomega.HaveOccurred())
	g.Expect(c.Get(context.TODO(), key, fetched)).To(gomega.HaveOccurred())
}

func TestParseAccountSpec(t *testing.T) {
	tests := []struct {
		name string
		args map[string]string
		want *AccountSpec
	}{
		{
			name: "parse",
			args: map[string]string{
				"storageAccountName": "test-account-name",
				"storageAccountSpec": storageAccountSpecString,
			},
			want: &AccountSpec{
				ReclaimPolicy:      v1alpha1.ReclaimRetain,
				StorageAccountName: "test-account-name",
				StorageAccountSpec: storageAccountSpec,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAccountSpec(tt.args)
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("ParseAccountSpec() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAccount_ConnectionSecretName(t *testing.T) {
	tests := []struct {
		name   string
		bucket Account
		want   string
	}{
		{"default", Account{}, ""},
		{"named", Account{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}, "foo"},
		{"override",
			Account{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec:       AccountSpec{ConnectionSecretNameOverride: "bar"}}, "bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.ConnectionSecretName(); got != tt.want {
				t.Errorf("Bucket.ConnectionSecretName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_ConnectionSecret(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		want    *corev1.Secret
	}{
		{
			name: "test",
			account: Account{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			},
			want: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: APIVersion,
							Kind:       AccountKind,
							Name:       name,
						},
					},
				},
				Data: map[string][]byte{},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.account.ConnectionSecret()
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Bucket.ConnectionSecret() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAccount_ObjectReference(t *testing.T) {
	tests := []struct {
		name   string
		bucket Account
		want   *corev1.ObjectReference
	}{
		{"test", Account{}, &corev1.ObjectReference{APIVersion: APIVersion, Kind: AccountKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.ObjectReference()
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Account.ObjectReference() = %v, want %v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAccount_OwnerReference(t *testing.T) {
	tests := []struct {
		name   string
		bucket Account
		want   metav1.OwnerReference
	}{
		{"test", Account{}, metav1.OwnerReference{APIVersion: APIVersion, Kind: AccountKind}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bucket.OwnerReference()
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("Account.OwnerReference() = \n%+v, want \n%+v\n%s", got, tt.want, diff)
			}
		})
	}
}

func TestAccount_IsAvailable(t *testing.T) {
	b := Account{}

	bReady := b
	bReady.Status.SetReady()

	bReadyAndFailed := bReady
	bReadyAndFailed.Status.SetFailed("", "")

	bNotReadyAndFailed := bReadyAndFailed
	bNotReadyAndFailed.Status.UnsetCondition(v1alpha1.Ready)

	tests := []struct {
		name   string
		bucket Account
		want   bool
	}{
		{"no conditions", b, false},
		{"running active", bReady, true},
		{"running and failed active", bReadyAndFailed, true},
		{"not running and failed active", bNotReadyAndFailed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bucket.IsAvailable(); got != tt.want {
				t.Errorf("Account.IsAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_IsBound(t *testing.T) {
	tests := []struct {
		name  string
		phase v1alpha1.BindingState
		want  bool
	}{
		{"bound", v1alpha1.BindingStateBound, true},
		{"not-bound", v1alpha1.BindingStateUnbound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := Account{
				Status: AccountStatus{
					BindingStatusPhase: v1alpha1.BindingStatusPhase{
						Phase: tt.phase,
					},
				},
			}
			if got := b.IsBound(); got != tt.want {
				t.Errorf("Account.IsBound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_SetBound(t *testing.T) {
	tests := []struct {
		name  string
		state bool
		want  v1alpha1.BindingState
	}{
		{"not-bound", false, v1alpha1.BindingStateUnbound},
		{"bound", true, v1alpha1.BindingStateBound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Account{}
			c.SetBound(tt.state)
			if c.Status.Phase != tt.want {
				t.Errorf("Account.SetBound(%v) = %v, want %v", tt.state, c.Status.Phase, tt.want)
			}
		})
	}
}

func TestContainer_GetContainerName(t *testing.T) {
	om := metav1.ObjectMeta{
		Namespace: "foo",
		Name:      "bar",
		UID:       "test-uid",
	}
	type fields struct {
		ObjectMeta metav1.ObjectMeta
		Spec       ContainerSpec
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "no name format",
			fields: fields{
				ObjectMeta: om,
				Spec:       ContainerSpec{},
			},
			want: "test-uid",
		},
		{
			name: "format string",
			fields: fields{
				ObjectMeta: om,
				Spec: ContainerSpec{
					NameFormat: "foo-%s",
				},
			},
			want: "foo-test-uid",
		},
		{
			name: "constant string",
			fields: fields{
				ObjectMeta: om,
				Spec: ContainerSpec{
					NameFormat: "foo-bar",
				},
			},
			want: "foo-bar",
		},
		{
			name: "invalid: multiple substitutions",
			fields: fields{
				ObjectMeta: om,
				Spec: ContainerSpec{
					NameFormat: "foo-%s-bar-%s",
				},
			},
			want: "foo-test-uid-bar-%!s(MISSING)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
			}
			if got := c.GetContainerName(); got != tt.want {
				t.Errorf("Container.GetContainerName() = %v, want %v", got, tt.want)
			}
		})
	}
}
