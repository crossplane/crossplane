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
	"log"
	"strings"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	databasev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/gcp/v1alpha1"
	gcpclients "github.com/crossplaneio/crossplane/pkg/clients/gcp"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	finalizer                 = "finalizer.cloudsqlinstances.database.gcp.crossplane.io"
	mysqlDefaultUserName      = "root"
	postgresqlDefaultUserName = "postgres"
	passwordDataLen           = 20

	errorSettingInstanceName    = "failed to set instance name"
	errorFetchingGCPProvider    = "failed to fetch GCP Provider"
	errorFetchingCloudSQLClient = "failed to fetch CloudSQL client"
	errorFetchingInstance       = "failed to fetch instance"
	errorCreatingInstance       = "failed to create instance"
	errorDeletingInstance       = "failed to delete instance"
	errorInitDefaultUser        = "failed to initialize default user"
	conditionStateChanged       = "instance state changed"
)

var (
	_ reconcile.Reconciler = &Reconciler{}
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
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("reconciling %s: %v", databasev1alpha1.CloudsqlInstanceKindAPIVersion, request)
	instance := &databasev1alpha1.CloudsqlInstance{}
	var cloudSQLInstance *sqladmin.DatabaseInstance

	// Fetch the CloudsqlInstance instance
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Printf("failed to get object at start of reconcile loop: %+v", err)
		return reconcile.Result{}, err
	}

	if instance.Status.InstanceName == "" {
		// we haven't generated a unique instance name yet, let's do that now
		instance.Status.InstanceName = "cloudsql-" + string(instance.UID)
		log.Printf("cloud sql instance %s does not yet have an instance name, setting it to %s", instance.Name, instance.Status.InstanceName)
		if err := r.Update(context.TODO(), instance); err != nil {
			return r.fail(instance, errorSettingInstanceName, err.Error())
		}
	}

	// look up the provider information for this instance
	provider := &gcpv1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	if err = r.Get(context.TODO(), providerNamespacedName, provider); err != nil {
		return r.fail(instance, errorFetchingGCPProvider, fmt.Sprintf("failed to get provider %+v: %+v", providerNamespacedName, err))
	}

	// create a Cloud SQL client during each reconciliation loop since each instance can have different creds
	cloudSQLClient, err := r.cloudSQLAPIFactory.CreateAPIInstance(r.clientset, provider.Namespace, provider.Spec.Secret)
	if err != nil {
		return r.fail(instance, errorFetchingCloudSQLClient, fmt.Sprintf("failed to get cloud sql client: %+v", err))
	}

	// check for CRD deletion and handle it if needed
	if instance.DeletionTimestamp != nil {
		if instance.Status.Condition(corev1alpha1.Deleting) == nil {
			// we haven't started the deletion of the CloudSQL resource yet, do it now
			log.Printf("cloud sql instance %s has been deleted, running finalizer now", instance.Name)
			return r.handleDeletion(cloudSQLClient, instance, provider)
		}
		// we already started the deletion of the CloudSQL resource, nothing more to do
		return reconcile.Result{}, nil
	}

	// Add finalizer to the CRD if it doesn't already exist
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(context.TODO(), instance); err != nil {
			log.Printf("failed to add finalizer to instance %s: %+v", instance.Name, err)
			return reconcile.Result{}, err
		}
	}

	// retrieve the CloudSQL instance from GCP to get the latest status
	cloudSQLInstance, err = cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		if !gcpclients.IsErrorNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get cloud sql instance %s: %+v", instance.Name, err))
		}

		// seems like we didn't find a cloud sql instance with this name, let's create one
		return r.handleCreation(cloudSQLClient, instance, provider)
	}

	stateChanged := instance.Status.State != cloudSQLInstance.State
	conditionType := gcpclients.CloudSQLConditionType(cloudSQLInstance.State)

	// cloud sql instance exists, update the CRD status now with its latest status
	if err := r.updateStatus(instance, gcpclients.CloudSQLStatusMessage(instance.Name, cloudSQLInstance), cloudSQLInstance); err != nil {
		// updating the CRD status failed, return the error and try the next reconcile loop
		log.Printf("failed to update status of instance %s: %+v", instance.Name, err)
		return reconcile.Result{}, err
	}

	if stateChanged {
		// the state of the instance has changed, let's set a corresponding condition on the CRD and then
		// requeue another reconiliation attempt
		if conditionType == corev1alpha1.Ready {
			// when we hit the running condition, clear out all old conditions first
			instance.Status.UnsetAllConditions()
		}

		conditionMessage := fmt.Sprintf("cloud sql instance %s is in the %s state", instance.Name, conditionType)
		log.Printf(conditionMessage)
		instance.Status.SetCondition(corev1alpha1.NewCondition(conditionType, conditionStateChanged, conditionMessage))
		return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
	}

	if conditionType != corev1alpha1.Ready {
		// the instance isn't running still, requeue another reconciliation attempt
		return reconcile.Result{Requeue: true}, nil
	}

	// ensure the default user is initialized
	if err = r.initDefaultUser(cloudSQLClient, instance, provider); err != nil {
		return r.fail(instance, errorInitDefaultUser, fmt.Sprintf("failed to init default user for cloud sql instance %s: %+v", instance.Name, err))
	}

	return reconcile.Result{}, nil
}

