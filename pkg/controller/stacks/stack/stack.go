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

package stack

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplaneio/crossplane/pkg/controller/stacks/hostaware"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplaneio/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/meta"
	runtimeresource "github.com/crossplaneio/crossplane-runtime/pkg/resource"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/stacks"
)

const (
	controllerName  = "stack.stacks.crossplane.io"
	stacksFinalizer = "finalizer.stacks.crossplane.io"

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second
)

var (
	log              = logging.Logger.WithName(controllerName)
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}

	roleVerbs = map[string][]string{
		"admin": {"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"},
		"edit":  {"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"},
		"view":  {"get", "list", "watch"},
	}
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	kube            client.Client
	hostKube        client.Client
	hostAwareConfig *hostaware.Config
	factory
}

// Controller is responsible for adding the Stack
// controller and its corresponding reconciler to the manager with any runtime configuration.
type Controller struct{}

// SetupWithManager creates a new Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	hostKube := mgr.GetClient()

	haCfg, err := hostaware.NewConfig()
	if err != nil {
		return err
	}
	if haCfg != nil {
		hostKube, _, err = hostaware.GetClients()
		if err != nil {
			return err
		}
	}

	r := &Reconciler{
		kube:            mgr.GetClient(),
		hostKube:        hostKube,
		hostAwareConfig: haCfg,
		factory:         &stackHandlerFactory{},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1alpha1.Stack{}).
		Complete(r)
}

// Reconcile reads that state of the Stack for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	log.V(logging.Debug).Info("reconciling", "kind", v1alpha1.StackKindAPIVersion, "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	i := &v1alpha1.Stack{}
	if err := r.kube.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	handler := r.factory.newHandler(ctx, i, r.kube, r.hostKube, r.hostAwareConfig)

	if meta.WasDeleted(i) {
		return handler.delete(ctx)
	}

	return handler.sync(ctx)
}

type handler interface {
	sync(context.Context) (reconcile.Result, error)
	create(context.Context) (reconcile.Result, error)
	update(context.Context) (reconcile.Result, error)
	delete(context.Context) (reconcile.Result, error)
}

type stackHandler struct {
	kube            client.Client
	hostKube        client.Client
	hostAwareConfig *hostaware.Config
	ext             *v1alpha1.Stack
}

type factory interface {
	newHandler(context.Context, *v1alpha1.Stack, client.Client, client.Client, *hostaware.Config) handler
}

type stackHandlerFactory struct{}

