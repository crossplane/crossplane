/*
Copyright 2019 The Crossplane Authors.

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
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	util "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/workload/v1alpha1"
)

const (
	finalizerName    = "finalizer.kubernetesapplicationresource." + v1alpha1.Group
	reconcileTimeout = 1 * time.Minute

	// The time we wait before requeueing a speculative reconcile.
	longWait  = 1 * time.Minute
	shortWait = 30 * time.Second
)

var (
	errUnmarshalTemplate = "cannot unmarshal template"
	errUpdateStatusFmt   = "cannot update status %s %s"
	errGetKey            = "cannot take object key from resource"
	errDeleteResource    = "cannot delete resource"
	errGetResource       = "cannot get resource"
	errSyncResource      = "cannot sync resource"
	errCreateResource    = "cannot create resource"
	errDeleteSecret      = "cannot delete secret"
	errGetSecret         = "cannot get secret"
	errSyncSecret        = "cannot sync secret"
)

// Ownership annotations
var (
	RemoteControllerNamespace = v1alpha1.KubernetesApplicationGroupVersionKind.GroupKind().String() + "/namespace"
	RemoteControllerName      = v1alpha1.KubernetesApplicationGroupVersionKind.GroupKind().String() + "/name"
	RemoteControllerUID       = v1alpha1.KubernetesApplicationGroupVersionKind.GroupKind().String() + "/uid"
)

// CreatePredicate accepts KubernetesApplicationResources that have been
// scheduled to a KubernetesTarget.
func CreatePredicate(event event.CreateEvent) bool {
	wl, ok := event.Object.(*v1alpha1.KubernetesApplicationResource)
	if !ok {
		return false
	}
	return wl.Spec.Target != nil
}

// UpdatePredicate accepts KubernetesApplicationResources that have been
// scheduled to a KubernetesTarget.
func UpdatePredicate(event event.UpdateEvent) bool {
	wl, ok := event.ObjectNew.(*v1alpha1.KubernetesApplicationResource)
	if !ok {
		return false
	}
	return wl.Spec.Target != nil
}

// Setup adds a controller that reconciles KubernetesApplicationResources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := "workload/" + strings.ToLower(v1alpha1.KubernetesApplicationResourceGroupKind)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.KubernetesApplicationResource{}).
		WithEventFilter(&predicate.Funcs{CreateFunc: CreatePredicate, UpdateFunc: UpdatePredicate}).
		Complete(&Reconciler{
			connecter: &clusterConnecter{kube: mgr.GetClient()},
			kube:      mgr.GetClient(),
			log:       l.WithValues("controller", name),
		})
}

// A syncer can sync resources with a KubernetesTarget.
type syncer interface {
	// sync the supplied resource with the external store. Returns true if the
	// resource requires further reconciliation.
	sync(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error)
}

// A deleter can delete resources from a KubernetesTarget.
type deleter interface {
	// delete the supplied resource from the external store.
	delete(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error)
}

// A syncdeleter can sync and delete KubernetesApplicationResources in a
// KubernetesTarget.
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

func (c *remoteCluster) sync(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
	meta.AddFinalizer(ar, finalizerName)

	template := &unstructured.Unstructured{}
	if err := json.Unmarshal(ar.Spec.Template.Raw, template); err != nil {
		return v1alpha1.KubernetesApplicationResourceStateFailed, errors.Wrap(err, errUnmarshalTemplate)
	}
	templates := createSecretTemplates(secrets, template.GetNamespace(), ar.GetName())
	for i := range templates {
		template := &templates[i]
		setRemoteController(ar, template)

		if err := c.secret.sync(ctx, template); err != nil {
			return v1alpha1.KubernetesApplicationResourceStateFailed, err
		}
	}

	setRemoteController(ar, template)

	status, err := c.unstructured.sync(ctx, template)
	// It's possible we read the remote object's status, but returned an error
	// because we failed to update said object. We still want to reflect the
	// latest remote status in this scenario.
	if status != nil {
		ar.Status.Remote = status
	}
	if err != nil {
		return v1alpha1.KubernetesApplicationResourceStateFailed, err
	}

	return v1alpha1.KubernetesApplicationResourceStateSubmitted, nil
}

func (c *remoteCluster) delete(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource, secrets []corev1.Secret) (v1alpha1.KubernetesApplicationResourceState, error) {
	template := &unstructured.Unstructured{}
	if err := json.Unmarshal(ar.Spec.Template.Raw, template); err != nil {
		return v1alpha1.KubernetesApplicationResourceStateFailed, errors.Wrap(err, errUnmarshalTemplate)
	}
	setRemoteController(ar, template)

	if err := c.unstructured.delete(ctx, template); err != nil {
		return v1alpha1.KubernetesApplicationResourceStateFailed, err
	}

	templates := createSecretTemplates(secrets, template.GetNamespace(), ar.GetName())
	for i := range templates {
		template := &templates[i]
		setRemoteController(ar, template)

		if err := c.secret.delete(ctx, template); err != nil {
			return v1alpha1.KubernetesApplicationResourceStateFailed, err
		}
	}

	// We return state Submitted here, but the status will not be updated
	// because the resource will cease to exist after removal of finalizer.
	return v1alpha1.KubernetesApplicationResourceStateSubmitted, nil
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
			Type: l.Type,
		}
	}
	return templates
}

type unstructuredClient struct {
	kube client.Client
}

func (c *unstructuredClient) sync(ctx context.Context, template *unstructured.Unstructured) (*v1alpha1.RemoteStatus, error) {
	remote := template.DeepCopy()

	key, err := client.ObjectKeyFromObject(remote)
	if err != nil {
		return nil, errors.Wrap(err, errGetKey)
	}

	// TODO(hasheddan): this Get operation and subsequent controller check can
	// be eliminated once resource.Apply is refactored to accept arbitrary
	// ApplyOption functions.
	if err := c.kube.Get(ctx, key, remote); err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.Wrap(c.kube.Create(ctx, remote), errCreateResource)
		}
		return nil, errors.Wrap(err, errGetResource)
	}
	if !haveSameController(remote, template) {
		return nil, errors.Wrap(errors.Errorf("%s %s/%s exists and is not controlled by %s %s",
			remote.GetObjectKind().GroupVersionKind().Kind, remote.GetNamespace(), remote.GetName(),
			v1alpha1.KubernetesApplicationResourceKind, template.GetAnnotations()[RemoteControllerName]), errSyncResource)
	}

	rs := getRemoteStatus(remote)
	return rs, errors.Wrap(resource.NewAPIPatchingApplicator(c.kube).Apply(ctx, template), errSyncResource)
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
	remote := template.DeepCopy()

	key, err := client.ObjectKeyFromObject(remote)
	if err != nil {
		return errors.Wrap(err, errGetKey)
	}

	if err := c.kube.Get(ctx, key, remote); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, errGetResource)
	}

	// The object exists, but we don't own it.
	if !haveSameController(remote, template) {
		return nil
	}

	// The object exists and we own it. Delete it.
	return errors.Wrap(c.kube.Delete(ctx, remote), errDeleteResource)
}

type secretClient struct {
	kube client.Client
}

func (c *secretClient) sync(ctx context.Context, template *corev1.Secret) error {
	// We make another copy of our template here so we can compare the template
	// as passed to this method with the remote resource.
	remote := template.DeepCopy()

	_, err := util.CreateOrUpdate(ctx, c.kube, remote, func() error {
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

	return errors.Wrap(err, errSyncSecret)
}

func (c *secretClient) delete(ctx context.Context, template *corev1.Secret) error {
	n := types.NamespacedName{Namespace: template.GetNamespace(), Name: template.GetName()}
	remote := template.DeepCopy()
	if err := c.kube.Get(ctx, n, remote); err != nil {
		// No object exists.
		if kerrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, errGetSecret)
	}

	// We don't own the existing object.
	if !haveSameController(remote, template) {
		return nil
	}

	// We own the existing object. delete it.
	return errors.Wrap(c.kube.Delete(ctx, remote), errDeleteSecret)
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
	if ar.Spec.Target == nil {
		return nil, errors.Errorf("%s %s is not scheduled", v1alpha1.KubernetesApplicationResourceKind, n)
	}

	n = types.NamespacedName{Namespace: ar.GetNamespace(), Name: ar.Spec.Target.Name}
	k := &v1alpha1.KubernetesTarget{}
	if err := c.kube.Get(ctx, n, k); err != nil {
		return nil, errors.Wrapf(err, "cannot get %s %s", v1alpha1.KubernetesTargetKind, n)
	}

	if k.GetWriteConnectionSecretToReference() == nil {
		return nil, errors.Errorf("%s %s has no connection secret", v1alpha1.KubernetesTargetKind, n)
	}

	n = types.NamespacedName{Namespace: k.GetNamespace(), Name: k.GetWriteConnectionSecretToReference().Name}
	s := &corev1.Secret{}
	if err := c.kube.Get(ctx, n, s); err != nil {
		return nil, errors.Wrapf(err, "cannot get secret %s", n)
	}

	if len(s.Data[runtimev1alpha1.ResourceCredentialsSecretKubeconfigKey]) != 0 {
		return clientcmd.RESTConfigFromKubeConfig(s.Data[runtimev1alpha1.ResourceCredentialsSecretKubeconfigKey])
	}

	u, err := url.Parse(string(s.Data[runtimev1alpha1.ResourceCredentialsSecretEndpointKey]))
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse Kubernetes endpoint as URL")
	}

	config := &rest.Config{
		Host:     u.String(),
		Username: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretUserKey]),
		Password: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretPasswordKey]),
		TLSClientConfig: rest.TLSClientConfig{
			// This field's godoc claims clients will use 'the hostname used to
			// contact the server' when it is left unset. In practice clients
			// appear to use the URL, including scheme and port.
			ServerName: u.Hostname(),
			CAData:     s.Data[runtimev1alpha1.ResourceCredentialsSecretCAKey],
			CertData:   s.Data[runtimev1alpha1.ResourceCredentialsSecretClientCertKey],
			KeyData:    s.Data[runtimev1alpha1.ResourceCredentialsSecretClientKeyKey],
		},
		BearerToken: string(s.Data[runtimev1alpha1.ResourceCredentialsSecretTokenKey]),
	}

	return config, nil
}

// connect returns a syncdeleter backed by a KubernetesTarget.
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
	log  logging.Logger
}

// Reconcile scheduled Kubernetes application resources by propagating them to
// their scheduled Kubernetes cluster.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	r.log.Debug("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	ar := &v1alpha1.KubernetesApplicationResource{}
	if err := r.kube.Get(ctx, req.NamespacedName, ar); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{Requeue: false}, nil
		}
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(err, "cannot get %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	meta.AddFinalizer(ar, finalizerName)
	if err := r.kube.Update(ctx, ar); err != nil {
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(err, "cannot update %s %s", v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	cluster, err := r.connect(ctx, ar)
	if err != nil {
		// If we're being deleted and we can't connect to our scheduled cluster
		// because it doesn't exist we assume the cluster was deleted.
		if ar.GetDeletionTimestamp() != nil && kerrors.IsNotFound(errors.Cause(err)) {
			meta.RemoveFinalizer(ar, finalizerName)
			return reconcile.Result{Requeue: false}, errors.Wrapf(r.kube.Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
		}
		ar.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(r.kube.Status().Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	secrets := r.getConnectionSecrets(ctx, ar)

	if ar.GetDeletionTimestamp() != nil {
		state, err := cluster.delete(ctx, ar, secrets)
		if err != nil {
			ar.Status.State = state
			ar.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(r.kube.Status().Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
		}
		meta.RemoveFinalizer(ar, finalizerName)
		return reconcile.Result{Requeue: false}, errors.Wrapf(r.kube.Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	state, err := cluster.sync(ctx, ar, secrets)
	if err != nil {
		ar.Status.State = state
		ar.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrapf(r.kube.Status().Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
	}

	ar.Status.State = state
	ar.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrapf(r.kube.Status().Update(ctx, ar), errUpdateStatusFmt, v1alpha1.KubernetesApplicationResourceKind, req.NamespacedName)
}

func (r *Reconciler) getConnectionSecrets(ctx context.Context, ar *v1alpha1.KubernetesApplicationResource) []corev1.Secret {
	secrets := make([]corev1.Secret, 0, len(ar.Spec.Secrets))
	for _, ref := range ar.Spec.Secrets {
		s := corev1.Secret{}
		n := types.NamespacedName{Namespace: ar.GetNamespace(), Name: ref.Name}
		if err := r.kube.Get(ctx, n, &s); err != nil {
			ar.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
			r.log.Debug("error getting connection secret", "namespace", n.Namespace, "name", n.Name, "err", err)
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

	// NOTE(hasheddan): we set an annotation for the UID of the remote
	// controller but do not check that it matches because it would prohibit a
	// lost KubernetesApplication from re-establishing ownership of its remote
	// resources.
	if a.GetAnnotations()[RemoteControllerNamespace] != b.GetAnnotations()[RemoteControllerNamespace] {
		return false
	}
	return a.GetAnnotations()[RemoteControllerName] == b.GetAnnotations()[RemoteControllerName]
}
