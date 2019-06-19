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

package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/pkg/errors"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	gcpclients "github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/resource"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName            = "cloudsqlinstances.database.gcp.crossplane.io"
	finalizer                 = "finalizer" + controllerName
	mysqlDefaultUserName      = "root"
	postgresqlDefaultUserName = "postgres"
	passwordDataLen           = 20
)

var (
	log = logging.Logger.WithName("controller." + controllerName)
)

// Add creates a new CloudsqlInstance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create clientset: %+v", err)
	}

	r := newCloudsqlInstanceReconciler(mgr, &gcpclients.CloudSQLClientFactory{}, clientset, NewReconcilerOptions())
	return addCloudsqlInstanceReconciler(mgr, r)
}

// newCloudsqlInstanceReconciler returns a new reconcile.Reconciler
func newCloudsqlInstanceReconciler(mgr manager.Manager, cloudSQLAPIFactory gcpclients.CloudSQLAPIFactory,
	clientset kubernetes.Interface, options ReconcilerOptions) *Reconciler {

	return &Reconciler{
		Client:             mgr.GetClient(),
		cloudSQLAPIFactory: cloudSQLAPIFactory,
		clientset:          clientset,
		options:            options,
	}
}

// addCloudsqlInstanceReconciler adds a new Controller to mgr with r as the reconcile.Reconciler
func addCloudsqlInstanceReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("CloudsqlInstance-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CloudsqlInstance
	err = c.Watch(&source.Kind{Type: &databasev1alpha1.CloudsqlInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// Reconciler reconciles a CloudsqlInstance object
type Reconciler struct {
	client.Client
	clientset          kubernetes.Interface
	cloudSQLAPIFactory gcpclients.CloudSQLAPIFactory
	options            ReconcilerOptions
}

// ReconcilerOptions represent options to configure the CloudSQL reconciler
type ReconcilerOptions struct {
	WaitSleepTime time.Duration
}

// NewReconcilerOptions creates a new ReconcilerOptions object with the default options
func NewReconcilerOptions() ReconcilerOptions {
	return ReconcilerOptions{
		WaitSleepTime: 10 * time.Second,
	}
}

// Reconcile reads that state of the cluster for a CloudsqlInstance object and makes changes based on the state read
// and what is in the CloudsqlInstance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	// TODO(negz): This method's cyclomatic complexity is very high. Consider
	// refactoring it if you touch it.

	log.V(logging.Debug).Info("reconciling", "kind", databasev1alpha1.CloudsqlInstanceKindAPIVersion, "request", request)
	instance := &databasev1alpha1.CloudsqlInstance{}
	var cloudSQLInstance *sqladmin.DatabaseInstance

	// Fetch the CloudsqlInstance instance
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		log.Error(err, "failed to get object at start of reconcile loop")
		return reconcile.Result{}, err
	}

	if instance.Status.InstanceName == "" {
		// we haven't generated a unique instance name yet, let's do that now
		instance.Status.InstanceName = "cloudsql-" + string(instance.UID)
		log.V(logging.Debug).Info("set cloud sql instance name", "instance", instance)
		if err := r.Update(context.TODO(), instance); err != nil {
			return r.fail(instance, err)
		}
	}

	// look up the provider information for this instance
	provider := &gcpv1alpha1.Provider{}
	n := meta.NamespacedNameOf(instance.Spec.ProviderReference)
	if err = r.Get(context.TODO(), n, provider); err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to get provider %+v", n))
	}

	// create a Cloud SQL client during each reconciliation loop since each instance can have different creds
	cloudSQLClient, err := r.cloudSQLAPIFactory.CreateAPIInstance(r.clientset, provider.Namespace, provider.Spec.Secret)
	if err != nil {
		return r.fail(instance, errors.Wrap(err, "failed to get cloud sql client"))
	}

	// check for CRD deletion and handle it if needed
	if instance.DeletionTimestamp != nil {
		log.V(logging.Debug).Info("cloud sql instance has been deleted, running finalizer now", "instance", instance)
		return r.handleDeletion(cloudSQLClient, instance, provider)
	}

	// Add finalizer to the CRD if it doesn't already exist
	meta.AddFinalizer(instance, finalizer)
	if err := r.Update(context.TODO(), instance); err != nil {
		log.Error(err, "failed to add finalizer to instance", "instance", instance.Name)
		return reconcile.Result{}, err
	}

	// retrieve the CloudSQL instance from GCP to get the latest status
	cloudSQLInstance, err = cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		if !gcpclients.IsErrorNotFound(err) {
			return r.fail(instance, errors.Wrapf(err, "failed to get cloud sql instance %s", instance.Name))
		}

		// seems like we didn't find a cloud sql instance with this name, let's create one
		return r.handleCreation(cloudSQLClient, instance, provider)
	}

	stateChanged := cloudSQLInstance.State != instance.Status.State

	// cloud sql instance exists, update the CRD status now with its latest status
	if err := r.updateStatus(instance, gcpclients.CloudSQLStatusMessage(instance.Name, cloudSQLInstance), cloudSQLInstance); err != nil {
		log.Error(err, "failed to update status of instance", "instance", instance)
		return reconcile.Result{}, err
	}

	if stateChanged {
		// the state of the instance has changed, let's set a corresponding condition on the CRD and then
		// requeue another reconiliation attempt
		c := gcpclients.CloudSQLCondition(cloudSQLInstance.State)
		log.V(logging.Debug).Info("state changed", "instance", instance, "condition", c.Reason)
		instance.Status.SetConditions(c, corev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
	}

	if cloudSQLInstance.State != databasev1alpha1.StateRunnable {
		// the instance isn't running still, requeue another reconciliation attempt
		instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
		return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
	}

	// ensure the default user is initialized
	if err = r.initDefaultUser(cloudSQLClient, instance, provider); err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to init default user for cloud sql instance %s", instance.Name))
	}

	instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
}

