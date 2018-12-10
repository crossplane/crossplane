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

package workload

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectl "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName = "workload.compute.crossplane.io"
	finalizer      = "finalizer." + controllerName

	errorClusterClient = "Failed to create cluster client"
	errorCreating      = "Failed to create"
	errorSynchronizing = "Failed to sync"
	errorDeleting      = "Failed to delete"
)

var (
	ctx           = context.Background()
	result        = reconcile.Result{}
	resultRequeue = reconcile.Result{Requeue: true}
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

	connect func(*computev1alpha1.Workload) (kubernetes.Interface, error)
	create  func(*computev1alpha1.Workload, kubernetes.Interface) (reconcile.Result, error)
	sync    func(*computev1alpha1.Workload, kubernetes.Interface) (reconcile.Result, error)
	delete  func(*computev1alpha1.Workload, kubernetes.Interface) (reconcile.Result, error)
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

	// Watch for changes to Instance
	err = c.Watch(&source.Kind{Type: &computev1alpha1.Workload{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *computev1alpha1.Workload, reason, msg string) (reconcile.Result, error) {
	log.Printf("%s: %s", reason, msg)
	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Failed, reason, msg))
	return resultRequeue, r.Update(ctx, instance)
}

// _connect establish connection to the target cluster
func (r *Reconciler) _connect(instance *computev1alpha1.Workload) (kubernetes.Interface, error) {
	ref := instance.Spec.TargetCluster

	k := &computev1alpha1.KubernetesCluster{}

	err := r.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, k)
	if err != nil {
		return nil, err
	}

	s, err := r.kubeclient.CoreV1().Secrets(k.Namespace).Get(k.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if kubeconfig, ok := s.Data[corev1alpha1.ResourceCredentialsSecretKubeconfigFileKey]; ok {
		// we have a full kubeconfig, just load that in its entirety
		config, err := getRestConfigFromKubeconfig(kubeconfig)
		if err != nil {
			return nil, err
		}

		return kubernetes.NewForConfig(config)
	}

	// read the individual connection config fields
	host, ok := s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]
	if !ok {
		return nil, fmt.Errorf("kubernetes cluster endpoint/host is not found")
	}
	hostName := string(host)
	if !strings.HasSuffix(hostName, ":443") {
		hostName = hostName + ":443"
	}

	user, _ := s.Data[corev1alpha1.ResourceCredentialsSecretUserKey]
	pass, _ := s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]
	ca, _ := s.Data[corev1alpha1.ResourceCredentialsSecretCAKey]
	cert, _ := s.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey]
	key, _ := s.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey]
	token, _ := s.Data[corev1alpha1.ResourceCredentialsTokenKey]

	config := &rest.Config{
		Host:     hostName,
		Username: string(user),
		Password: string(pass),
		TLSClientConfig: rest.TLSClientConfig{
			ServerName: "kubernetes",
			CAData:     ca,
			CertData:   cert,
			KeyData:    key,
		},
		BearerToken: string(token),
	}

	return kubernetes.NewForConfig(config)
}

// _create workload
func (r *Reconciler) _create(instance *computev1alpha1.Workload, client kubernetes.Interface) (reconcile.Result, error) {
	instance.Status.SetCreating()
	util.AddFinalizer(&instance.ObjectMeta, finalizer)

	// create target namespace
	targetNamespace := instance.Spec.TargetNamespace

	_, err := client.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: targetNamespace}})
	if err != nil && !errors.IsAlreadyExists(err) {
		return r.fail(instance, errorCreating, err.Error())
	}

	// propagate resources secrets
	for _, resource := range instance.Spec.Resources {
		// retrieve secret
		secretName := util.IfEmptyString(resource.SecretName, resource.Name)
		sec, err := r.kubeclient.CoreV1().Secrets(instance.Namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return r.fail(instance, errorCreating, err.Error())
		}

		// create secret
		sec.ObjectMeta = metav1.ObjectMeta{
			Name:      sec.Name,
			Namespace: targetNamespace,
		}
		_, err = util.ApplySecret(client, sec)
		if err != nil {
			return r.fail(instance, errorCreating, err.Error())
		}
	}

	// propagate deployment
	d := instance.Spec.TargetDeployment
	d.Spec.Selector.MatchLabels = d.Spec.Template.Labels
	d.Namespace = util.IfEmptyString(d.Namespace, targetNamespace)
	_, err = util.ApplyDeployment(client, d)
	if err != nil {
		return r.fail(instance, errorCreating, err.Error())
	}

	// propagate service
	s := instance.Spec.TargetService
	s.Namespace = util.IfEmptyString(s.Namespace, targetNamespace)
	_, err = util.ApplyService(client, s)
	if err != nil {
		return r.fail(instance, errorCreating, err.Error())
	}

	instance.Status.State = computev1alpha1.WorkloadStateCreating

	// update instance
	return result, r.Update(ctx, instance)
}

