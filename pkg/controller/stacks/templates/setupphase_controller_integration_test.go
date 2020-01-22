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
	"testing"
	"time"

	"github.com/crossplaneio/crossplane-runtime/pkg/logging"
	"github.com/crossplaneio/crossplane-runtime/pkg/test/integration"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplaneio/crossplane/apis"
	"github.com/crossplaneio/crossplane/pkg/test"
)

func TestRenderControllerCreation(t *testing.T) {
	// TODO set test case in the test logger
	logger := test.TestLogger{T: t}
	logging.SetLogger(logger)
	ctrl.SetLogger(logger)

	// TODO make a unique namespace per test run
	cases := map[string]struct {
		reason string
		test   func(c client.Client, mgr manager.Manager) error
	}{
		"MissingCRD": {
			reason: "should result in a retry rather than a hard error",
			test: func(c client.Client, mgr manager.Manager) error {
				namespace := "test-template-stack-setupphase-controller"
				ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
				defer cancel()

				n := &corev1.Namespace{}
				n.SetName(namespace)
				if err := c.Create(ctx, n); err != nil {
					return err
				}

				// Slurp test data
				// Create test objects
				// Defer a deletion . . . can it just be in the teardown for the test?

				u := &unstructured.Unstructured{}
				if err := test.UnmarshalFromFile("../../../../cluster/test/template-engine/helm2/sample-clusterinstall.yaml", u); err != nil {
					return err
				}
				u.SetNamespace(namespace)
				if err := c.Create(ctx, u); err != nil {
					return err
				}

				u = &unstructured.Unstructured{}
				if err := test.UnmarshalFromFile("../../../../cluster/test/template-engine/helm2/stack.yaml", u); err != nil {
					return err
				}
				u.SetNamespace(namespace)
				if err := c.Create(ctx, u); err != nil {
					return err
				}
				stackConfigurationName := u.GetName()

				setupReconciler := NewSetupPhaseReconciler(c, logger, mgr)

				reconcileRequest := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Namespace: namespace,
						Name:      stackConfigurationName,
					},
				}
				res, err := setupReconciler.Reconcile(reconcileRequest)

				if !res.Requeue && res.RequeueAfter == 0 && err == nil {
					return fmt.Errorf("MissingCRD(): setup reconciler did not request requeue when CRD did not exist. got = %v, %v, want = Requeue: true OR RequeueAfter > 0 OR err != nil ", res, err)
				}

				// TODO - maybe a separate test case
				// Create CRD
				// Wait for CRD to exist
				// Check that controller reconciles normally

				// Test cases
				// - Missing CRD should error
				// - Present CRD should *not* error
				// - HasGVK should return false if GVK does not exist
				// - HasGVK should return true if GVK does exist

				return nil
			},
		},
	}

	var cfg *rest.Config
	cfg = nil

	i, err := integration.New(cfg,
		integration.WithCRDPaths("../../../../cluster/charts/crossplane/templates/crds"),
		integration.WithCleaners(
			integration.NewCRDCleaner(),
			integration.NewCRDDirCleaner()),
	)

	if err != nil {
		t.Fatal(err)
	}

	// Setup:
	// Set up stack manager controllers
	// Set up template stack controllers
	//
	// Test case: basic template stack sanity test with helm2
	// Install template stack
	// Create the CR
	// Wait for progress
	// Check for the config map
	// Clean up

	if err := apis.AddToScheme(i.GetScheme()); err != nil {
		t.Fatal(err)
	}

	if err := corev1.AddToScheme(i.GetScheme()); err != nil {
		t.Fatal(err)
	}

	if err := apiextensions.AddToScheme(i.GetScheme()); err != nil {
		t.Fatal(err)
	}

	i.Run()

	defer func() {
		if err := i.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.test(i.GetClient(), i)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
