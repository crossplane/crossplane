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

package templates

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

// SetupPhaseReconciler reconciles a stack configuration object
type SetupPhaseReconciler struct {
	Client            client.Client
	Log               logr.Logger
	Manager           manager.Manager
	renderControllers map[renderCoordinate]*RenderPhaseReconciler
}

type renderCoordinate struct {
	GVK       string
	EventName string
}

type behavior struct {
	cfg *v1alpha1.StackConfigurationBehavior
	gvk *schema.GroupVersionKind
}

const (
	setupTimeout = 60 * time.Second
)

// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=stacks.crossplane.io,resources=stackconfigurations/status,verbs=get;update;patch

// Reconcile watches for stack configurations and configures render phase controllers in response to a stack configuration
func (r *SetupPhaseReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), setupTimeout)
	defer cancel()

	i := &v1alpha1.StackConfiguration{}
	if err := r.Client.Get(ctx, req.NamespacedName, i); err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.setup(i)
}

func (r *SetupPhaseReconciler) setup(sc *v1alpha1.StackConfiguration) error {
	behaviors := r.getBehaviors(sc)
	configName, err := client.ObjectKeyFromObject(sc)

	if err != nil {
		r.Log.Info("setup exiting early because of error getting stack config object key", "err", err, "stackConfiguration", sc)
		return err
	}

	rcErrors := make([]string, 0)
	for _, b := range behaviors {
		gvk := b.gvk
		// TODO it'd be great to create the CRD for the user if it doesn't exist yet - /ht @muvaf for this idea

		// TODO we don't want to be hard-coding the event name here.
		event := "reconcile"

		rc := renderCoordinate{
			GVK:       apiVersionKindFromGvk(gvk),
			EventName: event,
		}

		// If we've already created a controller for this render coordinate,
		// we don't want to create another one.
		//
		// This synchronization mechanism assumes that a single thread is calling this method. If that
		// assumption no longer holds true, something fancier will be needed to synchronize the contents
		// of the map and the check to see whether a key's value is set.
		if _, ok := r.renderControllers[rc]; !ok {
			if rr, err := r.newRenderController(gvk, event, configName); err != nil {
				// TODO what do we want to do if some of the registrations succeed and some of them fail?
				r.Log.Error(err, "error creating new render controller", "gvk", gvk)
				rcErrors = append(rcErrors, err.Error())
			} else {
				r.renderControllers[rc] = rr
			}
		} else {
			r.Log.V(logging.Debug).Info("Not creating controller for render coordinate; one already exists", "renderCoordinate", rc)
		}
	}

	if len(rcErrors) > 0 {
		return fmt.Errorf(strings.Join(rcErrors, "; "))
	}

	return nil
}

func apiVersionKindFromGvk(gvk *schema.GroupVersionKind) string {
	return fmt.Sprintf("%s.%s/%s", gvk.Kind, gvk.Group, gvk.Version)
}

// This exists because getting the individual behaviors may be a bit tricker in the future.
// For example, the engine may be configured at multiple levels. Another example is that
// behaviors may be configured at multiple levels, if there are stack-level behaviors in
// addition to object-level behaviors.
func (r *SetupPhaseReconciler) getBehaviors(sc *v1alpha1.StackConfiguration) []behavior {
	scbs := sc.Spec.Behaviors.CRDs

	behaviors := make([]behavior, 0)

	for rawGvk, scb := range scbs {
		scb := scb
		// We are assuming strings look like "Kind.group.com/version"
		gvkSplit := strings.SplitN(string(rawGvk), ".", 2)
		gvk := schema.FromAPIVersionAndKind(gvkSplit[1], gvkSplit[0])

		behaviors = append(behaviors, behavior{
			gvk: &gvk,
			cfg: &scb,
		})
	}

	return behaviors
}

// newRenderController creates and configures a render controller for the given stack configuration.
func (r *SetupPhaseReconciler) newRenderController(gvk *schema.GroupVersionKind, event string, configName types.NamespacedName) (*RenderPhaseReconciler, error) {
	apiType := &unstructured.Unstructured{}
	apiType.SetGroupVersionKind(*gvk)

	reconciler := &RenderPhaseReconciler{
		Client:     r.Manager.GetClient(),
		Log:        ctrl.Log.WithName("controllers").WithName(fmt.Sprintf("%s.%s/%s", gvk.Kind, gvk.Group, gvk.Version)),
		GVK:        gvk,
		EventName:  event,
		ConfigName: configName,
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(r.Manager.GetConfig())

	if err != nil {
		return nil, err
	}

	if hasGvk, err := HasGVK(discoveryClient, gvk); err != nil {
		return nil, err
	} else if !hasGvk {
		return nil, fmt.Errorf("Could not find requested GVK %s", gvk)
	}

	r.Log.V(logging.Debug).Info("Adding new controller to manager", "type", gvk)
	err = ctrl.NewControllerManagedBy(r.Manager).
		For(apiType).
		Owns(&batchv1.Job{}).
		Complete(reconciler)

	if err != nil {
		r.Log.Info("unable to create controller", "gvk", gvk, "err", err)
		return nil, err
	}

	return reconciler, nil
}

// SetupWithManager is a convenience method to register the reconciler with a controller manager.
func (r *SetupPhaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.StackConfiguration{}).
		Complete(r)
}

// NewSetupPhaseReconciler creates a setup phase reconciler and initializes all of its fields.
// It mostly exists to initialize its internal render controller map.
func NewSetupPhaseReconciler(c client.Client, l logr.Logger, m manager.Manager) *SetupPhaseReconciler {
	return &SetupPhaseReconciler{
		Client:            c,
		Log:               l,
		Manager:           m,
		renderControllers: map[renderCoordinate]*RenderPhaseReconciler{},
	}
}

// HasGVK is (mostly) copied from here: https://github.com/kubernetes/kubectl/blob/7ee592cfa6faa7bf554fbcf30bdfd0c87c02461d/pkg/generate/versioned/generator.go#L221
// It mostly exists to check for potential CRD lookup errors before they happen, such as when a setup controller is creating a render phase controller.
func HasGVK(client discovery.DiscoveryInterface, gvk *schema.GroupVersionKind) (bool, error) {
	resources, err := client.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if kerrors.IsNotFound(err) {
		// entire group is missing
		return false, nil
	}
	if err != nil {
		// other errors error
		return false, fmt.Errorf("failed to discover supported resources: %v", err)
	}
	for _, serverResource := range resources.APIResources {
		if serverResource.Kind == gvk.Kind {
			return true, nil
		}
	}
	return false, nil
}
