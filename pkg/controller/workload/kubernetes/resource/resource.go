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

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
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
	"github.com/crossplaneio/crossplane/pkg/apis/workload/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/logging"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

const (
	controllerName   = "kubernetesapplicationresource." + v1alpha1.Group
	finalizerName    = "finalizer." + controllerName
	reconcileTimeout = 1 * time.Minute

	reasonFetchingClient   = "failed to fetch Kubernetes client"
	reasonSyncingResource  = "failed to sync templated resource in Kubernetes cluster"
	reasonDeletingResource = "failed to delete templated resource from Kubernetes cluster"
	reasonGettingSecret    = "failed to get connection secret for resource dependency"
	reasonSyncingSecret    = "failed to update connection secret for resource dependency"
	reasonDeletingSecret   = "failed to delete connection secret for resource dependency"

	messageMissingTemplate = v1alpha1.KubernetesApplicationResourceKind + " must include a template"
)

// Ownership annotations
const (
	RemoteControllerNamespace = v1alpha1.KubernetesApplicationResourceKind + "." + v1alpha1.Group + "/namespace"
	RemoteControllerName      = v1alpha1.KubernetesApplicationResourceKind + "." + v1alpha1.Group + "/name"
	RemoteControllerUID       = v1alpha1.KubernetesApplicationResourceKind + "." + v1alpha1.Group + "/uid"
)

var (
	log = logging.Logger.WithName("controller." + controllerName)
)

// CreatePredicate accepts KubernetesApplicationResources that have been
// scheduled to a KubernetesCluster.
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*v1alpha1.KubernetesApplicationResource)
	if !ok {
		return false
	}
	return wl.Status.Cluster != nil
}

// UpdatePredicate accepts KubernetesApplicationResources that have been
// scheduled to a KubernetesCluster.
func UpdatePredicate(event event.UpdateEvent) bool {
	wl, ok := event.ObjectNew.(*v1alpha1.KubernetesApplicationResource)
	if !ok {
		return false
	}
	return wl.Status.Cluster != nil
}

// Add creates a new Instance Controller and adds it to the Manager with default RBAC.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	r := &Reconciler{
		connecter: &clusterConnecter{kube: mgr.GetClient()},
		kube:      mgr.GetClient(),
	}
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "cannot create Kubernetes controller")
	}

	err = c.Watch(
		&source.Kind{Type: &v1alpha1.KubernetesApplicationResource{}},
		&handler.EnqueueRequestForObject{},
		&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate},
	)
	return errors.Wrapf(err, "cannot watch for %s", v1alpha1.KubernetesApplicationResourceKind)
}

// A syncer can sync resources with a KubernetesCluster.
type syncer interface {
	// sync the supplied resource with the external store. Returns true if the
	// resource requires further reconciliation.
	sync(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) reconcile.Result
}

// A deleter can delete resources from a KubernetesCluster.
type deleter interface {
	// delete the supplied resource from the external store. Returns true if the
	// resource requires further reconciliation.
	delete(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) reconcile.Result
}

// A syncdeleter can sync and delete KubernetesApplicationResources in a
// KubernetesCluster.
type syncdeleter interface {
	syncer
	deleter
}

type unstructuredSyncer interface {
	sync(ctx context.Context, template *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error)
}

type unstructuredDeleter interface {
	delete(ctx context.Context, template *unstructured.Unstructured) error
}

type unstructuredSyncDeleter interface {
	unstructuredSyncer
	unstructuredDeleter
}

type secretSyncer interface {
	sync(ctx context.Context, template *corev1.Secret) error
}

type secretDeleter interface {
	delete(ctx context.Context, template *corev1.Secret) error
}

type secretSyncDeleter interface {
	secretSyncer
	secretDeleter
}

type remoteCluster struct {
	unstructured unstructuredSyncDeleter
	secret       secretSyncDeleter
}

