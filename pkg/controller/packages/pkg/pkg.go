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

package pkg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	runtimeresource "github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
	"github.com/crossplane/crossplane/pkg/controller/packages/hosted"
	"github.com/crossplane/crossplane/pkg/packages"
	"github.com/crossplane/crossplane/pkg/packages/truncate"
)

const (
	packagesFinalizer             = "finalizer.packages.crossplane.io"
	labelValueNamespaceMember     = "true"
	labelValueAggregationEnabled  = "true"
	labelValueActiveParentPackage = "true"

	reconcileTimeout      = 1 * time.Minute
	requeueAfterOnSuccess = 10 * time.Second

	saVolumeName      = "sa-token"
	envK8SServiceHost = "KUBERNETES_SERVICE_HOST"
	envK8SServicePort = "KUBERNETES_SERVICE_PORT"
	envPodNamespace   = "POD_NAMESPACE"
	saMountPath       = "/var/run/secrets/kubernetes.io/serviceaccount"

	errHostAwareModeNotEnabled            = "host aware mode is not enabled"
	errFailedToPrepareHostAwareDeployment = "failed to prepare host aware package controller deployment"
	errFailedToCreateDeployment           = "failed to create deployment"
	errFailedToGenerateSecretNames        = "failed to generate host secret names"

	errFailedToGetDeployment                    = "failed to get deployment"
	errFailedToSyncSASecret                     = "failed to sync package controller service account secret"
	errFailedToSyncImagePullSecrets             = "failed to sync package controller image pull secrets"
	errServiceAccountNotFound                   = "service account is not found (not created yet?)"
	errFailedToGetServiceAccount                = "failed to get service account"
	errServiceAccountTokenSecretNotGeneratedYet = "service account token secret is not generated yet"
	errFailedToGetServiceAccountTokenSecret     = "failed to get service account token secret"
	errFailedToCreateTokenSecret                = "failed to create sa token secret on target Kubernetes"
)

var (
	resultRequeue    = reconcile.Result{Requeue: true}
	requeueOnSuccess = reconcile.Result{RequeueAfter: requeueAfterOnSuccess}

	roleVerbs = map[string][]string{
		"admin": {"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"},
		"edit":  {"get", "list", "watch", "create", "delete", "deletecollection", "patch", "update"},
		"view":  {"get", "list", "watch"},
	}

	disableAutoMount = false

	allowPrivilegeEscalation = false
	privileged               = false
	runAsNonRoot             = true
)

// Reconciler reconciles a Instance object
type Reconciler struct {
	// kube is controller runtime client for resource (a.k.a tenant) Kubernetes where all custom resources live.
	kube client.Client
	// hostKube is controller runtime client for workload (a.k.a host)
	// Kubernetes where jobs for package installs and package controller deployments
	// created.
	hostKube             client.Client
	hostedConfig         *hosted.Config
	allowCore            bool
	allowFullDeployment  bool
	forceImagePullPolicy string
	log                  logging.Logger
	factory
}

// Setup adds a controller that reconciles Packages.
func Setup(mgr ctrl.Manager, l logging.Logger, hostControllerNamespace string, allowCore, allowFullDeployment bool, forceImagePullPolicy string) error {
	name := "packages/" + strings.ToLower(v1alpha1.PackageGroupKind)

	hostKube, _, err := hosted.GetClients()
	if err != nil {
		return err
	}

	hc, err := hosted.NewConfigForHost(hostControllerNamespace, mgr.GetConfig().Host)
	if err != nil {
		return err
	}

	r := &Reconciler{
		kube:                 mgr.GetClient(),
		hostKube:             hostKube,
		hostedConfig:         hc,
		allowCore:            allowCore,
		allowFullDeployment:  allowFullDeployment,
		forceImagePullPolicy: forceImagePullPolicy,
		factory:              &packageHandlerFactory{},
		log:                  l.WithValues("controller", name),
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.Package{}).
		Complete(r)
}