func (f *stackHandlerFactory) newHandler(ctx context.Context, ext *v1alpha1.Stack, kube client.Client, hostKube client.Client, hostAwareConfig *hostaware.Config) handler {
	return &stackHandler{
		kube:            kube,
		hostKube:        hostKube,
		hostAwareConfig: hostAwareConfig,
		ext:             ext,
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *stackHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.ext.Status.ControllerRef == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

func (h *stackHandler) create(ctx context.Context) (reconcile.Result, error) {
	h.ext.Status.SetConditions(runtimev1alpha1.Creating())

	// Add the finalizer before the RBAC and Deployments. If the Deployment
	// irreconcilably fails, the finalizer must be in place to delete the Roles
	patchCopy := h.ext.DeepCopy()
	meta.AddFinalizer(h.ext, stacksFinalizer)
	if err := h.kube.Patch(ctx, h.ext, client.MergeFrom(patchCopy)); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// create RBAC permissions
	if err := h.processRBAC(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// create controller deployment or job
	if err := h.processDeployment(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.processJob(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// the stack has successfully been created, the stack is ready
	h.ext.Status.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())
	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *stackHandler) update(ctx context.Context) (reconcile.Result, error) {
	log.V(logging.Debug).Info("updating not supported yet", "stack", h.ext.Name)
	return reconcile.Result{}, nil
}

func copyLabels(labels map[string]string) map[string]string {
	labelsCopy := map[string]string{}
	for k, v := range labels {
		labelsCopy[k] = v
	}
	return labelsCopy
}

func (h *stackHandler) crdListsDiffer(crds []apiextensions.CustomResourceDefinition) bool {
	return len(crds) != len(h.ext.Spec.CRDs)
}

// crdsFromStack fetches the CRDs of the Stack using the shared parent labels
// TODO(displague) change this to use GET on each, the CRDs may not be ready
func (h *stackHandler) crdsFromStack(ctx context.Context) ([]apiextensions.CustomResourceDefinition, error) {
	// Fetch CRDs because h.ext.Spec.CRDs doesn't have plural names
	crds := &apiextensions.CustomResourceDefinitionList{}
	stackLabels := h.ext.GetLabels()
	if err := h.kube.List(ctx, crds, client.MatchingLabels(map[string]string{
		stacks.LabelParentGroup:     stackLabels[stacks.LabelParentGroup],
		stacks.LabelParentKind:      stackLabels[stacks.LabelParentKind],
		stacks.LabelParentName:      stackLabels[stacks.LabelParentName],
		stacks.LabelParentNamespace: stackLabels[stacks.LabelParentNamespace],
		stacks.LabelParentUID:       stackLabels[stacks.LabelParentUID],
		stacks.LabelParentVersion:   stackLabels[stacks.LabelParentVersion],
	})); err != nil {
		return nil, errors.Wrap(err, "failed to list crds")
	}

	if h.crdListsDiffer(crds.Items) {
		return nil, errors.New("failed to list all expected crds")
	}

	return crds.Items, nil
}

// createPersonaClusterRoles creates admin, edit, and view clusterroles that are
// namespace+stack+version specific
func (h *stackHandler) createPersonaClusterRoles(ctx context.Context, labels map[string]string) error {
	crds, err := h.crdsFromStack(ctx)
	if err != nil {
		return err
	}

	for persona := range roleVerbs {
		name := stacks.PersonaRoleName(h.ext, persona)

		// Use a copy so AddLabels doesn't mutate labels
		labelsCopy := copyLabels(labels)

		// Create labels appropriate for the scope of the ClusterRole
		var crossplaneScope string

		if h.isNamespaced() {
			crossplaneScope = stacks.NamespaceScoped

			labelNamespace := fmt.Sprintf(stacks.LabelNamespaceFmt, h.ext.GetNamespace())
			labelsCopy[labelNamespace] = "true"
		} else {
			crossplaneScope = stacks.EnvironmentScoped
		}

		// Aggregation labels grant Stack CRD responsibilities
		// to namespace or environment personas like crossplane-env-admin
		// or crossplane:ns:default:view
		aggregationLabel := fmt.Sprintf(stacks.LabelAggregateFmt, crossplaneScope, persona)
		labelsCopy[aggregationLabel] = "true"

		// Each ClusterRole needs persona specific rules for each CRD
		rules := []rbacv1.PolicyRule{}

		for _, crd := range crds {
			kinds := []string{crd.Spec.Names.Plural}

			if subs := crd.Spec.Subresources; subs != nil {
				if subs.Status != nil {
					kinds = append(kinds, crd.Spec.Names.Plural+"/status")
				}
				if subs.Scale != nil {
					kinds = append(kinds, crd.Spec.Names.Plural+"/scale")
				}
			}

			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{crd.Spec.Group},
				Resources: kinds,
				Verbs:     roleVerbs[persona],
			})
		}

		// Assemble and create the ClusterRole
		cr := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labelsCopy,
			},
			Rules: rules,
		}

		if err := h.kube.Create(ctx, cr); err != nil && !kerrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "failed to create persona cluster roles")
		}
	}
	return nil
}

// generateNamespaceClusterRoles generates the crossplane:ns:{name}:{persona}
// roles for a given stack's namespace.
func generateNamespaceClusterRoles(stack *v1alpha1.Stack) (roles []*rbacv1.ClusterRole) {
	personas := []string{"admin", "edit", "view"}

	nsName := stack.GetNamespace()

	for _, persona := range personas {
		name := fmt.Sprintf(stacks.NamespaceClusterRoleNameFmt, nsName, persona)

		labels := map[string]string{
			fmt.Sprintf(stacks.LabelNamespaceFmt, nsName): "true",
			stacks.LabelScope: stacks.NamespaceScoped,
		}

		if persona == "admin" {
			labels[fmt.Sprintf(stacks.LabelAggregateFmt, "crossplane", persona)] = "true"
		}

		// By specifying MatchLabels, ClusterRole Aggregation will pass
		// along the rules from other ClusterRoles with the desired labels.
		// This is why we don't define any Rules here.
		role := &rbacv1.ClusterRole{
			AggregationRule: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							fmt.Sprintf(stacks.LabelAggregateFmt, stacks.NamespaceScoped, persona): "true",
							fmt.Sprintf(stacks.LabelNamespaceFmt, nsName):                          "true",
						},
					},
					{
						MatchLabels: map[string]string{
							fmt.Sprintf(stacks.LabelAggregateFmt, "namespace-default", persona): "true",
						},
					},
				},
			},

			// TODO(displague) set parent labels?
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
		}

		roles = append(roles, role)
	}

	return roles
}

