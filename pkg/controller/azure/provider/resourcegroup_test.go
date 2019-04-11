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

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	azurev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/azure/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	fakerg "github.com/crossplaneio/crossplane/pkg/clients/azure/resourcegroup/fake"
)

const (
	namespaceRG = "cool-namespace"
	uid         = types.UID("definitely-a-uuid")
	name        = "cool-rg"
	location    = "coolplace"
	// subscription = "totally-a-uuid"

	// providerSecretName = "cool-azure-secret"
	// providerSecretKey  = "credentials"
	// providerSecretData = "definitelyjson"
)

var (
	errorBoom = errors.New("boom")

	// provider = azurev1alpha1.Provider{
	// 	ObjectMeta: metav1.ObjectMeta{Namespace: namespaceRG, Name: providerName},
	// 	Spec: azurev1alpha1.ProviderSpec{
	// 		Secret: corev1.SecretKeySelector{
	// 			LocalObjectReference: corev1.LocalObjectReference{Name: providerSecretName},
	// 			Key:                  providerSecretKey,
	// 		},
	// 	},
	// 	Status: azurev1alpha1.ProviderStatus{
	// 		ConditionedStatus: corev1alpha1.ConditionedStatus{
	// 			Conditions: []corev1alpha1.Condition{{Type: corev1alpha1.Ready, Status: corev1.ConditionTrue}},
	// 		},
	// 	},
	// }

	// providerSecret = corev1.Secret{
	// 	ObjectMeta: metav1.ObjectMeta{Namespace: namespaceRG, Name: providerSecretName},
	// 	Data:       map[string][]byte{providerSecretKey: []byte(providerSecretData)},
	// }
)

type resourceModifier func(*azurev1alpha1.ResourceGroup)

func withConditions(c ...corev1alpha1.Condition) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Status.ConditionedStatus.Conditions = c }
}

func withFinalizers(f ...string) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.ObjectMeta.Finalizers = f }
}

// func withReclaimPolicy(p corev1alpha1.ReclaimPolicy) resourceModifier {
// 	return func(r *azurev1alpha1.ResourceGroup) { r.Spec.ReclaimPolicy = p }
// }

func withName(n string) resourceModifier {
	return func(r *azurev1alpha1.ResourceGroup) { r.Status.Name = n }
}

// func withDeletionTimestamp(t time.Time) resourceModifier {
// 	return func(r *azurev1alpha1.ResourceGroup) { r.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: t} }
// }

func resource(rm ...resourceModifier) *azurev1alpha1.ResourceGroup {
	r := &azurev1alpha1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespaceRG,
			Name:       name,
			UID:        uid,
			Finalizers: []string{},
		},
		Spec: azurev1alpha1.ResourceGroupSpec{
			Name:        name,
			Location:    location,
			ProviderRef: corev1.LocalObjectReference{Name: providerName},
		},
		Status: azurev1alpha1.ResourceGroupStatus{
			Name: name,
		},
	}

	for _, m := range rm {
		m(r)
	}

	return r
}

// Test that our Reconciler implementation satisfies the Reconciler interface.
var _ reconcile.Reconciler = &Reconciler{}

func TestCreate(t *testing.T) {
	cases := []struct {
		name        string
		csd         createsyncdeleter
		r           *azurev1alpha1.ResourceGroup
		want        *azurev1alpha1.ResourceGroup
		wantRequeue bool
	}{
		{
			name: "SuccessfulCreate",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ resources.Group) (resources.Group, error) {
					return resources.Group{}, nil
				},
			}},
			r: resource(),
			want: resource(
				withConditions(corev1alpha1.Condition{Type: corev1alpha1.Creating, Status: corev1.ConditionTrue}),
				withFinalizers(finalizerRG),
				withName(name),
			),
			wantRequeue: true,
		},
		{
			name: "FailedCreate",
			csd: &azureResourceGroup{client: &fakerg.MockClient{
				MockCreateOrUpdate: func(_ context.Context, _ string, _ resources.Group) (resources.Group, error) {
					return resources.Group{}, errorBoom
				},
			}},
			r: resource(),
			want: resource(withConditions(
				corev1alpha1.Condition{
					Type:    corev1alpha1.Failed,
					Status:  corev1.ConditionTrue,
					Reason:  reasonCreatingResource,
					Message: errorBoom.Error(),
				},
			)),
			wantRequeue: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotRequeue := tc.csd.Create(ctx, tc.r)

			if gotRequeue != tc.wantRequeue {
				t.Errorf("tc.mock.CreateOrUpdate(...): want: %t got: %t", tc.wantRequeue, gotRequeue)
			}

			if diff := deep.Equal(tc.want, tc.r); diff != nil {
				t.Errorf("r: want != got:\n%s", diff)
			}
		})
	}
}
