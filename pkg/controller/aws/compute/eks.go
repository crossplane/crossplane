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
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	awscomputev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/compute/v1alpha1"
	awsv1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/aws/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	awsClient "github.com/crossplaneio/crossplane/pkg/clients/aws"
	cloudformationclient "github.com/crossplaneio/crossplane/pkg/clients/aws/cloudformation"
	"github.com/crossplaneio/crossplane/pkg/clients/aws/eks"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName    = "eks.compute.aws.crossplane.io"
	finalizer         = "finalizer." + controllerName
	clusterNamePrefix = "eks-"

	errorClusterClient   = "Failed to create cluster client"
	errorCreateCluster   = "Failed to create new cluster"
	errorSyncCluster     = "Failed to sync cluster state"
	errorDeleteCluster   = "Failed to delete cluster"
	eksAuthConfigMapName = "aws-auth"
	eksAuthMapRolesKey   = "mapRoles"
	eksAuthMapUsersKey   = "mapUsers"
)

var (
	log           = logging.Logger.WithName("controller." + controllerName)
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

	connect func(*awscomputev1alpha1.EKSCluster) (eks.Client, error)
	create  func(*awscomputev1alpha1.EKSCluster, eks.Client) (reconcile.Result, error)
	sync    func(*awscomputev1alpha1.EKSCluster, eks.Client) (reconcile.Result, error)
	delete  func(*awscomputev1alpha1.EKSCluster, eks.Client) (reconcile.Result, error)
	secret  func(*eks.Cluster, *awscomputev1alpha1.EKSCluster, eks.Client) error
	awsauth func(*eks.Cluster, *awscomputev1alpha1.EKSCluster, eks.Client, string) error
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
	r.secret = r._secret
	r.awsauth = r._awsauth
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
	err = c.Watch(&source.Kind{Type: &awscomputev1alpha1.EKSCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *awscomputev1alpha1.EKSCluster, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetFailed(reason, msg)
	return resultRequeue, r.Update(context.TODO(), instance)
}

func (r *Reconciler) _connect(instance *awscomputev1alpha1.EKSCluster) (eks.Client, error) {
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

func (r *Reconciler) _create(instance *awscomputev1alpha1.EKSCluster, client eks.Client) (reconcile.Result, error) {
	clusterName := fmt.Sprintf("%s%s", clusterNamePrefix, instance.UID)

	// Create Master
	_, err := client.Create(clusterName, instance.Spec)
	if err != nil && !eks.IsErrorAlreadyExists(err) {
		if eks.IsErrorBadRequest(err) {
			instance.Status.SetFailed(errorCreateCluster, err.Error())
			// do not requeue on bad requests
			return result, r.Update(ctx, instance)
		}
		return r.fail(instance, errorCreateCluster, err.Error())
	}

	// Update status
	instance.Status.State = awscomputev1alpha1.ClusterStatusCreating
	instance.Status.UnsetAllDeprecatedConditions()
	instance.Status.SetCreating()
	instance.Status.ClusterName = clusterName

	return resultRequeue, r.Update(ctx, instance)
}

// generateAWSAuthConfigMap generates the configmap for configure auth
func generateAWSAuthConfigMap(instance *awscomputev1alpha1.EKSCluster, workerARN string) (*v1.ConfigMap, error) {
	data := map[string]string{}
	defaultRole := awscomputev1alpha1.MapRole{
		RoleARN:  workerARN,
		Username: "system:node:{{EC2PrivateDNSName}}",
		Groups:   []string{"system:bootstrappers", "system:nodes"},
	}

	// Serialize mapRoles
	roles := make([]awscomputev1alpha1.MapRole, len(instance.Spec.MapRoles))
	copy(roles, instance.Spec.MapRoles)
	roles = append(roles, defaultRole)

	rolesMarshalled, err := yaml.Marshal(roles)
	if err != nil {
		return nil, err
	}

	data[eksAuthMapRolesKey] = string(rolesMarshalled)

	// Serialize mapUsers
	if len(instance.Spec.MapUsers) > 0 {
		usersMarshalled, err := yaml.Marshal(instance.Spec.MapUsers)
		if err != nil {
			return nil, err
		}
		data[eksAuthMapUsersKey] = string(usersMarshalled)
	}

	name := eksAuthConfigMapName
	namespace := "kube-system"
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	return &cm, nil
}

// _awsauth generates an aws-auth configmap and pushes it to the remote eks cluster to configure auth
func (r *Reconciler) _awsauth(cluster *eks.Cluster, instance *awscomputev1alpha1.EKSCluster, client eks.Client, workerARN string) error {
	cm, err := generateAWSAuthConfigMap(instance, workerARN)
	if err != nil {
		return err
	}

	// Sync aws-auth to remote eks cluster to configure it's auth.
	token, err := client.ConnectionToken(instance.Status.ClusterName)
	if err != nil {
		return err
	}

	// Client to eks cluster
	caData, err := base64.StdEncoding.DecodeString(cluster.CA)
	if err != nil {
		return err
	}

	c := rest.Config{
		Host: cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
		},
		BearerToken: token,
	}

	clientset, err := kubernetes.NewForConfig(&c)
	if err != nil {
		return err
	}

	// Create or update aws-auth configmap on eks cluster
	_, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Create(cm)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = clientset.CoreV1().ConfigMaps(cm.Namespace).Update(cm)
		}
	}

	return err
}