// createNamespaceClusterRoles creates the crossplane:ns:{name}:{persona}
// roles for a given stack's namespace
func (h *stackHandler) createNamespaceClusterRoles(ctx context.Context) error {
	if !h.isNamespaced() {
		return nil
	}

	// Get the Namepsace because we need the UID for OwnerReference
	ns := &corev1.Namespace{}
	nsName := h.ext.GetNamespace()

	if err := h.kube.Get(ctx, types.NamespacedName{Name: nsName}, ns); err != nil {
		return errors.Wrapf(err, "failed to get namespace %q for stack %q", nsName, h.ext.GetName())
	}

	// generate namespace + stack specific clusterroles for all personas
	roles := generateNamespaceClusterRoles(h.ext)

	for _, role := range roles {
		// When the namespace is deleted, we no longer need these clusterroles.
		// Set the ClusterRole owner to the Namespace so they are cleaned up
		// automatically
		role.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       nsName,
				UID:        ns.GetUID(),
			},
		})

		// Creating the clusterroles. Since these Rules in these clusterroles
		// are populated through aggregation from the stacks installed in the
		// namespaces, we won't need to update them.
		if err := h.kube.Create(ctx, role); err != nil && !kerrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed to create clusterrole %s for stack %s", role.GetName(), h.ext.GetName())
		}
	}
	return nil
}

func (h *stackHandler) createDeploymentClusterRole(ctx context.Context, labels map[string]string) (string, error) {
	name := stacks.PersonaRoleName(h.ext, "system")
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Rules: h.ext.Spec.Permissions.Rules,
	}

	if err := h.kube.Create(ctx, cr); err != nil && !kerrors.IsAlreadyExists(err) {
		return "", errors.Wrap(err, "failed to create cluster role")
	}

	return name, nil
}

func (h *stackHandler) createNamespacedRoleBinding(ctx context.Context, clusterRoleName string, owner metav1.OwnerReference) error {
	// create rolebinding between service account and role
	crb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRoleName},
		Subjects: []rbacv1.Subject{
			{Name: h.ext.Name, Namespace: h.ext.Namespace, Kind: rbacv1.ServiceAccountKind},
		},
	}
	if err := h.kube.Create(ctx, crb); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create role binding")
	}
	return nil
}

func (h *stackHandler) createClusterRoleBinding(ctx context.Context, clusterRoleName string, labels map[string]string) error {
	// create clusterrolebinding between service account and role
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   h.ext.Name,
			Labels: labels,
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRoleName},
		Subjects: []rbacv1.Subject{
			{Name: h.ext.Name, Namespace: h.ext.Namespace, Kind: rbacv1.ServiceAccountKind},
		},
	}
	if err := h.kube.Create(ctx, crb); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create cluster role binding")
	}
	return nil
}

func (h *stackHandler) processRBAC(ctx context.Context) error {
	if len(h.ext.Spec.Permissions.Rules) == 0 {
		return nil
	}

	owner := meta.AsOwner(meta.ReferenceTo(h.ext, v1alpha1.StackGroupVersionKind))

	// create service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}

	if err := h.kube.Create(ctx, sa); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create service account")
	}

	labels := stacks.ParentLabels(h.ext)

	clusterRoleName, err := h.createDeploymentClusterRole(ctx, labels)
	if err != nil {
		return err
	}

	// give the SA rolebindings to run the the stack's controller
	var roleBindingErr error

	switch apiextensions.ResourceScope(h.ext.Spec.PermissionScope) {
	case apiextensions.ClusterScoped:
		roleBindingErr = h.createClusterRoleBinding(ctx, clusterRoleName, labels)
	case "", apiextensions.NamespaceScoped:
		if nsClusterRoleErr := h.createNamespaceClusterRoles(ctx); nsClusterRoleErr != nil {
			return nsClusterRoleErr
		}
		roleBindingErr = h.createNamespacedRoleBinding(ctx, clusterRoleName, owner)

	default:
		roleBindingErr = errors.New("invalid permissionScope for stack")
	}

	if roleBindingErr != nil {
		return roleBindingErr
	}

	// create persona roles
	return h.createPersonaClusterRoles(ctx, labels)
}

func (h *stackHandler) isNamespaced() bool {
	return apiextensions.ResourceScope(h.ext.Spec.PermissionScope) == apiextensions.NamespaceScoped
}

