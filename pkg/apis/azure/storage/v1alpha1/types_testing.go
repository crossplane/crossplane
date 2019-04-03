/*
Copyright 2018 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance With the License.
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
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-06-01/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
)

// TestAccount wrapper for testing account object
type TestAccount struct {
	*Account
}

// NewTestAccount creates new account wrapper
func NewTestAccount(ns, name string) *TestAccount {
	return &TestAccount{Account: &Account{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  ns,
			Name:       name,
			Finalizers: []string{},
		},
	}}
}

// WithUID sets UID value
func (a *TestAccount) WithUID(uid string) *TestAccount {
	a.ObjectMeta.UID = types.UID(uid)
	return a
}

// WithCondition sets status condition
func (a *TestAccount) WithCondition(c corev1alpha1.Condition) *TestAccount {
	a.Status.ConditionedStatus.SetCondition(c)
	return a
}

// WithFailedCondition sets and activates Failed condition
func (a *TestAccount) WithFailedCondition(reason, msg string) *TestAccount {
	a.Status.SetFailed(reason, msg)
	return a
}

// WithDeleteTimestamp sets metadata deletion timestamp
func (a *TestAccount) WithDeleteTimestamp(t metav1.Time) *TestAccount {
	a.Account.ObjectMeta.DeletionTimestamp = &t
	return a
}

// WithFinalizer sets finalizer
func (a *TestAccount) WithFinalizer(f string) *TestAccount {
	a.Account.ObjectMeta.Finalizers = append(a.Account.ObjectMeta.Finalizers, f)
	return a
}

// WithProvider set a provider
func (a *TestAccount) WithProvider(name string) *TestAccount {
	a.Spec.ProviderRef = corev1.LocalObjectReference{Name: name}
	return a
}

// WithReclaimPolicy sets resource reclaim policy
func (a *TestAccount) WithReclaimPolicy(policy corev1alpha1.ReclaimPolicy) *TestAccount {
	a.Spec.ReclaimPolicy = policy
	return a
}

// WithStorageAccountSpec sets storage account specs
func (a *TestAccount) WithStorageAccountSpec(spec *StorageAccountSpec) *TestAccount {
	a.Spec.StorageAccountSpec = spec
	return a
}

// WithStorageAccountStatus set storage account status
func (a *TestAccount) WithStorageAccountStatus(status *StorageAccountStatus) *TestAccount {
	a.Status.StorageAccountStatus = status
	return a
}

// WithSpecStatusFromProperties set storage account spec status from storage properties
func (a *TestAccount) WithSpecStatusFromProperties(props *storage.AccountProperties) *TestAccount {
	acct := &storage.Account{
		AccountProperties: props,
	}
	a.WithStorageAccountSpec(NewStorageAccountSpec(acct)).
		WithStorageAccountStatus(NewStorageAccountStatus(acct))
	return a
}

// WithStatusConnectionRef set connection references
func (a *TestAccount) WithStatusConnectionRef(ref string) *TestAccount {
	a.Status.ConnectionSecretRef = corev1.LocalObjectReference{Name: ref}
	return a
}
