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

package securitygroup

import (
	"context"
	"fmt"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	awscomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/clients/aws"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/ec2"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
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
	controllerName = "securitygroup.compute.aws.crossplane.io"
	finalizer      = "finalizer." + controllerName

	errorCreateSecurityGroup = "Failed to create new security groups"
	errorSyncSecurityGroup   = "Failed to sync security group"
	errorDeleteSecurityGroup = "Failed to delete security group"
)

var (
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
)

// Add creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))

}

// Reconciler reconciles a Provider object
type Reconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	connect func(*awscomputev1alpha1.SecurityGroup) (ec2.Client, error)
	create  func(*awscomputev1alpha1.SecurityGroup, ec2.Client) (reconcile.Result, error)
	sync    func(*awscomputev1alpha1.SecurityGroup, ec2.Client) (reconcile.Result, error)
	delete  func(*awscomputev1alpha1.SecurityGroup, ec2.Client) (reconcile.Result, error)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(controllerName),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete
	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &awscomputev1alpha1.SecurityGroup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *awscomputev1alpha1.SecurityGroup, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *Reconciler) _connect(instance *awscomputev1alpha1.SecurityGroup) (ec2.Client, error) {
	// Fetch Provider
	p := &awsv1alpha1.Provider{}
	providerNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ProviderRef.Name,
	}
	err := r.Get(ctx, providerNamespacedName, p)
	if err != nil {
		return nil, err
	}

	// Check provider status
	if !p.IsValid() {
		return nil, fmt.Errorf("provider status is invalid")
	}

	// Get Provider's AWS Config
	config, err := aws.Config(r.kubeclient, p)
	if err != nil {
		return nil, err
	}

	config.Region = instance.Spec.Region

	// Create new EC2 Client
	return ec2.NewClient(config), nil
}

func (r *Reconciler) _create(instance *awscomputev1alpha1.SecurityGroup, client ec2.Client) (reconcile.Result, error) {
	groupID, err := client.CreateSecurityGroup(instance.Spec.VpcID, instance.Spec.Name, instance.Spec.Description)
	if err != nil {
		return r.fail(instance, errorCreateSecurityGroup, err.Error())
	}

	instance.Spec.GroupID = *groupID

	// Update status
	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()

	return resultRequeue, r.Update(ctx, instance)
}

func (r *Reconciler) _sync(instance *awscomputev1alpha1.SecurityGroup, client ec2.Client) (reconcile.Result, error) {

	securityGroups, err := client.GetSecurityGroups([]string{instance.Spec.GroupID})
	if err != nil {
		return r.fail(instance, errorSyncSecurityGroup, err.Error())
	}

	if len(securityGroups) != 1 {
		return r.fail(instance, errorSyncSecurityGroup, "unexpected result from security group")
	}

	remotePermissions := make(map[string]awsec2.IpPermission)
	for _, perm := range securityGroups[0].IpPermissions {
		remotePermissions[awscomputev1alpha1.Key(&perm)] = perm
	}
	grants := make([]awsec2.IpPermission, 0)
	revokes := make([]awsec2.IpPermission, 0)
	for _, localPermission := range instance.Spec.IpPermissions {
		key := localPermission.Key()
		if remotePerm, ok := remotePermissions[key]; ok {
			grant, revoke := localPermission.Diff(remotePerm)
			delete(remotePermissions, key)
			if revoke != nil {
				revokes = append(revokes, *revoke)
			}
			if grant != nil {
				grants = append(grants, *grant)
			}
		} else {
			grants = append(grants, localPermission.Export())
		}
	}

	// Remove the rest of the permissions that don't exist locally.
	for _, remotePermission := range remotePermissions {
		revokes = append(revokes, remotePermission)
	}

	if len(grants) > 0 {
		if err := client.CreateIngress(instance.Spec.GroupID, grants); err != nil {
			return r.fail(instance, errorSyncSecurityGroup, err.Error())
		}
	}

	if len(revokes) > 0 {
		if err := client.RevokeIngress(instance.Spec.GroupID, revokes); err != nil {
			return r.fail(instance, errorSyncSecurityGroup, err.Error())
		}
	}

	// update resource status
	instance.Status.SetReady()

	return result, r.Update(ctx, instance)
}

// _delete check reclaim policy and if needed delete the eks cluster resource
func (r *Reconciler) _delete(instance *awscomputev1alpha1.SecurityGroup, client ec2.Client) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		// TODO: not found
		if err := client.DeleteSecurityGroup(instance.Spec.GroupID); err != nil {
			return r.fail(instance, errorDeleteSecurityGroup, err.Error())
		}
	}
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Provider instance
	instance := &awscomputev1alpha1.SecurityGroup{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Create SecurityGroup
	secClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorCreateSecurityGroup, err.Error())
	}

	// Add finalizer
	util.AddFinalizer(&instance.ObjectMeta, finalizer)

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, secClient)
	}

	// Create SecurityGroup
	if instance.Spec.GroupID == "" {
		return r.create(instance, secClient)
	}

	// Sync SecurityGroup Spec with remote security group
	return r.sync(instance, secClient)
}
