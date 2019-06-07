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
	"net/url"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	computev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/compute/v1alpha1"
	corev1alpha1 "github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName = "workload.compute.crossplane.io"
	finalizer      = "finalizer." + controllerName

	errorClusterClient = "Failed to create cluster client"
	errorCreating      = "Failed to create"
	errorSynchronizing = "Failed to sync"
	errorDeleting      = "Failed to delete"

	workloadReferenceLabelKey = "workloadRef"
)

var (
	log           = logging.Logger.WithName("controller." + controllerName)
	ctx           = context.Background()
	resultDone    = reconcile.Result{}
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

	propagateDeployment func(kubernetes.Interface, *appsv1.Deployment, string, string) (*appsv1.Deployment, error)
	propagateService    func(kubernetes.Interface, *corev1.Service, string, string) (*corev1.Service, error)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	r := &Reconciler{
		Client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		kubeclient:          kubernetes.NewForConfigOrDie(mgr.GetConfig()),
		recorder:            mgr.GetRecorder(controllerName),
		propagateDeployment: propagateDeployment,
		propagateService:    propagateService,
	}
	r.connect = r._connect
	r.create = r._create
	r.sync = r._sync
	r.delete = r._delete

	return r
}

// CreatePredicate accepts Workload instances with set `Status.Cluster` reference value
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*computev1alpha1.Workload)
	if !ok {
		return false
	}
	return wl.Status.Cluster != nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Instance
	err = c.Watch(&source.Kind{Type: &computev1alpha1.Workload{}}, &handler.EnqueueRequestForObject{}, &predicate.Funcs{CreateFunc: CreatePredicate})
	if err != nil {
		return err
	}

	return nil
}

// fail - helper function to set fail condition with reason and message
func (r *Reconciler) fail(instance *computev1alpha1.Workload, reason, msg string) (reconcile.Result, error) {
	instance.Status.SetDeprecatedCondition(corev1alpha1.NewDeprecatedCondition(corev1alpha1.DeprecatedFailed, reason, msg))
	return resultRequeue, r.Status().Update(ctx, instance)
}

// _connect establish connection to the target cluster
func (r *Reconciler) _connect(instance *computev1alpha1.Workload) (kubernetes.Interface, error) {
	ref := instance.Status.Cluster
	if ref == nil {
		return nil, fmt.Errorf("workload is not scheduled")
	}

	k := &computev1alpha1.KubernetesCluster{}

	err := r.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, k)
	if err != nil {
		return nil, err
	}

	s, err := r.kubeclient.CoreV1().Secrets(k.Namespace).Get(k.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// read the individual connection config fields
	user := s.Data[corev1alpha1.ResourceCredentialsSecretUserKey]
	pass := s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]
	ca := s.Data[corev1alpha1.ResourceCredentialsSecretCAKey]
	cert := s.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey]
	key := s.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey]
	token := s.Data[corev1alpha1.ResourceCredentialsTokenKey]
	host, ok := s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]
	if !ok {
		return nil, fmt.Errorf("kubernetes cluster endpoint/host is not found")
	}
	u, err := url.Parse(string(host))
	if err != nil {
		return nil, fmt.Errorf("cannot parse Kubernetes endpoint as URL: %+v", err)
	}

	config := &rest.Config{
		Host:     u.String(),
		Username: string(user),
		Password: string(pass),
		TLSClientConfig: rest.TLSClientConfig{
			// This field's godoc claims clients will use 'the hostname used to
			// contact the server' when it is left unset. In practice clients
			// appear to use the URL, including scheme and port.
			ServerName: u.Hostname(),
			CAData:     ca,
			CertData:   cert,
			KeyData:    key,
		},
		BearerToken: string(token),
	}

	return kubernetes.NewForConfig(config)
}

func addWorkloadReferenceLabel(m *metav1.ObjectMeta, uid string) {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[workloadReferenceLabelKey] = uid
}

func getWorkloadReferenceLabel(m metav1.ObjectMeta) string {
	if m.Labels == nil {
		return ""
	}
	return m.Labels[workloadReferenceLabelKey]
}

// propagateDeployment to the target cluster
func propagateDeployment(k kubernetes.Interface, d *appsv1.Deployment, ns, uid string) (*appsv1.Deployment, error) {
	// Update deployment selector - typically selector value is not provided and if it is not
	// matching template the deployment create operation will fail
	if d.Spec.Selector == nil {
		d.Spec.Selector = &metav1.LabelSelector{}
	}
	d.Spec.Selector.MatchLabels = d.Spec.Template.Labels

	// If deployment namespace value is not provided - default it to the workload target namespace
	d.Namespace = util.IfEmptyString(d.Namespace, ns)

	addWorkloadReferenceLabel(&d.ObjectMeta, uid)

	// Check if target deployment already exists on the target cluster
	dd, err := k.AppsV1().Deployments(d.Namespace).Get(d.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			dd = nil
		} else {
			return nil, err
		}
	}

	if dd == nil {
		return k.AppsV1().Deployments(d.Namespace).Create(d)
	}

	if getWorkloadReferenceLabel(dd.ObjectMeta) == uid {
		return k.AppsV1().Deployments(d.Namespace).Update(d)
	}

	return nil, fmt.Errorf("cannot propagate, deployment %s/%s already exists", d.Namespace, d.Name)
}

