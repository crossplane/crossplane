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

package compute

import (
	"context"
	"fmt"
	awscomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	awsClient "github.com/crossplaneio/crossplane/pkg/clients/aws"
	cloudformationclient "github.com/crossplaneio/crossplane/pkg/clients/aws/cloudformation"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
	"github.com/crossplaneio/crossplane/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	nodepoolControllerName = "eksnodepool.compute.aws.crossplane.io"
	nodepoolFinalizer      = "finalizer." + nodepoolControllerName

	errorClusterReferenceNotFound = "Failed to get ClusterRef"
	errorClusterReferenceWait     = "Waiting for ClusterRef Ready state"
)

// AddNodePoolReconciler creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddNodePoolReconciler(mgr manager.Manager) error {
	return addNodeReconciler(mgr, newNodeReconciler(mgr))
}

// NodeReconciler reconciles an EKSNodePool object
type NodeReconciler struct {
	client.Client
	scheme     *runtime.Scheme
	kubeclient kubernetes.Interface
	recorder   record.EventRecorder

	connect func(pool *awscomputev1alpha1.EKSNodePool) (eks.Client, error)
	create  func(*awscomputev1alpha1.EKSCluster, *awscomputev1alpha1.EKSNodePool, eks.Client) (reconcile.Result, error)
	sync    func(*awscomputev1alpha1.EKSNodePool, eks.Client) (reconcile.Result, error)
	delete  func(*awscomputev1alpha1.EKSNodePool, eks.Client) (reconcile.Result, error)
	awsauth func(*eks.Cluster, *awscomputev1alpha1.EKSCluster, eks.Client, map[string]string) error
}

// newReconciler returns a new reconcile.Reconciler
func newNodeReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &NodeReconciler{
		Client:     mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		kubeclient: kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:   mgr.GetRecorder(nodepoolControllerName),
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete

	return r
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func addNodeReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(nodepoolControllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Provider
	err = c.Watch(&source.Kind{Type: &awscomputev1alpha1.EKSNodePool{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// On events from Cluster -> reconcile related node pools
	err = c.Watch(
		&source.Kind{Type: &awscomputev1alpha1.EKSCluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &nodePoolMapper{},
		})
	if err != nil {
		return err
	}

	return nil
}

type nodePoolMapper struct {
}

func (m *nodePoolMapper) Map(mapObject handler.MapObject) []reconcile.Request {
	var result []reconcile.Request

	cluster, ok := mapObject.Object.(*awscomputev1alpha1.EKSCluster)
	if !ok {
		log.Printf("failed to cast to runtimeObject to EKSCluster: %+v", mapObject)
		return result
	}

	namespace := mapObject.Meta.GetNamespace()
	// TODO: remove client and figure out how to convert runtime.Object to actual type.
	//cluster := &awscomputev1alpha1.EKSCluster{}
	//clusterNamespacedName := types.NamespacedName{
	//	Namespace: namespace,
	//	Name:      mapObject.Meta.GetName(),
	//}
	//err := m.Get(ctx, clusterNamespacedName, cluster)
	//if err != nil {
	//	return result
	//}

	// If Cluster.IsReady() -> notify it's nodePools to start creating.
	// TODO: check that this just happened.
	if cluster.Status.IsReady() {
		for _, nodePool := range cluster.Spec.NodePools {
			req := reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      nodePool.Name,
				Namespace: namespace,
			}}
			result = append(result, req)
		}
	}

	return result
}

// fail - helper function to set fail condition with reason and message
func (r *NodeReconciler) fail(instance *awscomputev1alpha1.EKSNodePool, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *NodeReconciler) _connect(instance *awscomputev1alpha1.EKSNodePool) (eks.Client, error) {
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
	config, err := awsClient.Config(r.kubeclient, p)
	if err != nil {
		return nil, err
	}

	// Connection Region must be with Spec.Region
	if string(instance.Spec.Region) != config.Region {
		config.Region = string(instance.Spec.Region)
	}

	// Create new EKS Client
	return eks.NewClient(config), nil
}

func (r *NodeReconciler) _create(cluster *awscomputev1alpha1.EKSCluster, instance *awscomputev1alpha1.EKSNodePool, client eks.Client) (reconcile.Result, error) {
	// Requirement of matching names for nodepool to connect to master.
	instance.ClusterName = cluster.Status.ClusterName

	if instance.Status.CloudFormationStackID == "" {
		clusterWorkers, err := client.CreateWorkerNodes(instance.Name, cluster.Spec, instance.Spec)
		if err != nil {
			return r.fail(instance, errorSyncCluster, err.Error())
		}
		instance.Status.CloudFormationStackID = clusterWorkers.WorkerStackID
		return resultRequeue, r.Update(ctx, instance)
	}

	// Update status
	instance.Status.State = awscomputev1alpha1.NodePoolStatusCreating
	instance.Status.UnsetAllConditions()
	instance.Status.SetCreating()

	return resultRequeue, r.Update(ctx, instance)
}

func (r *NodeReconciler) getEKSCluster(instance *awscomputev1alpha1.EKSNodePool) (*awscomputev1alpha1.EKSCluster, error) {
	cluster := &awscomputev1alpha1.EKSCluster{}
	clusterNamespacedName := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.ClusterRef.Name,
	}
	err := r.Get(ctx, clusterNamespacedName, cluster)
	return cluster, err
}

func (r *NodeReconciler) _sync(instance *awscomputev1alpha1.EKSNodePool, client eks.Client) (reconcile.Result, error) {
	clusterWorker, err := client.GetWorkerNodes(instance.Status.CloudFormationStackID)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	if !cloudformationclient.IsCompletedState(clusterWorker.WorkersStatus) {
		return resultRequeue, r.Update(ctx, instance)
	}

	instance.Status.NodeInstanceRoleARN = clusterWorker.WorkerARN
	instance.Status.State = awscomputev1alpha1.NodePoolStatusActive
	instance.Status.SetReady()

	return result, r.Update(ctx, instance)
}

// _delete check reclaim policy and if needed delete the eks cluster resource
func (r *NodeReconciler) _delete(instance *awscomputev1alpha1.EKSNodePool, client eks.Client) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		if instance.Status.CloudFormationStackID != "" {
			if err := client.DeleteWorkerNodes(instance.Status.CloudFormationStackID); err != nil && !cloudformationclient.IsErrorNotFound(err) {
				return r.fail(instance, errorDeleteCluster, fmt.Sprintf("worker Delete Error: %s", err.Error()))
			}
		}
	}

	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *NodeReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Provider instance
	instance := &awscomputev1alpha1.EKSNodePool{}
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

	// Create EKS Client
	eksClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorClusterClient, err.Error())
	}

	// Add finalizer
	util.AddFinalizer(&instance.ObjectMeta, nodepoolFinalizer)

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, eksClient)
	}

	// Create nodepool when EKSCluster is in Ready() state.
	// Creating too early means the nodes won't connect.
	// Assuming there is tls certs that get copied into nodepool on init.
	if instance.Status.CloudFormationStackID == "" {
		cluster, err := r.getEKSCluster(instance)
		if err != nil {
			return r.fail(instance, errorClusterReferenceNotFound, "could not fetch the cluster reference. Can not create nodepool")
		}

		if err := controllerutil.SetControllerReference(cluster, instance, r.scheme); err != nil {
			return result, err
		}

		// If master EKS node IsReady we provision the node pool
		if cluster.Status.IsReady() {
			return r.create(cluster, instance, eksClient)
		}

		// We sleep until EKSCluster requeues when EKSCluster is ready.
		return result, nil
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, eksClient)
}