func (c *remoteCluster) sync(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) reconcile.Result {
	meta.AddFinalizer(ar, finalizerName)
	ar.Status.UnsetAllDeprecatedConditions()

	// Our CRD requires template to be specified, but just in case...
	if ar.Spec.Template == nil {
		ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
		ar.Status.SetFailed(reasonSyncingResource, messageMissingTemplate)
		return reconcile.Result{Requeue: true}
	}

	templates := createSecretTemplates(secrets, ar.Spec.Template.GetNamespace(), ar.GetName())
	for i := range templates {
		template := &templates[i]
		ensureNamespace(template)
		setRemoteController(ar, template)

		if err := c.secret.sync(ctx, template); err != nil {
			ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
			ar.Status.SetFailed(reasonSyncingSecret, err.Error())
			return reconcile.Result{Requeue: true}
		}
	}

	// We copy the resource template here so we can modify its namespace and
	// remote controller annotations without persisting those changes back to
	// the KubernetesApplicationResource.
	template := ar.Spec.Template.DeepCopy()
	ensureNamespace(template)
	setRemoteController(ar, template)

	status, err := c.unstructured.sync(ctx, template)
	// It's possible we read the remote object's status, but returned an error
	// because we failed to update said object. We still want to reflect the
	// latest remote status in this scenario.
	if status != nil {
		ar.Status.Remote = status
	}
	if err != nil {
		ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
		ar.Status.SetFailed(reasonSyncingResource, err.Error())
		return reconcile.Result{Requeue: true}
	}

	ar.Status.SetReady()
	ar.Status.State = v1alpha1.KubernetesApplicationResourceStateSubmitted
	return reconcile.Result{Requeue: false}
}

func (c *remoteCluster) delete(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) reconcile.Result {
	ar.Status.UnsetAllDeprecatedConditions()
	ar.Status.SetDeleting()

	// Our CRD requires template to be specified, but just in case...
	if ar.Spec.Template == nil {
		ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
		ar.Status.SetFailed(reasonDeletingResource, messageMissingTemplate)
		return reconcile.Result{Requeue: true}
	}

	// We copy the resource template here so we can modify its namespace and
	// remote controller annotations without persisting those changes back to
	// the KubernetesApplicationResource.
	template := ar.Spec.Template.DeepCopy()
	ensureNamespace(template)
	setRemoteController(ar, template)

	if err := c.unstructured.delete(ctx, template); err != nil {
		ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
		ar.Status.SetFailed(reasonDeletingResource, err.Error())
		return reconcile.Result{Requeue: true}
	}

	templates := createSecretTemplates(secrets, ar.Spec.Template.GetNamespace(), ar.GetName())
	for i := range templates {
		template := &templates[i]
		ensureNamespace(template)
		setRemoteController(ar, template)

		if err := c.secret.delete(ctx, template); err != nil {
			ar.Status.State = v1alpha1.KubernetesApplicationResourceStateFailed
			ar.Status.SetFailed(reasonDeletingSecret, err.Error())
			return reconcile.Result{Requeue: true}
		}
	}
	meta.RemoveFinalizer(ar, finalizerName)
	return reconcile.Result{Requeue: false}
}

func createSecretTemplates(local []corev1.Secret, namespace, namePrefix string) []corev1.Secret {
	templates := make([]corev1.Secret, len(local))
	for i, l := range local {
		templates[i] = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   namespace,
				Name:        fmt.Sprintf("%s-%s", namePrefix, l.GetName()),
				Labels:      l.GetLabels(),
				Annotations: l.GetAnnotations(),
			},
			Data: l.Data,
		}
	}
	return templates
}

type unstructuredClient struct {
	kube client.Client
}

func (c *unstructuredClient) sync(ctx context.Context, template *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
	// We make another copy of our template here so we can compare the template
	// as passed to this method with the remote resource.
	remote := template.DeepCopy()

	// TODO(negz): Handle immutable, server-populated, fields such as a
	// Service's ClusterIP. For example:
	// spec.clusterIP: Invalid value: "": field is immutable
	// Generate a JSON patch from the two unstructured contents?

	var rs *v1alpha1.RemoteStatus

	err := util.CreateOrUpdate(ctx, c.kube, remote, func() error {
		// Inside this anonymous function remote could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.

		if !haveSameController(remote, template) {
			return errors.Errorf("%s %s/%s exists and is not controlled by %s %s",
				remote.GetObjectKind().GroupVersionKind().Kind, remote.GetNamespace(), remote.GetName(),
				v1alpha1.KubernetesApplicationResourceKind, template.GetAnnotations()[RemoteControllerName])
		}

		// Propagate the 'status' field of remote (if any) before we overwrite
		// it with our template.
		rs = getRemoteStatus(remote)

		existing := remote.DeepCopy()
		template.DeepCopyInto(remote)

		// Keep important metadata from any existing resource.
		remote.SetUID(existing.GetUID())
		remote.SetResourceVersion(existing.GetResourceVersion())
		remote.SetNamespace(existing.GetNamespace())

		return nil
	})

	return rs, errors.Wrap(err, "cannot sync resource")
}