// handleCreation performs the operation to create the given CloudSQL instance in GCP
func (r *Reconciler) handleCreation(cloudSQLClient gcpclients.CloudSQLAPI, instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.Creating())

	cloudSQLInstance := &sqladmin.DatabaseInstance{
		Name:            instance.Status.InstanceName,
		Region:          instance.Spec.Region,
		DatabaseVersion: instance.Spec.DatabaseVersion,
		Settings: &sqladmin.Settings{
			Tier:           instance.Spec.Tier,
			DataDiskType:   instance.Spec.StorageType,
			DataDiskSizeGb: instance.Spec.StorageGB,
			IpConfiguration: &sqladmin.IpConfiguration{
				AuthorizedNetworks: []*sqladmin.AclEntry{
					// TODO: we need to come up with better AuthorizedNetworks handing, for now a short cut - open to all
					{Value: "0.0.0.0/0"},
				},
			},
		},
	}

	log.V(logging.Debug).Info("cloud sql instance not found, will try to create it now", "instance", instance, "creating", cloudSQLInstance)
	createOp, err := cloudSQLClient.CreateInstance(provider.Spec.ProjectID, cloudSQLInstance)
	if err != nil {
		return r.fail(instance, errors.Wrapf(err, "failed to start create operation for cloud sql instance %s", instance.Name))
	}

	log.V(logging.Debug).Info("started create of cloud sql instance", "instance", instance, "operation", createOp)
	instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{Requeue: true}, nil
}

// handleDeletion performs the operation to delete the given CloudSQL instance in GCP
func (r *Reconciler) handleDeletion(cloudSQLClient gcpclients.CloudSQLAPI, instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.Deleting())

	// first get the latest status of the CloudSQL resource that needs to be deleted
	if _, err := cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName); err != nil {
		if !gcpclients.IsErrorNotFound(err) {
			return r.fail(instance, errors.Wrapf(err, "failed to get cloud sql instance %s for deletion", instance.Name))
		}

		// CloudSQL instance doesn't exist, it's already deleted
		meta.RemoveFinalizer(instance, finalizer)
		instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
		return reconcile.Result{}, r.Update(context.TODO(), instance)
	}

	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		// attempt to delete the CloudSQL instance now
		deleteOp, err := cloudSQLClient.DeleteInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
		if err != nil {
			return r.fail(instance, errors.Wrapf(err, "failed to start delete operation for cloud sql instance %s", instance.Name))
		}

		log.V(logging.Debug).Info("started deletion of cloud sql instance", "instance", instance, "operation", deleteOp)
	}

	meta.RemoveFinalizer(instance, finalizer)
	instance.Status.SetConditions(corev1alpha1.ReconcileSuccess())
	return reconcile.Result{}, r.Update(context.TODO(), instance)
}

