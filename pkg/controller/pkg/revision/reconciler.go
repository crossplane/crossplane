/*
Copyright 2020 The Crossplane Authors.

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

package revision

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	apiextensionsv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	pkgmeta "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

const (
	reconcileTimeout = 1 * time.Minute

	shortWait = 30 * time.Second
	longWait  = 1 * time.Minute
)

const (
	errGetPackageRevision = "cannot get package revision"
	errUpdateStatus       = "cannot update package revision status"

	errInitParserBackend = "cannot initialize parser backend"
	errParseLogs         = "cannot parse pod logs"

	errDeleteProviderDeployment = "cannot delete provider package deployment"
	errDeleteProviderSA         = "cannot delete provider package service account"
	errCreateProviderDeployment = "cannot create provider package deployment"
	errCreateProviderSA         = "cannot create provider package service account"

	errEstablishControl = "cannot establish control of object"
)

// Event reasons.
const (
	reasonParse   event.Reason = "ParsePackage"
	reasonLint    event.Reason = "LintPackage"
	reasonInstall event.Reason = "InstallPackage"
)

// ReconcilerOption is used to configure the Reconciler.
type ReconcilerOption func(*Reconciler)

// WithInstallNamespace determines the namespace where install jobs will be
// created.
func WithInstallNamespace(n string) ReconcilerOption {
	return func(r *Reconciler) {
		r.namespace = n
	}
}

// WithNewPackageRevisionFn determines the type of package being reconciled.
func WithNewPackageRevisionFn(f func() v1alpha1.PackageRevision) ReconcilerOption {
	return func(r *Reconciler) {
		r.newPackageRevision = f
	}
}

// WithLogger specifies how the Reconciler should log messages.
func WithLogger(log logging.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		r.log = log
	}
}

// WithRecorder specifies how the Reconciler should record Kubernetes events.
func WithRecorder(er event.Recorder) ReconcilerOption {
	return func(r *Reconciler) {
		r.record = er
	}
}

// WithEstablisher specifies how the Reconciler should establish package resources.
func WithEstablisher(e Establisher) ReconcilerOption {
	return func(r *Reconciler) {
		r.establisher = e
	}
}

// WithParser specifies how the Reconciler should parse a package.
func WithParser(p parser.Parser) ReconcilerOption {
	return func(r *Reconciler) {
		r.parser = p
	}
}

// WithParserBackend specifies how the Reconciler should parse a package.
func WithParserBackend(p parser.Backend) ReconcilerOption {
	return func(r *Reconciler) {
		r.backend = p
	}
}

// WithLinter specifies how the Reconciler should parse a package.
func WithLinter(l Linter) ReconcilerOption {
	return func(r *Reconciler) {
		r.linter = l
	}
}

// BuildMetaScheme builds the default scheme used for identifying metadata in a
// Crossplane package.
func BuildMetaScheme() (*runtime.Scheme, error) {
	metaScheme := runtime.NewScheme()
	if err := pkgmeta.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		return nil, err
	}
	return metaScheme, nil
}

// BuildObjectScheme builds the default scheme used for identifying objects in a
// Crossplane package.
func BuildObjectScheme() (*runtime.Scheme, error) {
	objScheme := runtime.NewScheme()
	if err := apiextensionsv1alpha1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	if err := v1beta1.AddToScheme(objScheme); err != nil {
		return nil, err
	}
	return objScheme, nil
}

// Reconciler reconciles packages.
type Reconciler struct {
	namespace    string
	client       resource.ClientApplicator
	establisher  Establisher
	podLogClient kubernetes.Interface
	parser       parser.Parser
	linter       Linter
	backend      parser.Backend
	log          logging.Logger
	record       event.Recorder

	newPackageRevision func() v1alpha1.PackageRevision
}

// SetupProviderRevision adds a controller that reconciles ProviderRevisions.
func SetupProviderRevision(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ProviderRevisionGroupKind)
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize host clientset with in cluster config")
	}

	metaScheme, err := BuildMetaScheme()
	if err != nil {
		return errors.New("cannot build meta scheme for package parser")
	}
	objScheme, err := BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}

	r := NewReconciler(mgr,
		clientset,
		WithInstallNamespace(namespace),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(parser.NewPodLogBackend(parser.PodNamespace(namespace))),
		WithLinter(NewPackageLinter([]PackageLinterFn{OneMeta}, []ObjectLinterFn{IsProvider}, []ObjectLinterFn{IsCRD})),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ProviderRevision{}).
		Complete(r)
}

// SetupConfigurationRevision adds a controller that reconciles ConfigurationRevisions.
func SetupConfigurationRevision(mgr ctrl.Manager, l logging.Logger, namespace string) error {
	name := "packages/" + strings.ToLower(v1alpha1.ConfigurationRevisionGroupKind)
	nr := func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }

	clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "failed to initialize host clientset with in cluster config")
	}

	metaScheme, err := BuildMetaScheme()
	if err != nil {
		return errors.New("cannot build meta scheme for package parser")
	}
	objScheme, err := BuildObjectScheme()
	if err != nil {
		return errors.New("cannot build object scheme for package parser")
	}

	r := NewReconciler(mgr,
		clientset,
		WithInstallNamespace(namespace),
		WithNewPackageRevisionFn(nr),
		WithParser(parser.New(metaScheme, objScheme)),
		WithParserBackend(parser.NewPodLogBackend(parser.PodNamespace(namespace))),
		WithLinter(NewPackageLinter([]PackageLinterFn{OneMeta}, []ObjectLinterFn{IsConfiguration}, []ObjectLinterFn{Or(IsXRD, IsComposition)})),
		WithLogger(l.WithValues("controller", name)),
		WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&v1alpha1.ConfigurationRevision{}).
		Complete(r)
}

// NewReconciler creates a new package reconciler.
func NewReconciler(mgr ctrl.Manager, clientset kubernetes.Interface, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		client: resource.ClientApplicator{
			Client:     mgr.GetClient(),
			Applicator: resource.NewAPIUpdatingApplicator(mgr.GetClient()),
		},
		establisher:  NewAPIEstablisher(mgr.GetClient()),
		podLogClient: clientset,
		parser:       parser.New(nil, nil),
		linter:       NewPackageLinter(nil, nil, nil),
		log:          logging.NewNopLogger(),
		record:       event.NewNopRecorder(),
	}

	for _, f := range opts {
		f(r)
	}

	return r
}

// Reconcile package revision.
func (r *Reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) { // nolint:gocyclo
	log := r.log.WithValues("request", req)
	log.Debug("Reconciling")

	ctx, cancel := context.WithTimeout(context.Background(), reconcileTimeout)
	defer cancel()

	pr := r.newPackageRevision()
	if err := r.client.Get(ctx, req.NamespacedName, pr); err != nil {
		// There's no need to requeue if we no longer exist. Otherwise we'll be
		// requeued implicitly because we return an error.
		log.Debug(errGetPackageRevision, "error", err)
		return reconcile.Result{}, errors.Wrap(resource.IgnoreNotFound(err), errGetPackageRevision)
	}

	log = log.WithValues(
		"uid", pr.GetUID(),
		"version", pr.GetResourceVersion(),
		"name", pr.GetName(),
	)

	// Initialize parser backend to obtain package contents.
	reader, err := r.backend.Init(ctx, parser.PodClient(r.podLogClient), parser.PodName(pr.GetInstallPod().Name))
	if err != nil {
		log.Debug(errInitParserBackend, "error", err)
		r.record.Event(pr, event.Warning(reasonParse, err))
		// Requeue after shortWait because we may be waiting for parent package
		// controller to recreate Pod.
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Parse package contents.
	pkg, err := r.parser.Parse(ctx, reader)
	if err != nil {
		log.Debug(errParseLogs, "error", err)
		r.record.Event(pr, event.Warning(reasonParse, err))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// Lint package.
	if err := r.linter.Lint(pkg); err != nil {
		r.record.Event(pr, event.Warning(reasonLint, err))
		// NOTE(hasheddan): a failed lint typically will require manual
		// intervention, but on the off chance that we read pod logs early,
		// which caused a linting failure, we will requeue after long wait.
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	desiredState := pr.GetDesiredState()

	// If revision is inactive and is a provider, we must make sure controller
	// is not running before we clean up resources.
	if pr.GetObjectKind().GroupVersionKind() == v1alpha1.ProviderRevisionGroupVersionKind && desiredState == v1alpha1.PackageRevisionInactive {
		// TODO(hasheddan): we are safe to 0 index and assert to Provider meta
		// type here because of the linting checks we pass in for the
		// ProviderRevision controller. Having those checks passed at controller
		// setup and doing this assertion in the reconciler is not a great
		// practice because it is not obvious that they are related or that
		// changing the checks could cause panic here. It would be good to
		// remove all conditional logic from this reconcile loop so that it is
		// less difficult to maintain in the future.
		pkgProvider := pkg.GetMeta()[0].(*pkgmeta.Provider)
		s, d := buildProviderDeployment(pkgProvider, pr, r.namespace)
		if err := r.client.Delete(ctx, d); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteProviderDeployment, "error", err)
			r.record.Event(pr, event.Warning(errDeleteProviderDeployment, err))
			pr.SetConditions(v1alpha1.Unhealthy())
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
		}
		if err := r.client.Delete(ctx, s); resource.IgnoreNotFound(err) != nil {
			log.Debug(errDeleteProviderSA, "error", err)
			r.record.Event(pr, event.Warning(errDeleteProviderSA, err))
			pr.SetConditions(v1alpha1.Unhealthy())
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
		}
	}

	// NOTE(hasheddan): the following logic enforces the present guarantees of
	// the package manager, which includes:
	// 1. We will not create or update any objects unless we can create or
	// update all objects
	// 2. We will not create or update any objects unless we can gain control of
	// all objects
	// 3. We will only modify owner references when the package revision is
	// inactive
	// 4. If objects are reconciled by a packaged controller, we will not remove
	// owner references until the controller has been stopped
	if err := r.establisher.Check(ctx, pkg.GetObjects(), pr, pr.GetDesiredState() == v1alpha1.PackageRevisionActive); err != nil {
		log.Debug(errEstablishControl, "error", err)
		r.record.Event(pr, event.Warning(errEstablishControl, err))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	if err := r.establisher.Establish(ctx, pr, pr.GetDesiredState() == v1alpha1.PackageRevisionActive); err != nil {
		log.Debug(errEstablishControl, "error", err)
		r.record.Event(pr, event.Warning(errEstablishControl, err))
		pr.SetConditions(v1alpha1.Unhealthy())
		return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
	}

	// If package is a provider package, all resources have been created, and
	// the revision is active, we should start the controller.
	if pr.GetObjectKind().GroupVersionKind() == v1alpha1.ProviderRevisionGroupVersionKind && desiredState == v1alpha1.PackageRevisionActive {
		// TODO(hasheddan): see comment on conditional logic above.
		pkgProvider := pkg.GetMeta()[0].(*pkgmeta.Provider)
		s, d := buildProviderDeployment(pkgProvider, pr, r.namespace)
		if err := r.client.Apply(ctx, s); err != nil {
			log.Debug(errCreateProviderSA, "error", err)
			r.record.Event(pr, event.Warning(errCreateProviderSA, err))
			pr.SetConditions(v1alpha1.Unhealthy())
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
		}
		if err := r.client.Apply(ctx, d); err != nil {
			log.Debug(errCreateProviderDeployment, "error", err)
			r.record.Event(pr, event.Warning(errCreateProviderDeployment, err))
			pr.SetConditions(v1alpha1.Unhealthy())
			return reconcile.Result{RequeueAfter: shortWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
		}
		pr.SetControllerReference(&runtimev1alpha1.Reference{Name: d.GetName()})
	}

	// Update object list in package revision status and set ready condition to
	// true.
	pr.SetObjects(r.establisher.GetResourceRefs())
	// TODO(hasheddan): set remaining status fields (dependencies, crossplane
	// version, permission requests)
	r.record.Event(pr, event.Normal(reasonInstall, "Successfully installed package revision"))
	pr.SetConditions(v1alpha1.Healthy())
	return reconcile.Result{RequeueAfter: longWait}, errors.Wrap(r.client.Status().Update(ctx, pr), errUpdateStatus)
}