func getRemoteStatus(u runtime.Unstructured) *v1alpha1.RemoteStatus {
	status, ok := u.UnstructuredContent()["status"]
	if !ok {
		// This object does not have a status.
		return nil
	}

	j, err := json.Marshal(status)
	if err != nil {
		// This object's status cannot be represented as JSON.
		return nil
	}

	remote := &v1alpha1.RemoteStatus{}
	if err := json.Unmarshal(j, remote); err != nil {
		// This object's status cannot be represented as JSON.
		return nil
	}

	return remote
}

func (c *unstructuredClient) delete(ctx context.Context, template *unstructured.Unstructured) error {
	n := types.NamespacedName{Namespace: template.GetNamespace(), Name: template.GetName()}
	remote := template.DeepCopy()
	if err := c.kube.Get(ctx, n, remote); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get resource %s", n)
	}

	// The object exists, but we don't own it.
	if !haveSameController(remote, template) {
		return nil
	}

	// The object exists and we own it. Delete it.
	return errors.Wrapf(c.kube.Delete(ctx, remote), "cannot delete resource %s", n)
}

type secretClient struct {
	kube client.Client
}

func (c *secretClient) sync(ctx context.Context, template *corev1.Secret) error {
	// We make another copy of our template here so we can compare the template
	// as passed to this method with the remote resource.
	remote := template.DeepCopy()

	err := util.CreateOrUpdate(ctx, c.kube, remote, func() error {
		// Inside this anonymous function remote could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.

		if !haveSameController(remote, template) {
			return errors.Errorf("secret %s/%s exists and is not controlled by %s %s",
				remote.GetNamespace(), remote.GetName(),
				v1alpha1.KubernetesApplicationResourceKind, template.GetAnnotations()[RemoteControllerName])
		}

		existing := remote.DeepCopy()
		template.DeepCopyInto(remote)

		// Keep important metadata from any existing resource.
		remote.SetUID(existing.GetUID())
		remote.SetResourceVersion(existing.GetResourceVersion())
		remote.SetNamespace(existing.GetNamespace())

		return nil
	})

	return errors.Wrap(err, "cannot sync secret")
}

func (c *secretClient) delete(ctx context.Context, template *corev1.Secret) error {
	n := types.NamespacedName{Namespace: template.GetNamespace(), Name: template.GetName()}
	remote := template.DeepCopy()
	if err := c.kube.Get(ctx, n, remote); err != nil {
		// No object exists.
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot get secret %s", n)
	}

	// We don't own the existing object.
	if !haveSameController(remote, template) {
		return nil
	}

	// We own the existing object. delete it.
	return errors.Wrapf(c.kube.Delete(ctx, remote), "cannot delete secret %s", n)
}

// A connecter returns a createsyncdeletekeyer that can create, sync, and delete
// application resources with a remote Kubernetes cluster.
type connecter interface {
	connect(context.Context, *v1alpha1.KubernetesApplicationResource) (syncdeleter, error)
}

type clusterConnecter struct {
	kube    client.Client
	options client.Options
}

func (c *clusterConnecter) config(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) (*rest.Config, error) {
	n := types.NamespacedName{Namespace: ar.GetNamespace(), Name: ar.GetName()}
	if ar.Status.Cluster == nil {
		return nil, errors.Errorf("%s %s is not scheduled", v1alpha1.KubernetesApplicationResourceKind, n)
	}

	n = types.NamespacedName{Namespace: ar.Status.Cluster.Namespace, Name: ar.Status.Cluster.Name}
	k := &computev1alpha1.KubernetesCluster{}
	if err := c.kube.Get(ctx, n, k); err != nil {
		return nil, errors.Wrapf(err, "cannot get %s %s", computev1alpha1.KubernetesClusterKind, n)
	}

	n = types.NamespacedName{Namespace: k.GetNamespace(), Name: k.Status.CredentialsSecretRef.Name}
	s := &corev1.Secret{}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get secret %s", n)
	}

	u, err := url.Parse(string(s.Data[corev1alpha1.ResourceCredentialsSecretEndpointKey]))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse Kubernetes endpoint as URL")
	}

	config := &rest.Config{
		Host:     u.String(),
		Username: string(s.Data[corev1alpha1.ResourceCredentialsSecretUserKey]),
		Password: string(s.Data[corev1alpha1.ResourceCredentialsSecretPasswordKey]),
		TLSClientConfig: rest.TLSClientConfig{
			// This field's godoc claims clients will use 'the hostname used to
			// contact the server' when it is left unset. In practice clients
			// appear to use the URL, including scheme and port.
			ServerName: u.Hostname(),
			CAData:     s.Data[corev1alpha1.ResourceCredentialsSecretCAKey],
			CertData:   s.Data[corev1alpha1.ResourceCredentialsSecretClientCertKey],
			KeyData:    s.Data[corev1alpha1.ResourceCredentialsSecretClientKeyKey],
		},
		BearerToken: string(s.Data[corev1alpha1.ResourceCredentialsTokenKey]),
	}

	return config, nil
}