// TODO(negz): This method's cyclomatic complexity is very high. Consider
// refactoring it if you touch it.
// nolint:gocyclo
func (r *Reconciler) initDefaultUser(cloudSQLClient gcpclients.CloudSQLAPI,
	instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) error {

	// get the default database user name depending on the database version in use
	defaultUserName, err := getDefaultDBUserName(instance.Spec.DatabaseVersion)
	if err != nil {
		return err
	}

	// check if the default user has already been initialized
	secretName := instance.Spec.WriteConnectionSecretTo.Name
	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Get(secretName, metav1.GetOptions{}); err == nil {
		// we already have a password for the default user, we are done
		return nil
	}
	log.V(logging.Debug).Info("user is not initialized yet", "username", defaultUserName)

	users, err := cloudSQLClient.ListUsers(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to list users: %+v", err)
	}

	var defaultUser *sqladmin.User
	for i := range users.Items {
		if users.Items[i].Name == defaultUserName {
			defaultUser = users.Items[i]
			break
		}
	}

	if defaultUser == nil {
		return fmt.Errorf("failed to find user '%s'", defaultUserName)
	}

	password, err := util.GeneratePassword(passwordDataLen)
	if err != nil {
		return fmt.Errorf("failed to generate password for user '%s': %+v", defaultUser.Name, err)
	}
	defaultUser.Password = password

	// update the user via Cloud SQL API
	log.V(logging.Debug).Info("updating user", "username", defaultUser.Name)
	updateUserOp, err := cloudSQLClient.UpdateUser(provider.Spec.ProjectID, instance.Status.InstanceName, defaultUser.Name, defaultUser)
	if err != nil {
		return fmt.Errorf("failed to start update user operation for user '%s': %+v", defaultUser.Name, err)
	}

	// wait for the update user operation to complete
	log.V(logging.Debug).Info("waiting for update user operation to complete for user", "operation", updateUserOp, "username", defaultUser)
	updateUserOp, err = gcpclients.WaitUntilOperationCompletes(updateUserOp.Name, provider, cloudSQLClient, r.options.WaitSleepTime)
	if err != nil {
		return fmt.Errorf("failed to wait until update user operation %s completed for user '%s': %+v", updateUserOp.Name, defaultUser.Name, err)
	}

	log.V(logging.Debug).Info("update user operation completed", "username", defaultUser.Name, "operation", updateUserOp)
	if !gcpclients.IsOperationSuccessful(updateUserOp) {
		// the operation completed, but it failed
		m := fmt.Sprintf("update user operation for user '%s' failed: %+v", defaultUser.Name, updateUserOp)
		if updateUserOp.Error != nil && len(updateUserOp.Error.Errors) > 0 {
			m = fmt.Sprintf("%s. errors: %+v", m, updateUserOp.Error.Errors)
		}
		return fmt.Errorf(m)
	}

	s := resource.ConnectionSecretFor(instance, databasev1alpha1.CloudsqlInstanceGroupVersionKind)
	s.Data = map[string][]byte{
		corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(instance.Status.Endpoint),
		corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(defaultUser.Name),
		corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
	}
	log.V(logging.Debug).Info("creating connection secret", "secret", s, "username", defaultUser.Name)
	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(s); err != nil {
		return fmt.Errorf("failed to update connection secret %s: %+v", s.Name, err)
	}

	log.V(logging.Debug).Info("user initialized", "username", defaultUser.Name)
	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *databasev1alpha1.CloudsqlInstance, err error) (reconcile.Result, error) {
	instance.Status.SetConditions(corev1alpha1.ReconcileError(err))
	return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
}

func (r *Reconciler) updateStatus(instance *databasev1alpha1.CloudsqlInstance, message string,
	cloudSQLInstance *sqladmin.DatabaseInstance) error {

	var state, providerID, endpoint string
	if cloudSQLInstance != nil {
		state = cloudSQLInstance.State
		providerID = cloudSQLInstance.SelfLink
		if len(cloudSQLInstance.IpAddresses) > 0 {
			endpoint = cloudSQLInstance.IpAddresses[0].IpAddress
		}
	}

	instance.Status.Message = message
	instance.Status.State = state
	instance.Status.ProviderID = providerID
	instance.Status.Endpoint = endpoint

	if err := r.Update(context.TODO(), instance); err != nil {
		return fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
	}

	return nil
}

func getDefaultDBUserName(dbVersion string) (string, error) {
	if strings.HasPrefix(dbVersion, databasev1alpha1.PostgresqlDBVersionPrefix) {
		return postgresqlDefaultUserName, nil
	} else if strings.HasPrefix(dbVersion, databasev1alpha1.MysqlDBVersionPrefix) {
		return mysqlDefaultUserName, nil
	}

	return "", fmt.Errorf("database version does not match any known types: %s", dbVersion)
}