func (r *Reconciler) _sync(instance *awscomputev1alpha1.EKSCluster, client eks.Client) (reconcile.Result, error) {
	cluster, err := client.Get(instance.Status.ClusterName)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	if cluster.Status != awscomputev1alpha1.ClusterStatusActive {
		return resultRequeue, nil
	}

	// Create workers
	if instance.Status.CloudFormationStackID == "" {
		clusterWorkers, err := client.CreateWorkerNodes(instance.Status.ClusterName, instance.Spec)
		if err != nil {
			return r.fail(instance, errorSyncCluster, err.Error())
		}
		instance.Status.CloudFormationStackID = clusterWorkers.WorkerStackID
		return resultRequeue, r.Update(ctx, instance)
	}

	clusterWorker, err := client.GetWorkerNodes(instance.Status.CloudFormationStackID)
	if err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	if !cloudformationclient.IsCompletedState(clusterWorker.WorkersStatus) {
		return resultRequeue, r.Update(ctx, instance)
	}

	if err := r.awsauth(cluster, instance, client, clusterWorker.WorkerARN); err != nil {
		return r.fail(instance, errorSyncCluster, fmt.Sprintf("failed to set auth map on eks: %s", err.Error()))
	}

	if err := r.secret(cluster, instance, client); err != nil {
		return r.fail(instance, errorSyncCluster, err.Error())
	}

	// update resource status
	instance.Status.Endpoint = cluster.Endpoint
	instance.Status.State = awscomputev1alpha1.ClusterStatusActive
	instance.Status.SetReady()

	return result, r.Update(ctx, instance)
}

func (r *Reconciler) _secret(cluster *eks.Cluster, instance *awscomputev1alpha1.EKSCluster, client eks.Client) error {
	token, err := client.ConnectionToken(instance.Status.ClusterName)
	if err != nil {
		return err
	}

	// Avoid double base64 encoding on secret
	caData, err := base64.StdEncoding.DecodeString(cluster.CA)
	if err != nil {
		return err
	}

	secret := instance.ConnectionSecret()
	data := make(map[string][]byte)
	data[corev1alpha1.ResourceCredentialsSecretEndpointKey] = []byte(cluster.Endpoint)
	data[corev1alpha1.ResourceCredentialsSecretCAKey] = caData
	data[corev1alpha1.ResourceCredentialsTokenKey] = []byte(token)
	secret.Data = data

	// create connection secret
	if _, err := util.ApplySecret(r.kubeclient, secret); err != nil {
		return err
	}

	// Set secret reference
	instance.SetConnectionSecretReference(secret)

	return nil
}

// _delete check reclaim policy and if needed delete the eks cluster resource
func (r *Reconciler) _delete(instance *awscomputev1alpha1.EKSCluster, client eks.Client) (reconcile.Result, error) {
	if instance.Spec.ReclaimPolicy == corev1alpha1.ReclaimDelete {
		var deleteErrors []string
		if err := client.Delete(instance.Status.ClusterName); err != nil && !eks.IsErrorNotFound(err) {
			deleteErrors = append(deleteErrors, fmt.Sprintf("Master Delete Error: %s", err.Error()))
		}

		if instance.Status.CloudFormationStackID != "" {
			if err := client.DeleteWorkerNodes(instance.Status.CloudFormationStackID); err != nil && !cloudformationclient.IsErrorNotFound(err) {
				deleteErrors = append(deleteErrors, fmt.Sprintf("Worker Delete Error: %s", err.Error()))
			}
		}

		if len(deleteErrors) > 0 {
			return r.fail(instance, errorDeleteCluster, strings.Join(deleteErrors, ", "))
		}
	}

	meta.RemoveFinalizer(instance, finalizer)
	instance.Status.SetDeleting()
	return result, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Provider object and makes changes based on the state read
// and what is in the Provider.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", awscomputev1alpha1.EKSClusterKindAPIVersion, "request", request)
	// Fetch the Provider instance
	instance := &awscomputev1alpha1.EKSCluster{}
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
	meta.AddFinalizer(instance, finalizer)

	// Check for deletion
	if instance.DeletionTimestamp != nil {
		return r.delete(instance, eksClient)
	}

	// Create cluster instance
	if instance.Status.ClusterName == "" {
		return r.create(instance, eksClient)
	}

	// Sync cluster instance status with cluster status
	return r.sync(instance, eksClient)
}