// connect returns a syncdeleter backed by a KubernetesCluster.
// Cluster credentials are read from a Crossplane connection secret.
func (c *clusterConnecter) connect(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) (syncdeleter, error) {
	config, err := c.config(ctx, ar)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Kubernetes client configuration")
	}

	kc, err := client.New(config, c.options)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Kubernetes client")
	}

	return &remoteCluster{unstructured: &unstructuredClient{kube: kc}, secret: &secretClient{kube: kc}}, nil
}

// Reconciler reconciles a Instance object
type Reconciler struct {
	connecter
	kube client.Client
}

// Reconcile scheduled Kubernetes application resources by propagating them to
// their scheduled Kubernetes cluster.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.KubernetesApplicationResourceKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	ar := &v1alpha1.KubernetesApplicationResource{}
	if err := r.kube.Get(ctx, req.NamespacedName, ar); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{Requeue: false}, errors.Wrapf(err, "cannot get %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	cluster, err := r.connect(ctx, ar)
	if err != nil {
		// If we're being deleted and we can't connect to our scheduled cluster
		// because it doesn't exist we assume the cluster was deleted.
		if ar.GetDeletionTimestamp() != nil && kerrors.IsNotFound(errors.Cause(err)) {
			meta.RemoveFinalizer(ar, finalizerName)
			return reconcile.Result{Requeue: false}, errors.Wrapf(r.kube.Update(ctx, ar), "cannot update %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
		}
		ar.Status.SetFailed(reasonFetchingClient, err.Error())
		return reconcile.Result{Requeue: true}, errors.Wrapf(r.kube.Update(ctx, ar), "cannot update %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	secrets := r.getConnectionSecrets(ctx, ar)

	if ar.GetDeletionTimestamp() != nil {
		return cluster.delete(ctx, ar, secrets), errors.Wrapf(r.kube.Update(ctx, ar), "cannot update %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	return cluster.sync(ctx, ar, secrets), errors.Wrapf(r.kube.Update(ctx, ar), "cannot update %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
}

func (r *Reconciler) getConnectionSecrets(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) []corev1.Secret {
	secrets := make([]corev1.Secret, 0, len(ar.Spec.Secrets))
	for _, ref := range ar.Spec.Secrets {
		s := corev1.Secret{}
		n := types.NamespacedName{Namespace: ar.GetNamespace(), Name: ref.Name}
		if err := r.kube.Get(ctx, n, &s); err != nil {
			ar.Status.SetFailed(reasonGettingSecret, err.Error())
			continue
		}
		secrets = append(secrets, s)
	}
	return secrets
}

func setRemoteController(ctrl metav1.Object, obj metav1.Object) {
	meta.AddAnnotations(obj, map[string]string{
		RemoteControllerNamespace: ctrl.GetNamespace(),
		RemoteControllerName:      ctrl.GetName(),
		RemoteControllerUID:       string(ctrl.GetUID()),
	})
}

func hasController(obj metav1.Object) bool {
	a := obj.GetAnnotations()
	if a[RemoteControllerNamespace] == "" {
		return false
	}

	if a[RemoteControllerName] == "" {
		return false
	}

	if a[RemoteControllerUID] == "" {
		return false
	}

	return true
}

func haveSameController(a, b metav1.Object) bool {
	// We do not consider two objects without any controller to have
	// the same controller.
	if !hasController(a) {
		return false
	}

	return a.GetAnnotations()[RemoteControllerUID] == b.GetAnnotations()[RemoteControllerUID]
}

func ensureNamespace(obj metav1.Object) {
	if obj.GetNamespace() != "" {
		return
	}
	obj.SetNamespace(corev1.NamespaceDefault)
}
