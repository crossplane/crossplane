/*
Copyright 2018 The Conductor Authors.

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
	"time"

	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/database/v1alpha1"
	gcpv1alpha1 "github.com/upbound/conductor/pkg/apis/gcp/v1alpha1"
	gcpclients "github.com/upbound/conductor/pkg/clients/gcp"
	"github.com/upbound/conductor/pkg/util"
	"google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	connectionSecretRefFmt    = "%s-connection"
	connectionInstanceKey     = "instance"
	connectionDatabaseNameKey = "name"
	connectionUserKey         = "username"
	connectionPasswordKey     = "password"
	rootUserName              = "root"
	passwordDataLen           = 20
)

// AddCloudsqlInstance creates a new CloudsqlInstance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddCloudsqlInstance(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create clientset: %+v", err)
	}

	r := newCloudsqlInstanceReconciler(mgr, &gcpclients.CloudSQLClientFactory{}, clientset, NewReconcileCloudsqlInstanceOptions())
	return addCloudsqlInstanceReconciler(mgr, r)
}

// newCloudsqlInstanceReconciler returns a new reconcile.Reconciler
func newCloudsqlInstanceReconciler(mgr manager.Manager, cloudSQLAPIFactory gcpclients.CloudSQLAPIFactory,
	clientset kubernetes.Interface, options ReconcileCloudsqlInstanceOptions) reconcile.Reconciler {

	return &ReconcileCloudsqlInstance{
		Client:             mgr.GetClient(),
		cloudSQLAPIFactory: cloudSQLAPIFactory,
		clientset:          clientset,
		scheme:             mgr.GetScheme(),
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
	log.Printf("watching for changes to CloudSQL instances...")
	err = c.Watch(&source.Kind{Type: &databasev1alpha1.CloudsqlInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileCloudsqlInstance{}

// ReconcileCloudsqlInstance reconciles a CloudsqlInstance object
type ReconcileCloudsqlInstance struct {
	client.Client
	clientset          kubernetes.Interface
	cloudSQLAPIFactory gcpclients.CloudSQLAPIFactory
	cloudSQLClient     gcpclients.CloudSQLAPI
	config             *rest.Config
	scheme             *runtime.Scheme
	options            ReconcileCloudsqlInstanceOptions
}

// ReconcileCloudsqlInstanceOptions represent options to configure the CloudSQL reconciler
type ReconcileCloudsqlInstanceOptions struct {
	PostCreateSleepTime time.Duration
	WaitSleepTime       time.Duration
}

// NewReconcileCloudsqlInstanceOptions creates a new ReconcileCloudsqlInstanceOptions object with the default options
func NewReconcileCloudsqlInstanceOptions() ReconcileCloudsqlInstanceOptions {
	return ReconcileCloudsqlInstanceOptions{
		PostCreateSleepTime: 5 * time.Second,
		WaitSleepTime:       10 * time.Second,
	}
}

// Reconcile reads that state of the cluster for a CloudsqlInstance object and makes changes based on the state read
// and what is in the CloudsqlInstance.Spec
func (r *ReconcileCloudsqlInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
		instance.Status.InstanceName = fmt.Sprintf("%s-%s", instance.Name, string(instance.UID))
		log.Printf("cloud sql instance %s does not yet have an instance name, setting it to %s", instance.Name, instance.Status.InstanceName)
		if err := r.updateStatus(instance, getStatusMessage(instance, cloudSQLInstance), cloudSQLInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// look up the provider information for this instance
	provider := &gcpv1alpha1.Provider{}
	providerNamespacedName := apitypes.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err = r.Get(context.TODO(), providerNamespacedName, provider)
	if err != nil {
		log.Printf("failed to get provider %+v: %+v", providerNamespacedName, err)
		return reconcile.Result{}, err
	}

	// TODO: locking around access to this Cloud SQL client member
	if r.cloudSQLClient == nil {
		// we don't already have a Cloud SQL client, let's create one now
		c, err := r.cloudSQLAPIFactory.CreateAPIInstance(r.clientset, provider.Namespace, provider.Spec.Secret)
		if err != nil {
			err = fmt.Errorf("failed to get cloud sql client: %+v", err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}
		r.cloudSQLClient = c
	}

	cloudSQLInstance, err = r.cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		if !gcpclients.IsNotFound(err) {
			err = fmt.Errorf("failed to get cloud sql instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		// seems like we didn't find a cloud sql instance with this name, let's create one
		cloudSQLInstance = &sqladmin.DatabaseInstance{
			Name:            instance.Status.InstanceName,
			Region:          instance.Spec.Region,
			DatabaseVersion: instance.Spec.DatabaseVersion,
			Settings: &sqladmin.Settings{
				Tier:         instance.Spec.Tier,
				DataDiskType: instance.Spec.StorageType,
			},
		}

		log.Printf("cloud sql instance %s not found, will try to create it now: %+v", instance.Name, cloudSQLInstance)
		createOp, err := r.cloudSQLClient.CreateInstance(provider.Spec.ProjectID, cloudSQLInstance)
		if err != nil {
			err = fmt.Errorf("failed to start create operation for cloud sql instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		log.Printf("started create of cloud sql instance %s, operation %s %s", instance.Name, createOp.Name, createOp.Status)
		log.Printf("sleep a bit after creating the cloud sql instance...")
		time.Sleep(r.options.PostCreateSleepTime)
	}

	// wait for the cloud sql instance to be RUNNABLE
	running := false
	maxRetries := 50
	for i := 0; i <= maxRetries; i++ {
		cloudSQLInstance, err = r.cloudSQLClient.GetInstance(provider.Spec.ProjectID, instance.Status.InstanceName)
		if err != nil {
			log.Printf("failed to get cloud sql instance %s, waiting %v: %+v",
				instance.Name, r.options.WaitSleepTime, err)
		} else {
			// cloud sql instance has been created, update the CRD status now
			if err := r.updateStatus(instance, getStatusMessage(instance, cloudSQLInstance), cloudSQLInstance); err != nil {
				// updating the CRD status failed, return the error and try the next reconcile loop
				return reconcile.Result{}, err
			}

			if cloudSQLInstance.State == "RUNNABLE" {
				// CRD status updated and cloud sql instance in RUNNABLE state, we are all good
				log.Printf("cloud sql instance %s successfully created and in the RUNNABLE state", instance.Name)
				running = true
				break
			}

			log.Printf("cloud sql instance %s created but still in %s state, waiting %v",
				instance.Name, cloudSQLInstance.State, r.options.WaitSleepTime)
		}

		<-time.After(r.options.WaitSleepTime)
	}

	if !running {
		// the retry loop completed without finding the created cloud sql instance, report this as an error
		err = fmt.Errorf("gave up waiting for cloud sql instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	// ensure the root user is initialized
	err = r.initRootUser(instance, provider)
	if err != nil {
		err = fmt.Errorf("failed to init root user for cloud sql instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileCloudsqlInstance) initRootUser(instance *databasev1alpha1.CloudsqlInstance, provider *gcpv1alpha1.Provider) error {
	if instance.Spec.ConnectionSecretRef.Name == "" {
		// the user hasn't specified the name of the secret they want the connection information
		// stored in, generate one now
		secretName := fmt.Sprintf(connectionSecretRefFmt, instance.Name)
		log.Printf("connection secret ref for cloud sql instance %s is empty, setting it to %s", instance.Name, secretName)
		instance.Spec.ConnectionSecretRef.Name = secretName
		if err := r.Update(context.TODO(), instance); err != nil {
			return fmt.Errorf("failed to set connection secret ref: %+v", err)
		}
	}

	// first check if the root user has already been initialized
	connectionSecret, err := r.clientset.CoreV1().Secrets(instance.Namespace).Get(instance.Spec.ConnectionSecretRef.Name, metav1.GetOptions{})
	if err == nil {
		// we already have a password for the root user, we are done
		return nil
	}
	log.Printf("user '%s' is not initialized yet: %+v", rootUserName, err)

	users, err := r.cloudSQLClient.ListUsers(provider.Spec.ProjectID, instance.Status.InstanceName)
	if err != nil {
		return fmt.Errorf("failed to list users: %+v", err)
	}

	var rootUser *sqladmin.User
	for i := range users.Items {
		if users.Items[i].Name == rootUserName {
			rootUser = users.Items[i]
			break
		}
	}

	if rootUser == nil {
		return fmt.Errorf("failed to find user '%s'", rootUserName)
	}

	password, err := util.GeneratePassword(passwordDataLen)
	if err != nil {
		return fmt.Errorf("failed to generate password for user '%s': %+v", rootUser.Name, err)
	}
	rootUser.Password = password

	// update the user via Cloud SQL API
	log.Printf("updating user '%s'", rootUser.Name)
	updateUserOp, err := r.cloudSQLClient.UpdateUser(provider.Spec.ProjectID, instance.Status.InstanceName, rootUser.Host, rootUser.Name, rootUser)
	if err != nil {
		return fmt.Errorf("failed to start update user operation for user '%s': %+v", rootUser.Name, err)
	}

	// wait for the update user operation to complete
	log.Printf("waiting for update user operation %s to complete for user '%s'", updateUserOp.Name, rootUser.Name)
	updateUserOp, err = r.waitUntilOperationCompletes(updateUserOp.Name, provider)
	if err != nil {
		return fmt.Errorf("failed to wait until update user operation %s completed for user '%s': %+v", updateUserOp.Name, rootUser.Name, err)
	}

	log.Printf("update user operation for user '%s' completed. status: %s, errors: %+v", rootUser.Name, updateUserOp.Status, updateUserOp.Error)
	if !isOperationSuccessful(updateUserOp) {
		// the operation completed, but it failed
		m := fmt.Sprintf("update user operation for user '%s' failed: %+v", rootUser.Name, updateUserOp)
		if updateUserOp.Error != nil && len(updateUserOp.Error.Errors) > 0 {
			m = fmt.Sprintf("%s. errors: %+v", m, updateUserOp.Error.Errors)
		}
		return fmt.Errorf(m)
	}

	// save the user and connection info to a secret
	connectionSecret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            instance.Spec.ConnectionSecretRef.Name,
			Namespace:       instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{createOwnerRef(instance)},
		},
		Data: map[string][]byte{
			connectionInstanceKey: []byte(instance.Status.ConnectionName),
			connectionUserKey:     []byte(rootUser.Name),
			connectionPasswordKey: []byte(password),
		},
	}
	log.Printf("creating connection secret %s for user '%s'", connectionSecret.Name, rootUser.Name)
	if _, err := r.clientset.CoreV1().Secrets(instance.Namespace).Create(connectionSecret); err != nil {
		return fmt.Errorf("failed to update connection secret %s: %+v", connectionSecret.Name, err)
	}

	log.Printf("user '%s' initialized", rootUser.Name)
	return nil
}

func (r *ReconcileCloudsqlInstance) updateStatus(instance *databasev1alpha1.CloudsqlInstance, message string,
	cloudSQLInstance *sqladmin.DatabaseInstance) error {

	var state, providerID, connectionName string
	if cloudSQLInstance != nil {
		state = cloudSQLInstance.State
		providerID = cloudSQLInstance.SelfLink
		connectionName = cloudSQLInstance.ConnectionName
	}

	instance.Status = databasev1alpha1.CloudsqlInstanceStatus{
		Message:        message,
		State:          state,
		ProviderID:     providerID,
		ConnectionName: connectionName,
		InstanceName:   instance.Status.InstanceName,
	}
	err := r.Update(context.TODO(), instance)
	if err != nil {
		err = fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return err
	}

	return nil
}

func (r *ReconcileCloudsqlInstance) waitUntilOperationCompletes(operationID string, provider *gcpv1alpha1.Provider) (*sqladmin.Operation, error) {
	var err error
	var op *sqladmin.Operation

	maxRetries := 50
	for i := 0; i <= maxRetries; i++ {
		op, err = r.cloudSQLClient.GetOperation(provider.Spec.ProjectID, operationID)
		if err != nil {
			log.Printf("failed to get cloud sql operation %s, waiting %v: %+v",
				operationID, r.options.WaitSleepTime, err)
		} else if isOperationComplete(op) {
			// the operation has completed, simply return it
			return op, nil
		}

		<-time.After(r.options.WaitSleepTime)
	}

	return nil, fmt.Errorf("cloud sql operation %s did not complete in the allowed time period: %+v", operationID, op)
}

func isOperationComplete(op *sqladmin.Operation) bool {
	return op.EndTime != "" && op.Status == "DONE"
}

func isOperationSuccessful(op *sqladmin.Operation) bool {
	return op.Error == nil || len(op.Error.Errors) == 0
}

func getStatusMessage(instance *databasev1alpha1.CloudsqlInstance, cloudSQLInstance *sqladmin.DatabaseInstance) string {
	if cloudSQLInstance == nil {
		return fmt.Sprintf("Cloud SQL instance %s has not yet been created", instance.Name)
	}

	switch cloudSQLInstance.State {
	case "RUNNABLE":
		return fmt.Sprintf("Cloud SQL instance %s is running", instance.Name)
	case "PENDING_CREATE":
		return fmt.Sprintf("Cloud SQL instance %s is being created", instance.Name)
	case "FAILED":
		return fmt.Sprintf("Cloud SQL instance %s failed to be created", instance.Name)
	default:
		return fmt.Sprintf("Cloud SQL instance %s is in an unknown state %s", instance.Name, cloudSQLInstance.State)
	}
}

func createOwnerRef(instance *databasev1alpha1.CloudsqlInstance) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: databasev1alpha1.SchemeGroupVersion.String(),
		Kind:       databasev1alpha1.CloudsqlInstanceKind,
		Name:       instance.Name,
		UID:        instance.UID,
	}
}
