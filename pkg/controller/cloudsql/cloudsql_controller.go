/*
Copyright 2018 The Project Conductor Authors.

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

package cloudsql

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	gcpv1alpha1 "github.com/upbound/project-conductor/pkg/apis/gcp/v1alpha1"
	"github.com/upbound/project-conductor/pkg/clients"
	"golang.org/x/oauth2/google"
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

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new CloudSql Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this gcp.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	r, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return add(mgr, r)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	clientset, err := clients.GetClientset()
	if err != nil {
		return nil, err
	}

	hc, err := google.DefaultClient(context.Background(), sqladmin.SqlserviceAdminScope)
	if err != nil {
		return nil, err
	}

	return &ReconcileCloudSql{
		Client:        mgr.GetClient(),
		Clientset:     clientset,
		gcpHTTPClient: hc,
		scheme:        mgr.GetScheme(),
	}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("cloudsql-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CloudSql
	log.Printf("watching for changes to CloudSql instances...")
	err = c.Watch(&source.Kind{Type: &gcpv1alpha1.CloudSql{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileCloudSql{}

// ReconcileCloudSql reconciles a CloudSql object
type ReconcileCloudSql struct {
	client.Client
	Clientset     kubernetes.Interface
	gcpHTTPClient *http.Client
	scheme        *runtime.Scheme
}

// Reconcile reads that state of the cluster for a CloudSql object and makes changes based on the state read
// and what is in the CloudSql.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gcp.project-conductor.io,resources=cloudsqls,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileCloudSql) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CloudSql instance
	instance := &gcpv1alpha1.CloudSql{}
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

	cloudSqlClient, err := r.cloudSqlClient()
	if err != nil {
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	cloudSqlInstance, err := cloudSqlClient.Instances.Get(instance.Spec.ProjectID, instance.Name).Do()
	if err == nil {
		// cloud sql instance with this name already exists
		if instance.Status.ProviderID == "" {
			// we don't have the provider ID on the CRD yet, store it now
			log.Printf("cloud sql instance %s already exists but CRD has no ID yet, updating CRD now", instance.Name)
			err = r.updateStatus(instance, "cloud sql instance already exists", cloudSqlInstance)
			return reconcile.Result{}, err
		}

		if instance.Status.ProviderID != cloudSqlInstance.SelfLink {
			// the retrieved cloud sql instance doesn't match the ID stored in the CRD status
			err := fmt.Errorf("cloud sql instance %s has a a self link %s that does not match CRD provider ID %s",
				cloudSqlInstance.Name, cloudSqlInstance.SelfLink, instance.Status.ProviderID)
			log.Printf("%+v", err)
			return reconcile.Result{}, err
		}

		// cloud sql instance is already created and the CRD status is in agreement
		log.Printf("cloud sql instance %s already exists and matches CRD, ID %s", instance.Name, instance.Status.ProviderID)
		return reconcile.Result{}, nil
	} else if err != nil && !clients.IsGoogleAPINotFound(err) {
		err = fmt.Errorf("failed to get cloud sql instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	// seems like we didn't find a cloud sql instance with this name, let's create one
	cloudSqlInstance = &sqladmin.DatabaseInstance{
		Name:            instance.Name,
		Region:          instance.Spec.Region,
		DatabaseVersion: instance.Spec.DatabaseVersion,
		Settings: &sqladmin.Settings{
			Tier:         instance.Spec.Tier,
			DataDiskType: instance.Spec.StorageType,
		},
	}

	log.Printf("cloud sql instance %s not found, will try to create it now: %+v", instance.Name, cloudSqlInstance)
	insertOp, err := cloudSqlClient.Instances.Insert(instance.Spec.ProjectID, cloudSqlInstance).Do()
	if err != nil {
		err = fmt.Errorf("failed to start insert operation for cloud sql instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return reconcile.Result{}, err
	}

	log.Printf("started insert of cloud sql instance %s: %+v", instance.Name, insertOp)
	log.Printf("sleep a bit after creating the cloud sql instance...")
	time.Sleep(5 * time.Second)

	// wait for the cloud sql instance to finish being created
	maxRetries := 50
	waitSleep := 20 * time.Second
	for i := 0; i <= maxRetries; i++ {
		cloudSqlInstance, err = cloudSqlClient.Instances.Get(instance.Spec.ProjectID, instance.Name).Do()
		if err == nil {
			// cloud sql instance has been created, update the CRD status now
			if err := r.updateStatus(instance, "cloud sql instance successfully created", cloudSqlInstance); err != nil {
				// updating the CRD status failed, return the error and try the next reconcile loop
				err = fmt.Errorf("failed to update status for cloud sql instance %s: %+v", instance.Name, err)
				log.Printf("%+v", err)
				return reconcile.Result{}, err
			}

			if cloudSqlInstance.State == "RUNNABLE" {
				// CRD status updated and cloud sql instance in RUNNABLE state, we are all good
				log.Printf("cloud sql instance %s successfully created and in the RUNNABLE state. %+v", instance.Name, cloudSqlInstance)
				return reconcile.Result{}, nil
			}

			log.Printf("cloud sql instance %s created but still in %s state, waiting %v", instance.Name, cloudSqlInstance.State, waitSleep)
		} else {
			log.Printf("failed to get cloud sql instance %s, waiting %v: %+v", instance.Name, waitSleep, err)
		}

		<-time.After(waitSleep)
	}

	// the retry loop completed without finding the created cloud sql instance, report this as an error
	err = fmt.Errorf("gave up waiting for cloud sql instance %s: %+v", instance.Name, err)
	log.Printf("%+v", err)
	return reconcile.Result{}, err
}

func (r *ReconcileCloudSql) cloudSqlClient() (*sqladmin.Service, error) {
	service, err := sqladmin.New(r.gcpHTTPClient)
	if err != nil {
		return nil, fmt.Errorf("failed ot get CloudSql client: %+v", err)
	}

	return service, nil
}

func (r *ReconcileCloudSql) updateStatus(instance *gcpv1alpha1.CloudSql, message string, cloudSqlInstance *sqladmin.DatabaseInstance) error {
	instance.Status = gcpv1alpha1.CloudSqlStatus{
		Message:    message,
		State:      cloudSqlInstance.State,
		ProviderID: cloudSqlInstance.SelfLink,
	}
	err := r.Update(context.TODO(), instance)
	if err != nil {
		err = fmt.Errorf("failed to update status of CRD instance %s: %+v", instance.Name, err)
		log.Printf("%+v", err)
		return err
	}

	return nil
}