func (h *stackHandler) syncSATokenSecret(ctx context.Context, owner metav1.OwnerReference, fromSARef corev1.ObjectReference, toSecretRef corev1.ObjectReference) error {
	// Get the ServiceAccount
	fromKube := h.kube
	toKube := h.hostKube

	sa := corev1.ServiceAccount{}
	err := fromKube.Get(ctx, client.ObjectKey{
		Name:      fromSARef.Name,
		Namespace: fromSARef.Namespace,
	}, &sa)
	if kerrors.IsNotFound(err) {
		return errors.Wrap(err, "service account is not found (not created yet?)")
	}
	if err != nil {
		return errors.Wrap(err, "failed to get service account")
	}
	if len(sa.Secrets) < 1 {
		return errors.New("service account token secret is not generated yet")
	}
	saSecretRef := sa.Secrets[0]
	saSecretRef.Namespace = fromSARef.Namespace
	saSecret := corev1.Secret{}

	err = fromKube.Get(ctx, meta.NamespacedNameOf(&saSecretRef), &saSecret)

	if err != nil {
		return errors.Wrap(err, "failed to get service account token secret")
	}
	saSecretOnHost := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            toSecretRef.Name,
			Namespace:       toSecretRef.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Data: saSecret.Data,
	}

	err = toKube.Create(ctx, saSecretOnHost)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create sa token secret on target Kubernetes")
	}

	return nil
}

func (h *stackHandler) prepareHostAwarePodSpec(tokenSecret string, ps *corev1.PodSpec) error {
	// Opt out service account token automount
	disable := false
	ps.AutomountServiceAccountToken = &disable
	ps.ServiceAccountName = ""
	ps.DeprecatedServiceAccount = ""

	m := int32(420)
	ps.Volumes = append(ps.Volumes, corev1.Volume{
		Name: "sa-token",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tokenSecret,
				DefaultMode: &m,
			},
		},
	})

	for i := range ps.Containers {
		ps.Containers[i].Env = append(ps.Containers[i].Env,
			corev1.EnvVar{
				Name:  "KUBERNETES_SERVICE_HOST",
				Value: h.hostAwareConfig.TenantAPIServiceHost,
			}, corev1.EnvVar{
				Name:  "KUBERNETES_SERVICE_PORT",
				Value: h.hostAwareConfig.TenantAPIServicePort,
			}, corev1.EnvVar{
				// When POD_NAMESPACE is not set as stackinstalls namespace here, it is set as host namespace where actual
				// pod running. This result stack controller to fails with forbidden, since their sa only allows to watch
				// the namespace where stack is installed
				Name:  "POD_NAMESPACE",
				Value: h.ext.Namespace,
			})

		ps.Containers[i].VolumeMounts = append(ps.Containers[i].VolumeMounts, corev1.VolumeMount{
			Name:      "sa-token",
			ReadOnly:  true,
			MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
		})
	}

	return nil
}

func (h *stackHandler) prepareHostAwareDeployment(d *apps.Deployment, tokenSecret string) error {
	if err := h.prepareHostAwarePodSpec(tokenSecret, &d.Spec.Template.Spec); err != nil {
		return err
	}

	o := h.hostAwareConfig.ObjectReferenceOnHost(d.Name, d.Namespace)
	d.Name = o.Name
	d.Namespace = o.Namespace

	return nil
}
func (h *stackHandler) prepareHostAwareJob(j *batch.Job, tokenSecret string) error {
	if err := h.prepareHostAwarePodSpec(tokenSecret, &j.Spec.Template.Spec); err != nil {
		return err
	}

	o := h.hostAwareConfig.ObjectReferenceOnHost(j.Name, j.Namespace)
	j.Name = o.Name
	j.Namespace = o.Namespace

	return nil
}

func (h *stackHandler) processDeployment(ctx context.Context) error {
	controllerDeployment := h.ext.Spec.Controller.Deployment
	if controllerDeployment == nil {
		return nil
	}

	// ensure the deployment is set to use this stack's service account that we created
	deploymentSpec := *controllerDeployment.Spec.DeepCopy()
	deploymentSpec.Template.Spec.ServiceAccountName = h.ext.Name

	labels := stacks.ParentLabels(h.ext)
	gvk := schema.GroupVersionKind{
		Group:   apps.GroupName,
		Kind:    "Deployment",
		Version: apps.SchemeGroupVersion.Version,
	}
	d := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerDeployment.Name,
			Namespace: h.ext.Namespace,
			Labels:    labels,
		},
		Spec: deploymentSpec,
	}
	var saRef corev1.ObjectReference
	var saSecretRef corev1.ObjectReference
	if h.hostAwareConfig != nil {
		// We need to copy SA token secret from host to tenant
		saRef = corev1.ObjectReference{
			Name:      d.Spec.Template.Spec.ServiceAccountName,
			Namespace: d.Namespace,
		}
		saSecretRef = h.hostAwareConfig.ObjectReferenceOnHost(saRef.Name, saRef.Namespace)
		err := h.prepareHostAwareDeployment(d, saSecretRef.Name)
		if err != nil {
			return errors.Wrap(err, "failed process host aware stack controller deployment")
		}
	}
	if err := h.hostKube.Create(ctx, d); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create deployment")
	}
	if h.hostAwareConfig != nil {
		owner := meta.AsOwner(meta.ReferenceTo(d, gvk))
		err := h.syncSATokenSecret(ctx, owner, saRef, saSecretRef)
		if err != nil {
			return errors.Wrap(err, "failed sync stack controller service account secret")
		}
	}
	// save a reference to the stack's controller
	h.ext.Status.ControllerRef = meta.ReferenceTo(d, gvk)

	return nil
}

