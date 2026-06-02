/*
Copyright 2025 The Crossplane Authors.

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

package check

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Pipeline step inputs covering the function-patch-and-transform detection
// branches in pipelinePublishesConnectionDetails.
const (
	ptInputWithConnectionDetails    = `{"apiVersion":"pt.fn.crossplane.io/v1beta1","kind":"Resources","resources":[{"name":"r","connectionDetails":[{"name":"x"}]}]}`
	ptInputWithoutConnectionDetails = `{"apiVersion":"pt.fn.crossplane.io/v1beta1","kind":"Resources","resources":[{"name":"r"}]}`
	nonPTInputWithConnectionDetails = `{"apiVersion":"other.fn.crossplane.io/v1beta1","kind":"Input","resources":[{"name":"r","connectionDetails":[{"name":"x"}]}]}`
)

// connDetailsXRD builds an XRD that declares connectionSecretKeys.
func connDetailsXRD(name string) apiextensionsv1.CompositeResourceDefinition {
	x := xrd("example.org", "XThing", "v1", "")
	x.SetName(name)
	x.Spec.ConnectionSecretKeys = []string{"endpoint"}
	return x
}

// connDetailsFixture is the data and per-stage errors the fake client serves in
// TestCompositeConnectionDetailsRun. instances is keyed by list kind (e.g.
// "XThingList"). Zero-value fields serve empty lists and no error, so each case
// sets only the slice or error branch it exercises; a single non-nil error lets
// a case exercise one List failure in isolation.
type connDetailsFixture struct {
	comps     []apiextensionsv1.Composition
	xrds      []apiextensionsv1.CompositeResourceDefinition
	instances map[string][]unstructured.Unstructured
	compsErr  error
	xrdsErr   error
	instErr   error
}

// connDetailsClient serves the Composition, XRD, and XR/Claim instance List
// calls the connection-details check makes, from the given fixture.
func connDetailsClient(f connDetailsFixture) client.Client {
	return &test.MockClient{
		MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
			switch l := list.(type) {
			case *apiextensionsv1.CompositionList:
				if f.compsErr != nil {
					return f.compsErr
				}
				l.Items = f.comps
			case *apiextensionsv1.CompositeResourceDefinitionList:
				if f.xrdsErr != nil {
					return f.xrdsErr
				}
				l.Items = f.xrds
			case *unstructured.UnstructuredList:
				if f.instErr != nil {
					return f.instErr
				}
				l.Items = f.instances[l.GetObjectKind().GroupVersionKind().Kind]
			}
			return nil
		},
	}
}

func TestCompositeConnectionDetailsRun(t *testing.T) {
	type want struct {
		findings []Finding
		err      error
	}
	cases := map[string]struct {
		reason string
		client client.Client
		want   want
	}{
		"Clean": {
			reason: "A Composition that publishes nothing and no XRDs produce no findings.",
			client: connDetailsClient(connDetailsFixture{comps: []apiextensionsv1.Composition{{ObjectMeta: metav1.ObjectMeta{Name: "clean"}}}}),
			want:   want{findings: nil},
		},
		"WriteConnectionSecretsToNamespace": {
			reason: "A Composition with spec.writeConnectionSecretsToNamespace is flagged.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						WriteConnectionSecretsToNamespace: ptr.To("secrets-ns"),
					},
				}},
			}),
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "comp"}, FieldPath: ".spec.writeConnectionSecretsToNamespace"}}},
		},
		"NativeConnectionDetails": {
			reason: "A native patch-and-transform resource that declares connectionDetails is flagged.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Resources: []apiextensionsv1.ComposedTemplate{{ConnectionDetails: make([]apiextensionsv1.ConnectionDetail, 1)}},
					},
				}},
			}),
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "comp"}, FieldPath: ".spec.resources[].connectionDetails"}}},
		},
		"PipelineConnectionDetails": {
			reason: "A function-patch-and-transform pipeline step that declares connectionDetails is flagged.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Pipeline: []apiextensionsv1.PipelineStep{{
							Step:  "pt",
							Input: &runtime.RawExtension{Raw: []byte(ptInputWithConnectionDetails)},
						}},
					},
				}},
			}),
			want: want{findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "comp"}, FieldPath: ".spec.pipeline[].input.resources[].connectionDetails"}}},
		},
		"PipelineWithoutConnectionDetails": {
			reason: "A function-patch-and-transform step with no connectionDetails is not flagged.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Pipeline: []apiextensionsv1.PipelineStep{{
							Step:  "pt",
							Input: &runtime.RawExtension{Raw: []byte(ptInputWithoutConnectionDetails)},
						}},
					},
				}},
			}),
			want: want{findings: nil},
		},
		"PipelineStepWithoutInput": {
			reason: "A pipeline step with no input (e.g. function-auto-ready) is skipped.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Pipeline: []apiextensionsv1.PipelineStep{{Step: "pt"}},
					},
				}},
			}),
			want: want{findings: nil},
		},
		"PipelineNonPatchAndTransform": {
			reason: "A pipeline step that isn't function-patch-and-transform is ignored even if it has connectionDetails.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Pipeline: []apiextensionsv1.PipelineStep{{
							Step:  "pt",
							Input: &runtime.RawExtension{Raw: []byte(nonPTInputWithConnectionDetails)},
						}},
					},
				}},
			}),
			want: want{findings: nil},
		},
		"PipelineUnparseableInput": {
			reason: "A pipeline step whose input isn't valid JSON makes the check incomplete.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						Pipeline: []apiextensionsv1.PipelineStep{{
							Step:  "pt",
							Input: &runtime.RawExtension{Raw: []byte("{not json")},
						}},
					},
				}},
			}),
			want: want{err: cmpopts.AnyError},
		},
		"XRDConnectionSecretKeys": {
			reason: "An XRD that declares connectionSecretKeys is flagged.",
			client: connDetailsClient(connDetailsFixture{xrds: []apiextensionsv1.CompositeResourceDefinition{connDetailsXRD("xthings.example.org")}}),
			want: want{
				findings: []Finding{
					{
						Resource: ResourceRef{
							Group: apiextensionsv1.Group,
							Kind:  apiextensionsv1.CompositeResourceDefinitionKind,
							Name:  "xthings.example.org",
						},
						FieldPath: ".spec.connectionSecretKeys",
					},
				},
			},
		},
		"XRInstanceWriteConnectionSecretToRef": {
			reason: "An XR instance with spec.writeConnectionSecretToRef is flagged.",
			client: connDetailsClient(connDetailsFixture{
				xrds: []apiextensionsv1.CompositeResourceDefinition{xrd("example.org", "XThing", "v1", "")},
				instances: map[string][]unstructured.Unstructured{
					"XThingList": {{Object: map[string]any{
						"apiVersion": "example.org/v1",
						"kind":       "XThing",
						"metadata":   map[string]any{"name": "my-xr"},
						"spec":       map[string]any{"writeConnectionSecretToRef": map[string]any{"name": "s"}},
					}}},
				},
			}),
			want: want{findings: []Finding{{Resource: ResourceRef{Group: "example.org", Kind: "XThing", Name: "my-xr"}, FieldPath: ".spec.writeConnectionSecretToRef"}}},
		},
		"ListCompositionsError": {
			reason: "A failure listing Compositions surfaces as an error with no findings.",
			client: connDetailsClient(connDetailsFixture{compsErr: errBoom}),
			want:   want{err: cmpopts.AnyError},
		},
		"ListXRDsError": {
			reason: "A failure listing XRDs returns the Composition findings gathered so far plus the error.",
			client: connDetailsClient(connDetailsFixture{
				comps: []apiextensionsv1.Composition{{
					ObjectMeta: metav1.ObjectMeta{Name: "comp"},
					Spec: apiextensionsv1.CompositionSpec{
						WriteConnectionSecretsToNamespace: ptr.To("secrets-ns"),
					},
				}},
				xrdsErr: errBoom,
			}),
			want: want{
				findings: []Finding{{Resource: ResourceRef{Group: apiextensionsv1.Group, Kind: apiextensionsv1.CompositionKind, Name: "comp"}, FieldPath: ".spec.writeConnectionSecretsToNamespace"}},
				err:      cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &CompositeConnectionDetails{Client: tc.client}
			got, err := c.Run(context.Background())
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nRun(): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.findings, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nRun(): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
