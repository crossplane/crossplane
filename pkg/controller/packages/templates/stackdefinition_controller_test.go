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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/packages/v1alpha1"
)

const (
	testStackDefinitionYAML = `---
apiVersion: Packages.crossplane.io/v1alpha1
kind: StackDefinition
metadata:
  name: 'my-gcp'
  namespace: 'crossplane-system'
spec:
  behaviors:
    engine:
      type: kustomize
      kustomize:
        kustomization:
          namePrefix: mystuff
          commonLabels:
            group: my-gcp-stuff
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
      image: 'crossplane/stack-gcp-sample:0.3.0'
      path: kustomize
    crd:
      apiVersion: gcp.stacks.crossplane.io/v1alpha1
      kind: GCPSample
  title: "GCP Sample Environment"
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
  website: "https://upbound.io"
  source: "https://github.com/crossplane/stack-gcp-sample"
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
	testPackageYAML = `---
metadata:
  creationTimestamp: null
  name: my-gcp
  namespace: crossplane-system
  ownerReferences:
  - apiVersion: packages.crossplane.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: StackDefinition
    name: my-gcp
    uid: ""
spec:
  category: Environment Stack
  company: Upbound
  controller: {}
  customresourcedefinitions: []
  dependsOn:
  - crd: '*.cache.gcp.crossplane.io/v1beta1'
  - crd: '*.compute.gcp.crossplane.io/v1alpha3'
  - crd: '*.database.gcp.crossplane.io/v1beta1'
  - crd: '*.storage.gcp.crossplane.io/v1alpha3'
  - crd: '*.servicenetworking.gcp.crossplane.io/v1alpha3'
  - crd: '*.gcp.crossplane.io/v1alpha3'
  keywords:
  - easy
  - resource class
  - private network
  - cheap
  - minimal
  license: Apache-2.0
  maintainers:
  - email: monus@upbound.io
    name: Muvaffak Onus
  overview: This stack provisions a private network and creates resource classes that
    has minimal node settings and refer to that private network.
  overviewShort: Start using GCP with Crossplane without needing to create your own
    resource classes!
  owners:
  - email: monus@upbound.io
    name: Muvaffak Onus
  permissionScope: Cluster
  permissions:
    rules:
    - apiGroups:
      - packages.crossplane.io
      resourceNames:
      - my-gcp
      resources:
      - stackdefinitions
      - stackdefinitions/status
      verbs:
      - get
      - list
      - watch
  source: https://github.com/crossplane/stack-gcp-sample
  title: GCP Sample Environment
  version: "1.0"
  website: https://upbound.io
status:
  conditionedStatus: {}

`
)

func TestStackDefinitionController(t *testing.T) {
	// Add our test data to the "cluster"
	testDef := &v1alpha1.StackDefinition{}
	parse(testStackDefinitionYAML, testDef)
	testPackage := &v1alpha1.Package{}
	parse(testPackageYAML, testPackage)
	type args struct {
		def  *v1alpha1.StackDefinition
		kube client.Client
	}
	type want struct {
		stack *v1alpha1.Package
		err   error
		rec   ctrl.Result
	}

	cases := map[string]struct {
		args
		want
		reason string
	}{
		"CreatePackage": {
			reason: "should create a stack which doesn't exist yet",
			args: args{
				def: testDef,
				kube: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
						switch o := obj.(type) {
						case *v1alpha1.StackDefinition:
							testDef.DeepCopyInto(o)
							return nil
						case *v1alpha1.Package:
							return kerrors.NewNotFound(schema.GroupResource{}, testPackage.Name)
						default:
							return errors.New("boom")
						}
					},
					MockCreate: func(_ context.Context, obj runtime.Object, opts ...client.CreateOption) error {
						switch s := obj.(type) {
						case *v1alpha1.Package:
							if diff := cmp.Diff(testPackage, s); diff != "" {
								t.Errorf("Reconcile: -want, +got:\n%s", diff)
							}
							return nil
						default:
							return errors.New("boom")
						}
					},
				},
			},
			want: want{
				stack: testPackage,
				rec:   ctrl.Result{RequeueAfter: longWait},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &StackDefinitionReconciler{Client: tc.kube, Log: logging.NewNopLogger()}
			res, err := r.Reconcile(ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: tc.args.def.Namespace,
				Name:      tc.args.def.Name,
			}})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Reconcile: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.rec, res); diff != "" {
				t.Errorf("Reconcile: -want, +got:\n%s", diff)
			}
		})
	}
}

func parse(data string, obj runtime.Object) {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(data), 4096)
	if err := dec.Decode(obj); err != nil {
		panic("cannot parse the test YAML")
	}
}
