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

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/crossplaneio/crossplane-runtime/pkg/test/integration"
	"github.com/crossplaneio/crossplane/apis"
	"github.com/crossplaneio/crossplane/apis/stacks/v1alpha1"
)

const (
	testStackDefinition = `
---
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackDefinition

metadata:
  name: 'minimal-gcp'

spec:
  behaviors:
    engine:
      type: kustomize

      kustomize:
        kustomization:
          namePrefix: mystuff
          commonLabels:
            group: minimal-gcp-stuff

        overlays:
        - apiVersion: gcp.crossplane.io/v1alpha3
          kind: Provider
          name: gcp-provider
          bindings:
            - from: "spec.credentialsSecretRef"
              to: "spec.credentialsSecretRef"
            - from: "spec.projectID"
              to: "spec.projectID"
        - apiVersion: v1
          kind: ConfigMap
          name: gcp-config
          bindings:
            - from: "spec.region"
              to: "data.region"

    source:
      image: 'crossplane/minimal-gcp:0.0.1'
      path: resources/minimal-gcp

    crd:
      apiVersion: gcp.resourcepacks.crossplane.io/v1alpha1
      kind: MinimalGCP

  title: "Minimal GCP Environment"
  readme: ""
  overview: "This stack provisions a private network and creates resource classes that has minimal node settings and refer to that private network."
  overviewShort: "Start using GCP with Crossplane without needing to create your own resource classes!"
  version: "1.0"
  maintainers:
  - name: "Muvaffak Onus"
    email: "monus@upbound.io"
  owners:
  - name: "Muvaffak Onus"
    email: "monus@upbound.io"
  company: "Upbound"
  category: "Environment Stack"
  keywords:
   - "easy"
   - "resource class"
   - "private network"
   - "cheap"
   - "minimal"
  website: "httos://upbound.io"
  source: "https://github.com/muvaf/minimal-gcp"
  permissionScope: Cluster

  dependsOn:
    - crd: '*.cache.gcp.crossplane.io/v1beta1'
    - crd: '*.compute.gcp.crossplane.io/v1alpha3'
    - crd: '*.database.gcp.crossplane.io/v1beta1'
    - crd: '*.storage.gcp.crossplane.io/v1alpha3'
    - crd: '*.servicenetworking.gcp.crossplane.io/v1alpha3'
    - crd: '*.gcp.crossplane.io/v1alpha3'
  license: Apache-2.0
`
)

func TestStackDefinitionController(t *testing.T) {
	t.Skip("Skipping so that this test will not run in CI, until we have better support for it. For more context, see https://github.com/crossplaneio/crossplane/issues/1033")

	cases := map[string]struct {
		reason string
		test   func(c client.Client, mgr manager.Manager) error
	}{
		"CreateStack": {
			reason: "should create a stack which doesn't exist yet",
			test: func(c client.Client, mgr manager.Manager) error {
				namespace := "test-stackdefinition-controller"
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Set up test namespace
				n := &corev1.Namespace{}
				n.SetName(namespace)
				if err := c.Create(ctx, n); err != nil {
					t.Log("CreateStack(): returning error result when trying to create namespace.", "err", err)
					return err
				}

				// Add our test data to the "cluster"
				u := &unstructured.Unstructured{}
				if err := yaml.Unmarshal([]byte(testStackDefinition), u); err != nil {
					t.Log("CreateStack(): returning error result when unmarshaling stack definition.", "err", err)
					return err
				}

				u.SetNamespace(namespace)
				if err := c.Create(ctx, u); err != nil {
					t.Log("CreateStack(): returning error result when creating stack definition.", "err", err)
					return err
				}

				name := u.GetName()
				nn := types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				}

				// A paranoid test may check for the precondition that the expected result Stack
				// does not exist at this time. Let's be paranoid.
				s := &v1alpha1.Stack{}
				if err := c.Get(ctx, nn, s); err == nil {
					t.Error("CreateStack(): returning error result when fetching created stack\ngot = err: nil\nwant = err: not found", "err", err, "stack", s)
				} else if !kerrors.IsNotFound(err) {
					t.Errorf("CreateStack(): returning error result when fetching created stack\ngot = err: %v\nwant = err: not found", err)
				}

				// Now we reconcile
				reconciler := StackDefinitionReconciler{
					Client: c,
					Log:    ctrl.Log,
				}
				reconcileRequest := ctrl.Request{
					NamespacedName: nn,
				}
				res, err := reconciler.Reconcile(reconcileRequest)

				// We want our reconcile to succeed
				if res.Requeue || res.RequeueAfter != 0 || err != nil {
					return fmt.Errorf("CreateStack(): reconcile did not complete successfully.\ngot = result: %v, err: %v\nwant = Requeue: false, RequeueAfter: 0, err: nil", res, err)
				}

				// Now that our reconcile has succeeded, we expect the stack to exist
				ss := &v1alpha1.Stack{}
				if err := c.Get(ctx, nn, ss); err != nil {
					t.Log("CreateStack(): returning error result when fetching created stack", "err", err, "stack", s)
					return err
				}

				// NOTE the test does not check the fields of the stack. Ideally it would, for correctness guarantees

				return nil
			},
		},
	}

	var cfg *rest.Config

	i, err := integration.New(cfg,
		integration.WithCRDPaths("../../../../cluster/charts/crossplane/templates/crds"),
		integration.WithCleaners(
			integration.NewCRDCleaner(),
			integration.NewCRDDirCleaner()),
	)

	if err != nil {
		t.Fatal(err)
	}

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