// handleCreation performs the operation to create the given CloudSQL instance in GCP
func (r *Reconciler) handleCreation(cloudSQLClient gcpclients.CloudSQLAPI,
	instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) (reconcile.Result, error) {

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

	log.Printf("cloud sql instance %s not found, will try to create it now: %+v", instance.Name, cloudSQLInstance)
	createOp, err := cloudSQLClient.CreateInstance(provider.Spec.ProjectID, cloudSQLInstance)
	if err != nil {
		return r.fail(instance, errorCreatingInstance, fmt.Sprintf("failed to start create operation for cloud sql instance %s: %+v", instance.Name, err))
	}

	log.Printf("started create of cloud sql instance %s, operation %s %s", instance.Name, createOp.Name, createOp.Status)
	return reconcile.Result{Requeue: true}, nil
}

// handleDeletion performs the operation to delete the given CloudSQL instance in GCP
func (r *Reconciler) handleDeletion(cloudSQLClient gcpclients.CloudSQLAPI,
	instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) (reconcile.Result, error) {

	// first get the latest status of the CloudSQL resource that needs to be deleted
	_, err := cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		if !gcpclients.IsErrorNotFound(err) {
			return r.fail(instance, errorFetchingInstance, fmt.Sprintf("failed to get cloud sql instance %s for deletion: %+v", instance.Name, err))
		}

		// CloudSQL instance doesn't exist, it's already deleted
		return r.markAsDeleting(instance)
	}

	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		// attempt to delete the CloudSQL instance now
		deleteOp, err := cloudSQLClient.DeleteInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
		if err != nil {
			return r.fail(instance, errorDeletingInstance, fmt.Sprintf("failed to start delete operation for cloud sql instance %s: %+v", instance.Name, err))
		}

		log.Printf("started deletion of cloud sql instance %s, operation %s %s", instance.Name, deleteOp.Name, deleteOp.Status)
	}
	return r.markAsDeleting(instance)
}

func (r *Reconciler) markAsDeleting(instance *databasev1alpha1.CloudsqlInstance) (reconcile.Result, error) {
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	return reconcile.Result{}, r.Update(context.TODO(), instance)
}