// propagateService to the target cluster
func propagateService(k kubernetes.Interface, s *corev1.Service, ns, uid string) (*corev1.Service, error) {
	// If service namespace vlaue is not provided - default it to the workload target namespace
	s.Namespace = util.IfEmptyString(s.Namespace, ns)

	addWorkloadReferenceLabel(&s.ObjectMeta, uid)

	// check if service already exists
	ss, err := k.CoreV1().Services(s.Namespace).Get(s.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			ss = nil
		} else {
			return nil, err
		}
	}

	if ss == nil {
		return k.CoreV1().Services(s.Namespace).Create(s)
	}

	if getWorkloadReferenceLabel(ss.ObjectMeta) == uid {
		return k.CoreV1().Services(s.Namespace).Update(s)
	}

	return nil, fmt.Errorf("cannot propagate, service %s/%s already exists", s.Namespace, s.Name)
}

// _create workload
func (r *Reconciler) _create(instance *computev1alpha1.Workload, client kubernetes.Interface) (reconcile.Result, error) {
	instance.Status.SetCreating()
	meta.AddFinalizer(instance, finalizer)

	// create target namespace
	targetNamespace := instance.Spec.TargetNamespace

	_, err := client.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: targetNamespace}})
	if err != nil && !errors.IsAlreadyExists(err) {
		return r.fail(instance, errorCreating, err.Error())
	}

	uid := string(instance.UID)

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
		addWorkloadReferenceLabel(&sec.ObjectMeta, uid)
		_, err = util.ApplySecret(client, sec)
		if err != nil {
			return r.fail(instance, errorCreating, err.Error())
		}
	}

	// propagate deployment
	d, err := r.propagateDeployment(client, instance.Spec.TargetDeployment, targetNamespace, uid)
	if err != nil {
		return r.fail(instance, errorCreating, err.Error())
	}
	instance.Status.Deployment = meta.ReferenceTo(d)

	// propagate service
	s, err := r.propagateService(client, instance.Spec.TargetService, targetNamespace, uid)
	if err != nil {
		return r.fail(instance, errorCreating, err.Error())
	}
	instance.Status.Service = meta.ReferenceTo(s)

	instance.Status.State = computev1alpha1.WorkloadStateCreating

	// update instance
	return resultDone, r.Status().Update(ctx, instance)
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
		instance.Status.SetDeprecatedCondition(corev1alpha1.NewDeprecatedCondition(corev1alpha1.DeprecatedReady, "", ""))
		return resultDone, r.Status().Update(ctx, instance)
	}

	return resultRequeue, r.Status().Update(ctx, instance)
}

// _delete workload
func (r *Reconciler) _delete(instance *computev1alpha1.Workload, client kubernetes.Interface) (reconcile.Result, error) {
	ns := instance.Spec.TargetNamespace

	// delete service
	if s := instance.Status.Service; s != nil {
		if err := client.CoreV1().Services(s.Namespace).Delete(s.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return r.fail(instance, errorDeleting, err.Error())
		}
	}

	// delete deployment
	if d := instance.Status.Deployment; d != nil {
		if err := client.AppsV1().Deployments(d.Namespace).Delete(d.Name, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return r.fail(instance, errorDeleting, err.Error())
		}
	}

	// delete resources secrets
	for _, resource := range instance.Spec.Resources {
		secretName := util.IfEmptyString(resource.SecretName, resource.Name)
		if err := client.CoreV1().Secrets(ns).Delete(secretName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return r.fail(instance, errorDeleting, err.Error())
		}
	}

	instance.Status.SetDeprecatedCondition(corev1alpha1.NewDeprecatedCondition(corev1alpha1.DeprecatedDeleting, "", ""))
	meta.RemoveFinalizer(instance, finalizer)
	return resultDone, r.Status().Update(ctx, instance)
}

// Reconcile reads that state of the cluster for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", computev1alpha1.WorkloadKindAPIVersion, "request", request)
	// fetch the CRD instance
	instance := &computev1alpha1.Workload{}

	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return resultDone, nil
		}
		return resultDone, err
	}

	if instance.Status.Cluster == nil {
		return resultDone, nil
	}

	// target cluster client
	targetClient, err := r.connect(instance)
	if err != nil {
		return r.fail(instance, errorClusterClient, err.Error())
	}

	// Check for deletion
	if instance.DeletionTimestamp != nil && instance.Status.DeprecatedCondition(corev1alpha1.DeprecatedDeleting) == nil {
		return r.delete(instance, targetClient)
	}

	if instance.Status.State == "" {
		return r.create(instance, targetClient)
	}

	return r.sync(instance, targetClient)
}
