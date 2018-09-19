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
	gcpclients "github.com/upbound/conductor/pkg/clients/gcp"
	"google.golang.org/api/sqladmin/v1beta4"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AddCloudsqlInstance creates a new CloudsqlInstance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func AddCloudsqlInstance(mgr manager.Manager) error {
	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to get clientset: %+v", err)
	}

	cloudSQLClient, err := gcpclients.NewCloudSQLClient(clientset)
	if err != nil {
		return fmt.Errorf("failed to get CloudSQL client: %+v", err)
	}

	r := newCloudsqlInstanceReconciler(mgr, cloudSQLClient, NewReconcileCloudsqlInstanceOptions())
	return addCloudsqlInstanceReconciler(mgr, r)
}

// newCloudsqlInstanceReconciler returns a new reconcile.Reconciler
func newCloudsqlInstanceReconciler(mgr manager.Manager, cloudSQLClient gcpclients.CloudSQLAPI,
	options ReconcileCloudsqlInstanceOptions) reconcile.Reconciler {

	return &ReconcileCloudsqlInstance{
		Client:         mgr.GetClient(),
		cloudSQLClient: cloudSQLClient,
		scheme:         mgr.GetScheme(),
		options:        options,
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
	cloudSQLClient gcpclients.CloudSQLAPI
	scheme         *runtime.Scheme
	options        ReconcileCloudsqlInstanceOptions
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
		WaitSleepTime:       20 * time.Second,
	}
}

// Reconcile reads that state of the cluster for a CloudsqlInstance object and makes changes based on the state read
// and what is in the CloudsqlInstance.Spec
func (r *ReconcileCloudsqlInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CloudsqlInstance instance
	instance := &databasev1alpha1.CloudsqlInstance{}
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

	cloudSQLInstance, err := r.cloudSQLClient.GetInstance(instance.Spec.ProjectID, instance.Name)
	if err != nil {
		if !gcpclients.IsNotFound(err) {
			err = fmt.Errorf("failed to get cloud sql instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		// seems like we didn't find a cloud sql instance with this name, let's create one
		cloudSQLInstance = &sqladmin.DatabaseInstance{
			Name:            instance.Name,
			Region:          instance.Spec.Region,
			DatabaseVersion: instance.Spec.DatabaseVersion,
			Settings: &sqladmin.Settings{
				Tier:         instance.Spec.Tier,
				DataDiskType: instance.Spec.StorageType,
			},
		}

		log.Printf("cloud sql instance %s not found, will try to create it now: %+v", instance.Name, cloudSQLInstance)
		createOp, err := r.cloudSQLClient.CreateInstance(instance.Spec.ProjectID, cloudSQLInstance)
		if err != nil {
			err = fmt.Errorf("failed to start create operation for cloud sql instance %s: %+v", instance.Name, err)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		log.Printf("started create of cloud sql instance %s: %+v", instance.Name, createOp)
		log.Printf("sleep a bit after creating the cloud sql instance...")
		time.Sleep(r.options.PostCreateSleepTime)
	}

	// wait for the cloud sql instance to be RUNNABLE
	maxRetries := 50
	for i := 0; i <= maxRetries; i++ {
		cloudSQLInstance, err = r.cloudSQLClient.GetInstance(instance.Spec.ProjectID, instance.Name)
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
				log.Printf("cloud sql instance %s successfully created and in the RUNNABLE state. %+v",
					instance.Name, cloudSQLInstance)
				return reconcile.Result{}, nil
			}

			log.Printf("cloud sql instance %s created but still in %s state, waiting %v",
				instance.Name, cloudSQLInstance.State, r.options.WaitSleepTime)
		}

		<-time.After(r.options.WaitSleepTime)
	}

	// the retry loop completed without finding the created cloud sql instance, report this as an error
	err = fmt.Errorf("gave up waiting for cloud sql instance %s: %+v", instance.Name, err)
	log.Printf("%+v", err)
	return reconcile.Result{}, err
}

func (r *ReconcileCloudsqlInstance) updateStatus(instance *databasev1alpha1.CloudsqlInstance, message string,
	cloudSQLInstance *sqladmin.DatabaseInstance) error {

	instance.Status = databasev1alpha1.CloudsqlInstanceStatus{
		Message:    message,
		State:      cloudSQLInstance.State,
		ProviderID: cloudSQLInstance.SelfLink,
	}
	err := r.Update(context.TODO(), instance)
	if err != nil {
		err = fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return err
	}

	return nil
}

func getStatusMessage(instance *databasev1alpha1.CloudsqlInstance, cloudSQLInstance *sqladmin.DatabaseInstance) string {
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