func (r *Reconciler) initDefaultUser(cloudSQLClient gcpclients.CloudSQLAPI,
	instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) error {

	// get the default database user name depending on the database version in use
	defaultUserName, err := getDefaultDBUserName(instance.Spec.DatabaseVersion)
	if err != nil {
		return err
	}

	// first ensure the connection secret name has been set
	secretName, err := r.ensureConnectionSecretNameSet(instance)
	if err != nil {
		return err
	}

	// check if the default user has already been initialized
	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Get(secretName, metav1.GetOptions{}); err == nil {
		// we already have a password for the default user, we are done
		return nil
	}
	log.Printf("user '%s' is not initialized yet: %+v", defaultUserName, err)

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
	log.Printf("updating user '%s'", defaultUser.Name)
	updateUserOp, err := cloudSQLClient.UpdateUser(provider.Spec.ProjectID, instance.Status.InstanceName, defaultUser.Name, defaultUser)
	if err != nil {
		return fmt.Errorf("failed to start update user operation for user '%s': %+v", defaultUser.Name, err)
	}

	// wait for the update user operation to complete
	log.Printf("waiting for update user operation %s to complete for user '%s'", updateUserOp.Name, defaultUser.Name)
	updateUserOp, err = gcpclients.WaitUntilOperationCompletes(updateUserOp.Name, provider, cloudSQLClient, r.options.WaitSleepTime)
	if err != nil {
		return fmt.Errorf("failed to wait until update user operation %s completed for user '%s': %+v", updateUserOp.Name, defaultUser.Name, err)
	}

	log.Printf("update user operation for user '%s' completed. status: %s, errors: %+v", defaultUser.Name, updateUserOp.Status, updateUserOp.Error)
	if !gcpclients.IsOperationSuccessful(updateUserOp) {
		// the operation completed, but it failed
		m := fmt.Sprintf("update user operation for user '%s' failed: %+v", defaultUser.Name, updateUserOp)
		if updateUserOp.Error != nil && len(updateUserOp.Error.Errors) > 0 {
			m = fmt.Sprintf("%s. errors: %+v", m, updateUserOp.Error.Errors)
		}
		return fmt.Errorf(m)
	}

	// save the user and connection info to a secret
	connectionSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{instance.OwnerReference()},
		},
		Data: map[string][]byte{
			corev1alpha1.ResourceCredentialsSecretEndpointKey: []byte(instance.Status.Endpoint),
			corev1alpha1.ResourceCredentialsSecretUserKey:     []byte(defaultUser.Name),
			corev1alpha1.ResourceCredentialsSecretPasswordKey: []byte(password),
		},
	}
	log.Printf("creating connection secret %s for user '%s'", connectionSecret.Name, defaultUser.Name)
	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(connectionSecret); err != nil {
		return fmt.Errorf("failed to update connection secret %s: %+v", connectionSecret.Name, err)
	}

	log.Printf("user '%s' initialized", defaultUser.Name)
	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *databasev1alpha1.CloudsqlInstance, reason, msg string) (reconcile.Result, error) {
	log.Printf("instance %s failed: '%s' %s", instance.Name, reason, msg)
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
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

func (r *Reconciler) ensureConnectionSecretNameSet(instance *databasev1alpha1.CloudsqlInstance) (string, error) {
	// if the secret name doesn't already exist, we'll need to update the instance with it
	updateNeeded := instance.Spec.ConnectionSecretRef.Name == ""

	// get or create the connection secret name
	secretName := instance.ConnectionSecretName()

	// if an update on the instance was needed, do it now
	if updateNeeded {
		if err := r.Update(context.TODO(), instance); err != nil {
			return "", fmt.Errorf("failed to set connection secret ref: %+v", err)
		}
	}

	return secretName, nil
}

func getDefaultDBUserName(dbVersion string) (string, error) {
	if strings.HasPrefix(dbVersion, databasev1alpha1.PostgresqlDBVersionPrefix) {
		return postgresqlDefaultUserName, nil
	} else if strings.HasPrefix(dbVersion, databasev1alpha1.MysqlDBVersionPrefix) {
		return mysqlDefaultUserName, nil
	}

	return "", fmt.Errorf("database version does not match any known types: %s", dbVersion)
}
