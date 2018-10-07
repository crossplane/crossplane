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
	"log"

	databasev1alpha1 "github.com/upbound/conductor/pkg/apis/aws/database/v1alpha1"
	awsv1alpha1 "github.com/upbound/conductor/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/upbound/conductor/pkg/apis/core/v1alpha1"
	"github.com/upbound/conductor/pkg/clients/aws"
	"github.com/upbound/conductor/pkg/clients/aws/rds"
	"github.com/upbound/conductor/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	recorderName = "aws.rds.instance"
	finalizer    = "finalizer.rds.aws.conductor.io"

	errorFetchingAwsProvider        = "Failed to fetch AWS Provider"
	errorProviderStatusInvalid      = "Provider status is invalid"
	errorRDSClient                  = "Failed to create RDS client"
	errorGetDbInstance              = "Failed to retrieve DBInstance"
	errorCreatingPassword           = "Failed to generate DBInstance password"
	errorRetrievingConnectionSecret = "Failed to retrieve connection secret"
	errorCreatingDbInstance         = "Failed to create DBInstance"
	errorCreatingConnectionSecret   = "Failed to create connection secret"
	errorDeletingDbInstance         = "Failed to delete DBInstance"

	connectionSecretUsernameKey = "Username"
	connectionSecretPasswordKey = "Password"
	connectionSecretEndpointKey = "Endpoint"
)

var (
	_ reconcile.Reconciler = &Reconciler{}
)

// Add creates a new Instance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(recorderName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("instance-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Instance
	err = c.Watch(&source.Kind{Type: &databasev1alpha1.RDSInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to Instance Secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &databasev1alpha1.RDSInstance{},
	})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *databasev1alpha1.RDSInstance, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return reconcile.Result{Requeue: true}, r.Update(context.TODO(), instance)
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CRD instance
	instance := &databasev1alpha1.RDSInstance{}
	ctx := context.Background()

	err := r.Get(ctx, request.NamespacedName, instance)
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

	// Generate DBInstance Name
	if instance.Status.InstanceName == "" {
		instance.Status.InstanceName = instance.Name + "-" + string(instance.UID)
	}

	// Fetch AWS Provider
	p := &awsv1alpha1.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err = r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return r.fail(instance, errorFetchingAwsProvider, err.Error())
	}

	// Check provider status
	if pc := p.Status.GetCondition(corev1alpha1.Invalid); pc != nil && pc.Status == corev1.ConditionTrue {
		return r.fail(instance, errorProviderStatusInvalid, pc.Reason)
	}

	// Create new RDS Client
	svc, err := RDSService(p, r.kubeclient)
	if err != nil {
		return r.fail(instance, errorRDSClient, err.Error())
	}

	// Search for the RDS instance in AWS
	db, err := svc.GetInstance(instance.Status.InstanceName)
	if err != nil {
		return r.fail(instance, errorGetDbInstance, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil && instance.Status.GetCondition(corev1alpha1.Deleting) == nil {
		if _, err = svc.DeleteInstance(instance.Status.InstanceName); err != nil {
			return r.fail(instance, errorDeletingDbInstance, err.Error())
		}

		instance.Status.SetCondition(*corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
		util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
		return reconcile.Result{}, r.Update(ctx, instance)
	}

	// Add finalizer
	if !util.HasFinalizer(&instance.ObjectMeta, finalizer) {
		util.AddFinalizer(&instance.ObjectMeta, finalizer)
		if err := r.Update(ctx, instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Next 3 steps need to be performed all together
	// 1. Generate Password
	// 2. Save password into connection secret
	// 3. Create new database with the generated password
	if db == nil {
		// generate new password
		password, err := util.GeneratePassword(20)
		if err != nil {
			return r.fail(instance, errorCreatingPassword, err.Error())
		}

		_, err = r.ApplyConnectionSecret(NewConnectionSecret(instance, password))
		if err != nil {
			return r.fail(instance, errorCreatingConnectionSecret, err.Error())
		}

		// Create DB Instance
		db, err = svc.CreateInstance(instance.Status.InstanceName, password, &instance.Spec)
		if err != nil {
			return r.fail(instance, errorCreatingDbInstance, err.Error())
		}
	} else {
		// Search for connection secret
		connSecret, err := r.kubeclient.CoreV1().Secrets(instance.Namespace).Get(instance.Spec.ConnectionSecretRef.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// There is nothing we can do to recover connect secret password.
				// Nothing, short of re-creating the database instance.
				// TODO: come up with better database instance password persistence
				return r.fail(instance, errorRetrievingConnectionSecret, err.Error())
			} else {
				return r.fail(instance, errorRetrievingConnectionSecret, err.Error())
			}
		}
		connSecret.Data[connectionSecretEndpointKey] = []byte(db.Endpoint)
		_, err = r.ApplyConnectionSecret(connSecret)
		if err != nil {
			return r.fail(instance, errorCreatingConnectionSecret, err.Error())
		}
	}

	// Update status - if changed
	conditionType := rds.ConditionType(db.Status)
	requeue := conditionType != corev1alpha1.Running

	if instance.Status.State != db.Status {
		instance.Status.State = db.Status
		instance.Status.ProviderID = db.ARN
		instance.Status.UnsetAllConditions()

		condition := corev1alpha1.NewCondition(conditionType, "", "")
		instance.Status.SetCondition(*condition)

		// Requeue this request until database is in Running state
		return reconcile.Result{Requeue: requeue}, r.Update(ctx, instance)
	}

	return reconcile.Result{Requeue: requeue}, nil
}

// RDSService - creates new instance of RDS client based on provider information
var RDSService = func(p *awsv1alpha1.Provider, k kubernetes.Interface) (rds.Service, error) {
	// Get Provider's AWS Config
	config, err := aws.Config(p, k)
	if err != nil {
		return nil, err
	}

	// Create new RDS Client
	return rds.NewClient(config), nil
}

func NewConnectionSecret(instance *databasev1alpha1.RDSInstance, password string) *corev1.Secret {
	if instance.APIVersion == "" {
		instance.APIVersion = "database.aws.conductor.io/v1alpha1"
	}
	if instance.Kind == "" {
		instance.Kind = "RDSInstance"
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.ConnectionSecretRef.Name,
			Namespace: instance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: instance.APIVersion,
					Kind:       instance.Kind,
					Name:       instance.Name,
					UID:        instance.UID,
				},
			},
		},

		Data: map[string][]byte{
			connectionSecretUsernameKey: []byte(instance.Spec.MasterUsername),
			connectionSecretPasswordKey: []byte(password),
		},
	}
}

func (r *Reconciler) ApplyConnectionSecret(s *corev1.Secret) (*corev1.Secret, error) {
	_, err := r.kubeclient.CoreV1().Secrets(s.Namespace).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return r.kubeclient.CoreV1().Secrets(s.Namespace).Create(s)
		}
		return nil, err
	}
	return r.kubeclient.CoreV1().Secrets(s.Namespace).Update(s)
}