// _sync Workload status
func (r *Reconciler) _sync(instance *computev1alpha1.Workload, client kubernetes.Interface) (reconcile.Result, error) {
	ns := instance.Spec.TargetNamespace

	s := instance.Spec.TargetService
	ss, err := client.CoreV1().Services(ns).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		return r.fail(instance, errorSynchronizing, err.Error())
	}
	instance.Status.ServiceStatus = ss.Status

	d := instance.Spec.TargetDeployment
	dd, err := client.AppsV1().Deployments(ns).Get(d.Name, metav1.GetOptions{})
	if err != nil {
		return r.fail(instance, errorSynchronizing, err.Error())
	}
	instance.Status.DeploymentStatus = dd.Status

	// TODO: decide how to better determine the workload status
	if util.LatestDeploymentCondition(dd.Status.Conditions).Type == appsv1.DeploymentAvailable {
		instance.Status.State = computev1alpha1.WorkloadStateRunning
		instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Ready, "", ""))
		return result, r.Update(ctx, instance)
	}

	return resultRequeue, r.Update(ctx, instance)
}

// _delete workload
func (r *Reconciler) _delete(instance *computev1alpha1.Workload, client kubernetes.Interface) (reconcile.Result, error) {
	ns := instance.Spec.TargetNamespace

	// delete service
	err := client.CoreV1().Services(ns).Delete(instance.Spec.TargetService.Name, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return r.fail(instance, errorDeleting, err.Error())
	}

	// delete deployment
	err = client.AppsV1().Deployments(ns).Delete(instance.Spec.TargetDeployment.Name, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return r.fail(instance, errorDeleting, err.Error())
	}

	// delete resources secrets
	for _, resource := range instance.Spec.Resources {
		secretName := util.IfEmptyString(resource.SecretName, resource.Name)
		if err := client.CoreV1().Secrets(ns).Delete(secretName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return r.fail(instance, errorDeleting, err.Error())
		}
	}

	instance.Status.SetCondition(corev1alpha1.NewCondition(corev1alpha1.Deleting, "", ""))
	util.RemoveFinalizer(&instance.ObjectMeta, finalizer)
	return reconcile.Result{}, r.Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// fetch the CRD instance
	instance := &computev1alpha1.Workload{}

	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return result, nil
		}
		return result, err
	}

	// target cluster client
	targetClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorClusterClient, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil && instance.Status.Condition(corev1alpha1.Deleting) == nil {
		return r.delete(instance, targetClient)
	}

	// Check if target cluster is assigned
	if instance.Status.State == "" {
		return r.create(instance, targetClient)
	}

	// sync the resource
	return r.sync(instance, targetClient)
}

// getRestConfigFromKubeconfig converts the given raw kubeconfig into a restful config
func getRestConfigFromKubeconfig(kubeconfig []byte) (*rest.Config, error) {
	// open a temp file that we'll write the raw kubeconfig data to
	kubeconfigTempFile, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeconfigTempFile.Name())

	// write the raw data to the temp file, then close the file
	if _, err := kubeconfigTempFile.Write(kubeconfig); err != nil {
		return nil, err
	}
	if err := kubeconfigTempFile.Close(); err != nil {
		return nil, err
	}

	return kubectl.BuildConfigFromFlags("", kubeconfigTempFile.Name())
}