// Reconcile reads that state of the Package for a Instance object and makes changes based on the state read
// and what is in the Instance.Spec
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	r.log.Debug("Reconciling", "request", req)

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	// fetch the CRD instance
	p := &v1alpha1.Package{}
	if err := r.kube.Get(ctx, req.NamespacedName, p); err != nil {
		if kerrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	meta.AddFinalizer(p, packagesFinalizer)
	err := r.kube.Update(ctx, p)
	if err != nil {
		return fail(ctx, r.kube, p, err)
	}

	handler := r.factory.newHandler(r.log, p, r.kube, r.hostKube, r.hostedConfig, r.allowCore, r.allowFullDeployment, r.forceImagePullPolicy)

	if meta.WasDeleted(p) {
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

type packageHandler struct {
	// kube is controller runtime client for resource (a.k.a tenant) Kubernetes where all custom resources live.
	kube client.Client
	// hostKube is controller runtime client for workload (a.k.a host)
	// Kubernetes where jobs for package installs and package controller deployments
	// created.
	hostKube             client.Client
	hostAwareConfig      *hosted.Config
	allowCore            bool
	allowFullDeployment  bool
	forceImagePullPolicy string
	ext                  *v1alpha1.Package
	log                  logging.Logger
}

type factory interface {
	newHandler(logging.Logger, *v1alpha1.Package, client.Client, client.Client, *hosted.Config, bool, bool, string) handler
}

type packageHandlerFactory struct{}

func (f *packageHandlerFactory) newHandler(log logging.Logger, ext *v1alpha1.Package, kube client.Client, hostKube client.Client, hostAwareConfig *hosted.Config, allowCore, allowFullDeployment bool, forceImagePullPolicy string) handler {
	return &packageHandler{
		kube:                 kube,
		hostKube:             hostKube,
		hostAwareConfig:      hostAwareConfig,
		allowCore:            allowCore,
		allowFullDeployment:  allowFullDeployment,
		forceImagePullPolicy: forceImagePullPolicy,
		ext:                  ext,
		log:                  log,
	}
}

// ************************************************************************************************
// Syncing/Creating functions
// ************************************************************************************************
func (h *packageHandler) sync(ctx context.Context) (reconcile.Result, error) {
	if h.ext.Status.ControllerRef == nil {
		return h.create(ctx)
	}

	return h.update(ctx)
}

func (h *packageHandler) create(ctx context.Context) (reconcile.Result, error) {
	h.ext.Status.SetConditions(runtimev1alpha1.Creating())

	if err := h.validatePackagePermissions(); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	// create RBAC permissions
	if err := h.processRBAC(ctx); err != nil {
		h.log.Debug("failed to create RBAC permissions", "error", err)
		return fail(ctx, h.kube, h.ext, err)
	}

	crdHandlers := []crdHandler{
		h.createListFulfilledCRDHandler(),
		h.createNamespaceLabelsCRDHandler(),
		h.createMultipleParentLabelsCRDHandler(),
		h.createPersonaClusterRolesCRDHandler(),
	}

	if err := h.processCRDs(ctx, crdHandlers...); err != nil {
		h.log.Debug("failed to process package CRDs", "error", err)
		return fail(ctx, h.kube, h.ext, err)
	}

	// create controller deployment
	if err := h.processDeployment(ctx); err != nil {
		h.log.Debug("failed to create deployment", "error", err)
		return fail(ctx, h.kube, h.ext, err)
	}

	// the package has successfully been created, the package is ready
	h.ext.Status.SetConditions(runtimev1alpha1.Available(), runtimev1alpha1.ReconcileSuccess())
	return requeueOnSuccess, h.kube.Status().Update(ctx, h.ext)
}

func (h *packageHandler) update(ctx context.Context) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func copyLabels(labels map[string]string) map[string]string {
	labelsCopy := map[string]string{}
	for k, v := range labels {
		labelsCopy[k] = v
	}
	return labelsCopy
}

// crdListFulfillsPackage verifies that all CRDs provided by the package are present
// among the given crds.  Error will describe a mismatch, if any. Extra CRDs in
// the crds will be ignored.
func crdListFulfilled(want v1alpha1.CRDList, got []apiextensions.CustomResourceDefinition) error {
	for _, crdWant := range want {
		group := crdWant.GroupVersionKind().Group
		version := crdWant.GroupVersionKind().Version
		kind := crdWant.GroupVersionKind().Kind

		found := false
		for i := range got {
			if got[i].Spec.Group != group {
				continue
			}

			if got[i].Spec.Names.Kind != kind {
				continue
			}

			if crdVersionExists(&got[i], version) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Missing CRD with APIVersion %q and Kind %q", crdWant.APIVersion, crdWant.Kind)
		}
	}
	return nil
}

func crdVersionExists(crd *apiextensions.CustomResourceDefinition, version string) bool {
	for _, v := range crd.Spec.Versions {
		if v.Name == version {
			return true
		}
	}
	return false
}

// crdsFromPackage API fetches the CRDs of the Package
//
// The CRDs returned by this method represent those both provided the Package spec
// and present in the API. Use crdListFulfilled to ensure all CRDs are accounted
// for. Errors will only result from API queries.
func (h *packageHandler) crdsFromPackage(ctx context.Context) ([]apiextensions.CustomResourceDefinition, error) {
	results := []apiextensions.CustomResourceDefinition{}
	crds := &apiextensions.CustomResourceDefinitionList{}

	if len(h.ext.Spec.CRDs) == 0 {
		return results, nil
	}

	// fake client used during testing doesn't work with nil
	listOpts := &client.ListOptions{}

	if err := h.kube.List(ctx, crds, listOpts); err != nil {
		return nil, errors.Wrap(err, "CRDs could not be listed")
	}

	for _, crdWant := range h.ext.Spec.CRDs {
		group := crdWant.GroupVersionKind().Group
		version := crdWant.GroupVersionKind().Version
		kind := crdWant.GroupVersionKind().Kind

		for i := range crds.Items {
			if crds.Items[i].Spec.Group != group ||
				crds.Items[i].Spec.Names.Kind != kind ||
				!crdVersionExists(&crds.Items[i], version) {
				continue
			}
			results = append(results, crds.Items[i])
		}
	}

	return results, nil
}

type crdHandler func(ctx context.Context, crds []apiextensions.CustomResourceDefinition) error

func (h *packageHandler) processCRDs(ctx context.Context, crdHandlers ...crdHandler) error {
	crds, err := h.crdsFromPackage(ctx)
	if err != nil {
		return err
	}

	for _, handler := range crdHandlers {
		if err := handler(ctx, crds); err != nil {
			return err
		}
	}
	return nil
}

// createListFulfilledCRDHandler provides a handler which verifies all
// Package expected CRDs are present in the provided list
func (h *packageHandler) createListFulfilledCRDHandler() crdHandler {
	return func(_ context.Context, crds []apiextensions.CustomResourceDefinition) error {
		return crdListFulfilled(h.ext.Spec.CRDs, crds)
	}
}

// createNamespaceLabelsCRDHandler provides a handler which labels CRDs with the
// namespaces of the packages they are managed by.
func (h *packageHandler) createNamespaceLabelsCRDHandler() crdHandler {
	labelNamespace := fmt.Sprintf(packages.LabelNamespaceFmt, h.ext.GetNamespace())
	return func(ctx context.Context, crds []apiextensions.CustomResourceDefinition) error {
		for i := range crds {
			labels := crds[i].GetLabels()

			if labels[packages.LabelKubernetesManagedBy] != packages.LabelValuePackageManager {
				continue
			}

			if labels[labelNamespace] == labelValueNamespaceMember {
				continue
			}

			crdPatch := client.MergeFrom(crds[i].DeepCopy())

			labels[labelNamespace] = labelValueNamespaceMember
			crds[i].SetLabels(labels)

			h.log.Debug("adding labels for CRD", "labelNamespace", labelNamespace, "name", crds[i].GetName())
			if err := h.kube.Patch(ctx, &crds[i], crdPatch); err != nil {
				return err
			}
		}
		return nil
	}
}

// createMultipleParentLabelsCRDHandler provides a handler which labels CRDs
// with the packages they are managed by. This will allow for a single Namespaced
// package to be installed in multiple namespaces, or different packages (possibly
// only differing by versions) to provide the same CRDs without the risk that a
// single PackageInstall removal will delete a CRD until there are no remaining
// package parent labels.
func (h *packageHandler) createMultipleParentLabelsCRDHandler() crdHandler {
	labelMultiParent := packages.MultiParentLabel(h.ext)

	return func(ctx context.Context, crds []apiextensions.CustomResourceDefinition) error {
		for i := range crds {
			labels := crds[i].GetLabels()

			if labels[packages.LabelKubernetesManagedBy] != packages.LabelValuePackageManager {
				continue
			}

			if labels[labelMultiParent] == labelValueActiveParentPackage {
				continue
			}

			crdPatch := client.MergeFrom(crds[i].DeepCopy())

			labels[labelMultiParent] = labelValueActiveParentPackage
			crds[i].SetLabels(labels)

			h.log.Debug("adding labels for CRD", "labelMultiParent", labelMultiParent, "name", crds[i].GetName())
			if err := h.kube.Patch(ctx, &crds[i], crdPatch); err != nil {
				return err
			}
		}
		return nil
	}
}

// createPersonaClusterRolesCRDHandler provides a handler which creates admin,
// edit, and view clusterroles that are namespace+package+version specific
func (h *packageHandler) createPersonaClusterRolesCRDHandler() crdHandler {
	labels := packages.ParentLabels(h.ext)

	return func(ctx context.Context, crds []apiextensions.CustomResourceDefinition) error {

		for persona := range roleVerbs {
			name := packages.PersonaRoleName(h.ext, persona)

			// Use a copy so AddLabels doesn't mutate labels
			labelsCopy := copyLabels(labels)

			// Create labels appropriate for the scope of the ClusterRole
			var crossplaneScope string

			if h.isNamespaced() {
				crossplaneScope = packages.NamespaceScoped

				labelNamespace := fmt.Sprintf(packages.LabelNamespaceFmt, h.ext.GetNamespace())
				labelsCopy[labelNamespace] = labelValueNamespaceMember
			} else {
				crossplaneScope = packages.EnvironmentScoped
			}

			// Aggregation labels grant Package CRD responsibilities
			// to namespace or environment personas like crossplane-env-admin
			// or crossplane:ns:default:view
			aggregationLabel := fmt.Sprintf(packages.LabelAggregateFmt, crossplaneScope, persona)
			labelsCopy[aggregationLabel] = labelValueAggregationEnabled

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

}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// policiesEqual verifies that two RBAC policy rules match
//
// TODO(displague) Any difference in the order of the policies array fields
// will result in a mismatch. This is suitable for permitting RBAC rules
// expected to have come from 'package unpack', but may need to be revisited if
// other tools are creating Package resources.
func policiesEqual(a, b rbacv1.PolicyRule) bool {
	return stringSlicesEqual(a.APIGroups, b.APIGroups) &&
		stringSlicesEqual(a.NonResourceURLs, b.NonResourceURLs) &&
		stringSlicesEqual(a.ResourceNames, b.ResourceNames) &&
		stringSlicesEqual(a.Resources, b.Resources) &&
		stringSlicesEqual(a.Verbs, b.Verbs)
}

func isDefaultPackagePolicy(rule rbacv1.PolicyRule) bool {
	for _, permitted := range packages.PackageCoreRBACRules {
		if policiesEqual(rule, permitted) {
			return true
		}
	}
	return false
}

func isPermittedAPIGroup(apigroup string) bool {
	return strings.Contains(apigroup, ".") &&
		!strings.HasSuffix(apigroup, "k8s.io")
}

func isPermittedPackagePolicy(rule rbacv1.PolicyRule) bool {
	if isDefaultPackagePolicy(rule) {
		return true
	}

	if len(rule.NonResourceURLs) > 0 {
		return false
	}

	if len(rule.APIGroups) == 0 {
		return false
	}

	for _, g := range rule.APIGroups {
		if !isPermittedAPIGroup(g) {
			return false
		}
	}
	return true
}

func (h *packageHandler) validatePackagePermissions() error {
	if !h.allowCore {
		for _, rule := range h.ext.Spec.Permissions.Rules {
			if !isPermittedPackagePolicy(rule) {
				h.log.Debug("restricted rule found in package spec permissions", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName(), "rule", rule)
				return errors.Errorf("permissions contain a restricted rule")
			}
		}
	}

	return nil
}

func (h *packageHandler) createDeploymentClusterRole(ctx context.Context, labels map[string]string) (string, error) {
	name := packages.PersonaRoleName(h.ext, "system")
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

func (h *packageHandler) createNamespacedRoleBinding(ctx context.Context, clusterRoleName string, owner metav1.OwnerReference) error {
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

func (h *packageHandler) createClusterRoleBinding(ctx context.Context, clusterRoleName string, labels map[string]string) error {
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

func (h *packageHandler) processRBAC(ctx context.Context) error {
	if len(h.ext.Spec.Permissions.Rules) == 0 {
		return nil
	}

	owner := meta.AsOwner(meta.ReferenceTo(h.ext, v1alpha1.PackageGroupVersionKind))

	// create service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            h.ext.Name,
			Namespace:       h.ext.Namespace,
			OwnerReferences: []metav1.OwnerReference{owner},
			Annotations:     h.ext.Spec.ServiceAccountAnnotations(),
		},
	}

	if err := h.kube.Create(ctx, sa); err != nil && !kerrors.IsAlreadyExists(err) {
		return errors.Wrap(err, "failed to create service account")
	}

	labels := packages.ParentLabels(h.ext)

	clusterRoleName, err := h.createDeploymentClusterRole(ctx, labels)
	if err != nil {
		return err
	}

	// give the SA rolebindings to run the the package's controller
	var roleBindingErr error

	switch apiextensions.ResourceScope(h.ext.Spec.PermissionScope) {
	case apiextensions.ClusterScoped:
		roleBindingErr = h.createClusterRoleBinding(ctx, clusterRoleName, labels)
	case "", apiextensions.NamespaceScoped:
		roleBindingErr = h.createNamespacedRoleBinding(ctx, clusterRoleName, owner)

	default:
		roleBindingErr = errors.New("invalid permissionScope for package")
	}

	return roleBindingErr

}

func (h *packageHandler) isNamespaced() bool {
	switch apiextensions.ResourceScope(h.ext.Spec.PermissionScope) {
	case apiextensions.NamespaceScoped, apiextensions.ResourceScope(""):
		return true
	}
	return false
}

// syncSATokenSecret function copies service account token secret from custom resource Kubernetes (a.k.a tenant
// Kubernetes) to Host Cluster. This secret then mounted to package controller pods so that they can authenticate.
func (h *packageHandler) syncSATokenSecret(ctx context.Context, owner metav1.OwnerReference,
	fromSARef corev1.ObjectReference, toSecretRef corev1.ObjectReference) error {
	// Get the ServiceAccount
	fromKube := h.kube
	toKube := h.hostKube

	sa := corev1.ServiceAccount{}
	err := fromKube.Get(ctx, client.ObjectKey{
		Name:      fromSARef.Name,
		Namespace: fromSARef.Namespace,
	}, &sa)

	if kerrors.IsNotFound(err) {
		return errors.Wrap(err, errServiceAccountNotFound)
	}
	if err != nil {
		return errors.Wrap(err, errFailedToGetServiceAccount)
	}
	if len(sa.Secrets) < 1 {
		return errors.New(errServiceAccountTokenSecretNotGeneratedYet)
	}
	saSecretRef := sa.Secrets[0]
	saSecretRef.Namespace = fromSARef.Namespace
	saSecret := corev1.Secret{}

	err = fromKube.Get(ctx, meta.NamespacedNameOf(&saSecretRef), &saSecret)

	if err != nil {
		return errors.Wrap(err, errFailedToGetServiceAccountTokenSecret)
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
		return errors.Wrap(err, errFailedToCreateTokenSecret)
	}

	return nil
}

// prepareHostAwarePodSpec modifies input pod spec as follows, such that it communicates with custom resource
// Kubernetes Apiserver (a.k.a. tenant Kubernetes) rather than the apiserver of the Kubernetes Cluster where the pod
// runs inside (a.k.a. Host Cluster):
// - Set KUBERNETES_SERVICE_HOST
// - Set KUBERNETES_SERVICE_PORT
// - Disabled automountServiceAccountToken
// - Unset serviceAccountName
// - Mount service account token secret which is copied from custom resource Kubernetes apiserver
func (h *packageHandler) prepareHostAwarePodSpec(tokenSecret string, ps *corev1.PodSpec) error {
	if h.hostAwareConfig == nil {
		return errors.New(errHostAwareModeNotEnabled)
	}
	// Opt out service account token automount
	ps.AutomountServiceAccountToken = &disableAutoMount
	ps.ServiceAccountName = ""
	ps.DeprecatedServiceAccount = ""

	ps.Volumes = append(ps.Volumes, corev1.Volume{
		Name: saVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: tokenSecret,
			},
		},
	})
	for i := range ps.Containers {
		ps.Containers[i].Env = append(ps.Containers[i].Env,
			corev1.EnvVar{
				Name:  envK8SServiceHost,
				Value: h.hostAwareConfig.TenantAPIServiceHost,
			}, corev1.EnvVar{
				Name:  envK8SServicePort,
				Value: h.hostAwareConfig.TenantAPIServicePort,
			}, corev1.EnvVar{
				// When POD_NAMESPACE is not set as packageinstalls namespace here, it is set as host namespace where actual
				// pod running. This result package controller to fail with forbidden, since their sa only allows to watch
				// the namespace where package is installed
				Name:  envPodNamespace,
				Value: h.ext.Namespace,
			})

		ps.Containers[i].VolumeMounts = append(ps.Containers[i].VolumeMounts, corev1.VolumeMount{
			Name:      saVolumeName,
			ReadOnly:  true,
			MountPath: saMountPath,
		})
	}

	return nil
}

func (h *packageHandler) hostSARefs(d *apps.Deployment) (corev1.ObjectReference, corev1.ObjectReference) {
	var saRef corev1.ObjectReference
	var saSecretRef corev1.ObjectReference

	// We need to copy SA token secret from host to tenant
	saRef = corev1.ObjectReference{
		Name:      d.Spec.Template.Spec.ServiceAccountName,
		Namespace: d.Namespace,
	}

	if h.hostAwareConfig != nil {
		saSecretRef = h.hostAwareConfig.ObjectReferenceOnHost(saRef.Name, saRef.Namespace)
	} else {
		saSecretRef = saRef
	}
	return saRef, saSecretRef
}

func (h *packageHandler) prepareHostAwareDeployment(d *apps.Deployment, tokenSecret string) error {
	if h.hostAwareConfig == nil {
		return nil
	}

	err := h.prepareHostAwarePodSpec(tokenSecret, &d.Spec.Template.Spec)
	if err != nil {
		return errors.Wrap(err, errFailedToPrepareHostAwareDeployment)
	}

	d.Spec.Template.Spec.ImagePullSecrets, err = hosted.ImagePullSecretsOnHost(d.Namespace, d.Spec.Template.Spec.ImagePullSecrets)
	if err != nil {
		return errors.Wrap(err, errFailedToGenerateSecretNames)
	}

	o := h.hostAwareConfig.ObjectReferenceOnHost(d.Name, d.Namespace)
	d.Name = o.Name
	d.Namespace = o.Namespace

	a := hosted.ObjectReferenceAnnotationsOnHost("package", h.ext.GetName(), h.ext.GetNamespace())
	meta.AddAnnotations(d, a)
	return nil
}

func (h *packageHandler) prepareDeployment(d *apps.Deployment) {
	controllerDeployment := h.ext.Spec.Controller.Deployment
	if controllerDeployment == nil {
		return
	}

	controllerDeployment.Spec.DeepCopyInto(&d.Spec)

	// force the deployment to use package opinionated names and service account
	suffix := "-controller"
	size := truncate.LabelValueLength - len(suffix)
	name, _ := truncate.Truncate(h.ext.Name, size, truncate.DefaultSuffixLength)
	name += suffix
	matchLabels := map[string]string{"app": name}
	labels := packages.ParentLabels(h.ext)

	d.SetName(name)
	d.SetNamespace(h.ext.Namespace)
	meta.AddLabels(d, labels)

	d.Spec.Template.Spec.ServiceAccountName = h.ext.Name

	if !h.allowFullDeployment {
		d.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: &runAsNonRoot,
		}
	}

	for _, c := range [][]corev1.Container{
		d.Spec.Template.Spec.Containers,
		d.Spec.Template.Spec.InitContainers,
	} {
		for i := range c {
			if h.forceImagePullPolicy != "" {
				c[i].ImagePullPolicy = corev1.PullPolicy(h.forceImagePullPolicy)
			}
			if !h.allowFullDeployment {
				c[i].SecurityContext = &corev1.SecurityContext{
					AllowPrivilegeEscalation: &allowPrivilegeEscalation,
					Privileged:               &privileged,
					RunAsNonRoot:             &runAsNonRoot,
				}
			}
		}
	}

	d.Spec.Template.SetName(name)
	meta.AddLabels(&d.Spec.Template, matchLabels)
	d.Spec.Selector = &metav1.LabelSelector{MatchLabels: matchLabels}
}

func (h *packageHandler) processDeployment(ctx context.Context) error {
	if h.ext.Spec.Controller.Deployment == nil {
		return nil
	}

	var err error
	d := &apps.Deployment{}
	h.prepareDeployment(d)
	saRef, saSecretRef := h.hostSARefs(d)

	if err = h.prepareHostAwareDeployment(d, saSecretRef.Name); err != nil {
		return err
	}

	err = h.hostKube.Get(ctx, types.NamespacedName{Name: d.GetName(), Namespace: d.GetNamespace()}, d)

	if err != nil {
		if kerrors.IsNotFound(err) {
			if err := h.hostKube.Create(ctx, d); err != nil {
				return errors.Wrap(err, errFailedToCreateDeployment)
			}
		} else {
			return errors.Wrap(err, errFailedToGetDeployment)
		}
	}

	gvk := apps.SchemeGroupVersion.WithKind("Deployment")
	d.SetGroupVersionKind(gvk)

	if err := h.finalizeAwareHostDeployment(ctx, d, saRef, saSecretRef); err != nil {
		return err
	}

	// save a reference to the package's controller
	h.ext.Status.ControllerRef = meta.ReferenceTo(d, gvk)
	return nil
}

// finalizeAwareHostDeployment copies tenant secrets, including image pull
// secrets and service accounts tokens, to the host for use in the deployment.
// The supplied deployment is modified to operate in the host environment.
func (h *packageHandler) finalizeAwareHostDeployment(ctx context.Context, d *apps.Deployment, saRef, saSecretRef corev1.ObjectReference) error {
	if h.hostAwareConfig == nil {
		return nil
	}

	err := hosted.SyncImagePullSecrets(ctx, h.kube, h.hostKube, h.ext.GetNamespace(), h.ext.Spec.Controller.Deployment.Spec.Template.Spec.ImagePullSecrets, d.Spec.Template.Spec.ImagePullSecrets, d)
	if err != nil {
		return errors.Wrap(err, errFailedToSyncImagePullSecrets)
	}

	owner := meta.AsOwner(meta.ReferenceTo(d, d.GroupVersionKind()))
	err = h.syncSATokenSecret(ctx, owner, saRef, saSecretRef)

	return errors.Wrap(err, errFailedToSyncSASecret)
}

// delete performs clean up (finalizer) actions when a Package is being deleted.
// This function ensures that all the resources (ClusterRoles,
// ClusterRoleBindings) that this Package owns are also cleaned up.
func (h *packageHandler) delete(ctx context.Context) (reconcile.Result, error) {
	h.log.Debug("deleting package", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())

	labels := packages.ParentLabels(h.ext)
	packageControllerNamespace := h.ext.GetNamespace()
	if h.hostAwareConfig != nil {
		packageControllerNamespace = h.hostAwareConfig.HostControllerNamespace
	}

	if err := h.hostKube.DeleteAllOf(ctx, &apps.Deployment{}, client.MatchingLabels(labels), client.InNamespace(packageControllerNamespace)); runtimeresource.IgnoreNotFound(err) != nil {
		h.log.Debug("deleting package controller deployment", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.hostKube.DeleteAllOf(ctx, &batch.Job{}, client.MatchingLabels(labels), client.InNamespace(packageControllerNamespace)); runtimeresource.IgnoreNotFound(err) != nil {
		h.log.Debug("deleting package controller jobs", "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.kube.DeleteAllOf(ctx, &rbacv1.ClusterRole{}, client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		h.log.Debug("failed to delete package clusterroles", "error", err, "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.kube.DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(labels)); runtimeresource.IgnoreNotFound(err) != nil {
		h.log.Debug("failed to delete package clusterrolebindings", "error", err, "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())
		return fail(ctx, h.kube, h.ext, err)
	}

	if err := h.removeCRDLabels(ctx); err != nil {
		return fail(ctx, h.kube, h.ext, err)
	}

	meta.RemoveFinalizer(h.ext, packagesFinalizer)
	if err := h.kube.Update(ctx, h.ext); err != nil {
		h.log.Debug("failed to remove package finalizer", "error", err, "namespace", h.ext.GetNamespace(), "name", h.ext.GetName())
		return fail(ctx, h.kube, h.ext, err)
	}

	return reconcile.Result{}, nil
}

// removeCRDLabels Removes all labels added to CRDs by this Package, leaving the
// CRDs and labels left by other packages in place
// TODO(displague) if single-parent labels exist, matching this package,
// delete them
func (h *packageHandler) removeCRDLabels(ctx context.Context) error {
	// crds may be an incomplete list if CRDs were manually removed.
	// This is ok since we won't need to remove labels on deleted crds.
	// If crds are omitted based on other differences (mismatched
	// versions) those crds are in a weird state and will be missed here.
	crds, err := h.crdsFromPackage(ctx)
	if err != nil {
		h.log.Debug("failed to fetch CRDs for package", "error", err, "name", h.ext.GetName())
		return err
	}

	packageNS := h.ext.GetNamespace()

	labelMultiParentNSPrefix := packages.MultiParentLabelPrefix(h.ext)
	labelMultiParent := packages.MultiParentLabel(h.ext)
	labelNamespace := fmt.Sprintf(packages.LabelNamespaceFmt, packageNS)

	for i := range crds {
		name := crds[i].GetName()
		labels := crds[i].GetLabels()
		if labels[packages.LabelKubernetesManagedBy] != packages.LabelValuePackageManager {
			h.log.Debug("skipping package label removal for unmanaged CRD", "name", name)

			continue
		}

		h.log.Debug("removing labels from CRD", "labelMultiParent", labelMultiParent, "labelNamespace", labelNamespace, "name", name)

		crdPatch := client.MergeFrom(crds[i].DeepCopy())

		meta.RemoveLabels(&crds[i], labelMultiParent)

		// TODO(displague) remove matching single parent labels? should we pick
		// another parent at random? what value is there in these labels?

		// clear the namespace label after the last parent package is removed
		if !packages.HasPrefixedLabel(&crds[i], labelMultiParentNSPrefix) {
			meta.RemoveLabels(&crds[i], labelNamespace)
		}

		if err := h.kube.Patch(ctx, &crds[i], crdPatch); err != nil {
			h.log.Debug("failed to patch CRD", "error", err, "name", name)

			return err
		}
	}
	return nil
}

// fail - helper function to set fail condition with reason and message
func fail(ctx context.Context, kube client.StatusClient, i *v1alpha1.Package, err error) (reconcile.Result, error) {
	i.Status.SetConditions(runtimev1alpha1.ReconcileError(err))
	return resultRequeue, kube.Status().Update(ctx, i)
}
