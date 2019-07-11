/*
Copyright 2019 The Crossplane Authors.

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

package database

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/gcp/cloudsql"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

type localOperations interface {
	// Bucket object managedOperations
	addFinalizer(context.Context) error
	isReclaimDelete() bool
	isInstanceReady() bool
	needsUpdate(*sqladmin.DatabaseInstance) bool
	removeFinalizer(context.Context) error

	// Controller-runtime managedOperations
	updateObject(ctx context.Context) error
	updateInstanceStatus(context.Context, *sqladmin.DatabaseInstance) error
	updateReconcileStatus(context.Context, error) error
	updateConnectionSecret(ctx context.Context) (*corev1.Secret, error)
}

type localHandler struct {
	*v1alpha1.CloudsqlInstance
	client client.Client
}

var _ localOperations = &localHandler{}

func newLocalHandler(instance *v1alpha1.CloudsqlInstance, kube client.Client) *localHandler {
	return &localHandler{
		CloudsqlInstance: instance,
		client:           kube,
	}
}

//
// Crossplane GCP Bucket object managedOperations
//
func (h *localHandler) addFinalizer(ctx context.Context) error {
	meta.AddFinalizer(h, finalizer)
	return h.updateObject(ctx)
}

func (h *localHandler) removeFinalizer(ctx context.Context) error {
	meta.RemoveFinalizer(h, finalizer)
	return h.updateObject(ctx)
}

func (h *localHandler) isInstanceReady() bool {
	return h.IsRunnable()
}

func (h *localHandler) isReclaimDelete() bool {
	return h.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete
}

func (h *localHandler) needsUpdate(actual *sqladmin.DatabaseInstance) bool {
	// TODO: update functionality is not supported for this instance.
	//   In order to add this support we need to refactor DatabaseInstanceType
	return false

	// TODO: when we ready to support update, delete 4 lines above and uncomment the lines below
	//  consider using cmp.Equal to determine whether an update is required
}

func (h *localHandler) updateObject(ctx context.Context) error {
	return h.client.Update(ctx, h.CloudsqlInstance)
}

func (h *localHandler) updateInstanceStatus(ctx context.Context, inst *sqladmin.DatabaseInstance) error {
	h.SetStatus(inst)
	return h.client.Status().Update(ctx, h.CloudsqlInstance)
}

func (h *localHandler) updateReconcileStatus(ctx context.Context, err error) error {
	if err == nil {
		h.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	} else {
		h.Status.SetConditions(corev1alpha1.ReconcileError(err))
	}
	return h.client.Status().Update(ctx, h.CloudsqlInstance)
}

func (h *localHandler) getConnectionSecret(ctx context.Context) (*corev1.Secret, error) {
	key := types.NamespacedName{
		Name:      h.ConnectionSecret().Name,
		Namespace: h.GetNamespace(),
	}
	s := &corev1.Secret{}
	return s, h.client.Get(ctx, key, s)
}

func (h *localHandler) updateConnectionSecret(ctx context.Context) (*corev1.Secret, error) {
	secret := h.ConnectionSecret()

	password, err := util.GeneratePassword(v1alpha1.PasswordLength)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate password")
	}

	s := secret.DeepCopy()
	if err := util.CreateOrUpdate(ctx, h.client, s, func() error {
		if !meta.HaveSameController(s, secret) {
			return errors.Errorf("connection secret %s/%s exists and is not controlled by %s/%s",
				s.GetNamespace(), s.GetName(), h.GetNamespace(), h.GetName())
		}

		if _, found := s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]; !found {
			s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey] = []byte(password)
		}
		s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = secret.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]
		s.Data[corev1alpha1.ResourceCredentialsSecretUserKey] = secret.Data[corev1alpha1.ResourceCredentialsSecretUserKey]
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

type managedOperations interface {
	localOperations
	// DatabaseInstance managedOperations
	getInstance(ctx context.Context) (*sqladmin.DatabaseInstance, error)
	createInstance(ctx context.Context) error
	updateInstance(ctx context.Context) error
	deleteInstance(ctx context.Context) error

	// DatabaseUser managedOperations
	updateUserCreds(ctx context.Context) error
}

type managedHandler struct {
	*v1alpha1.CloudsqlInstance
	localOperations
	instance cloudsql.InstanceService
	user     cloudsql.UserService
}

var _ managedOperations = &managedHandler{}

func newManagedHandler(ctx context.Context, inst *v1alpha1.CloudsqlInstance, tops localOperations, creds *google.Credentials) (*managedHandler, error) {
	instClient, err := cloudsql.NewInstanceClient(ctx, creds)
	if err != nil {
		return nil, err
	}
	userClient, err := cloudsql.NewUserClient(ctx, creds)
	if err != nil {
		return nil, err
	}
	return &managedHandler{
		CloudsqlInstance: inst,
		localOperations:  tops,
		instance:         instClient,
		user:             userClient,
	}, nil
}

func (h *managedHandler) getInstance(ctx context.Context) (*sqladmin.DatabaseInstance, error) {
	inst, err := h.instance.Get(ctx, h.GetResourceName())
	if err == nil {
		h.SetStatus(inst)
	}
	return inst, err
}

func (h *managedHandler) createInstance(ctx context.Context) error {
	h.Status.SetConditions(corev1alpha1.Creating())
	return h.instance.Create(ctx, h.DatabaseInstance(h.GetResourceName()))
}

func (h *managedHandler) updateInstance(ctx context.Context) error {
	name := h.GetResourceName()
	return h.instance.Update(ctx, name, h.DatabaseInstance(name))
}

func (h *managedHandler) deleteInstance(ctx context.Context) error {
	return h.instance.Delete(ctx, h.GetResourceName())
}

func (h *managedHandler) getUser(ctx context.Context) (*sqladmin.User, error) {
	instanceName := h.GetResourceName()
	userName := h.DatabaseUserName()
	users, err := h.user.List(ctx, instanceName)
	if err != nil {
		return nil, err
	}
	for _, v := range users {
		if v.Name == userName {
			return v, nil
		}
	}
	return nil, &googleapi.Error{
		Code:    http.StatusNotFound,
		Message: fmt.Sprintf("user: %s is not found", userName),
	}
}

// updateUserCreds
//
//  Currently there is no "good" way to validate user password drift, which
//  leaves us with two options:
//  1. Set once an forget it (previous approach)
//  2. Perform user update even if there are no changes (including in password)
//
// TODO(illya): In the future, we need to come up with more sophisticated means
//  to detect the password value drift
func (h *managedHandler) updateUserCreds(ctx context.Context) error {

	secret, err := h.updateConnectionSecret(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to update connection secret")
	}

	user, err := h.getUser(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get user")
	}
	user.Password = string(secret.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey])

	return h.user.Update(ctx, user.Instance, user.Name, user)
}