func (h *stackHandler) processJob(ctx context.Context) error {
	controllerJob := h.ext.Spec.Controller.Job
	if controllerJob == nil {
		return nil
	}

	// ensure the job is set to use this stack's service account that we created
	jobSpec := *controllerJob.Spec.DeepCopy()
	jobSpec.Template.Spec.ServiceAccountName = h.ext.Name

	labels := stacks.ParentLabels(h.ext)
	gvk := schema.GroupVersionKind{
		Group:   batch.GroupName,
		Kind:    "Job",
		Version: batch.SchemeGroupVersion.Version,
	}
	j := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerJob.Name,
			Namespace: h.ext.Namespace,
			Labels:    labels,
		},
		Spec: jobSpec,
	}

	var saRef corev1.ObjectReference
	var saSecretRef corev1.ObjectReference
	if h.hostAwareConfig != nil {
		// We need to copy SA token secret from host to tenant
		saRef = corev1.ObjectReference{
			Name:      j.Spec.Template.Spec.ServiceAccountName,
			Namespace: j.Namespace,
		}
		saSecretRef = h.hostAwareConfig.ObjectReferenceOnHost(saRef.Name, saRef.Namespace)
		err := h.prepareHostAwareJob(j, saSecretRef.Name)
		if err != nil {
			return errors.Wrap(err, "failed process host aware stack controller job")
		}
	}

	if err := h.hostKube.Create(ctx, j); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create job")
	}

	if h.hostAwareConfig != nil {
		owner := meta.AsOwner(meta.ReferenceTo(j, gvk))
		err := h.syncSATokenSecret(ctx, owner, saRef, saSecretRef)
		if err != nil {
			return errors.Wrap(err, "failed sync stack controller service account secret")
		}
	}
	// save a reference to the stack's controller
	h.ext.Status.ControllerRef = meta.ReferenceTo(j, gvk)

	return nil
}

// delete performs clean up (finalizer) actions when a Stack is being deleted.
// This function ensures that all the resources (ClusterRoles,
// ClusterRoleBindings) that this Stack owns are also cleaned up.
func (h *stackHandler) delete(ctx context.Context) (reconcile.Result, error) {
	log.V(logging.Debug).Info("deleting stack", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	labels := stacks.ParentLabels(h.ext)

	stackControllerNamespace := h.ext.GetNamespace()
	if h.hostAwareConfig != nil {
		stackControllerNamespace = h.hostAwareConfig.HostControllerNamespace
	}

	log.V(logging.Debug).Info("deleting stack controller deployment", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	if err := h.hostKube.DeleteAllOf(ctx, &apps.Deployment{}, client.MatchingLabels(labels), client.InNamespace(stackControllerNamespace)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	log.V(logging.Debug).Info("deleting stack controller jobs", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	if err := h.hostKube.DeleteAllOf(ctx, &batch.Job{}, client.MatchingLabels(labels), client.InNamespace(stackControllerNamespace)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	log.V(logging.Debug).Info("deleting stack clusterroles", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	if err := h.kube.DeleteAllOf(ctx, &rbacv1.ClusterRole{}, client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	log.V(logging.Debug).Info("deleting stack clusterrolebindings", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	if err := h.kube.DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	log.V(logging.Debug).Info("removing stack finalizer", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	meta.RemoveFinalizer(h.ext, stacksFinalizer)
	if err := h.kube.Update(ctx, h.ext); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	return reconcile.Result{}, nil
}

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i *v1alpha1.Stack, err error) (reconcile.Result, error) {
	log.V(logging.Debug).Info("failed to reconcile Stack", "namespace", i.GetNamespace(), "name", i.GetName(), "error", err)

	i.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
