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

package usage

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

var _ admission.Handler = &ValidatingHandler{}

type mockRESTMapper struct {
	meta.RESTMapper
	restMappingFn func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return m.restMappingFn(gk, versions...)
}

func TestValidatingHandle(t *testing.T) {
	type params struct {
		mapper meta.RESTMapper
		opts   []ValidatingHandlerOption
	}

	type args struct {
		request admission.Request
	}

	type want struct {
		resp admission.Response
	}

	cases := map[string]struct {
		reason string
		params params
		args   args
		want   want
	}{
		"AllowDelete": {
			reason: "We should allow delete operations since they are not our concern.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"DenyBadObject": {
			reason: "We should return an error if we cannot decode the object.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object:    runtime.RawExtension{Raw: []byte("not-json")},
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Wrap(errors.New("invalid character 'o' in literal null (expecting 'u')"), errDecodeUsage)),
			},
		},
		"AllowClusterScopedUsage": {
			reason: "We should allow Usage objects that are cluster-scoped (no namespace).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "ClusterUsage",
								"metadata": {
									"name": "test"
								},
								"spec": {
									"of": {
										"apiVersion": "example.org/v1",
										"kind": "Cluster",
										"resourceRef": {"name": "my-cluster"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"AllowNamespacedUsageOfNamespacedResource": {
			reason: "We should allow a namespaced Usage that references a namespaced resource.",
			params: params{
				mapper: &mockRESTMapper{
					restMappingFn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Scope: meta.RESTScopeNamespace,
						}, nil
					},
				},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "test",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"apiVersion": "example.org/v1",
										"kind": "MyResource",
										"resourceRef": {"name": "my-resource"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"AllowUnresolvableGVK": {
			reason: "We should allow a Usage when the referenced GVK cannot be resolved, e.g. because the CRD is not installed yet.",
			params: params{
				mapper: &mockRESTMapper{
					restMappingFn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
						return nil, errBoom
					},
				},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "test",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"apiVersion": "unknown.io/v1",
										"kind": "Unknown",
										"resourceRef": {"name": "my-resource"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"DenyNamespacedUsageOfClusterScopedResourceWithRef": {
			reason: "We should deny a namespaced Usage that references a cluster-scoped resource when a resourceRef is specified.",
			params: params{
				mapper: &mockRESTMapper{
					restMappingFn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Scope: meta.RESTScopeRoot,
						}, nil
					},
				},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "protect-cluster",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"apiVersion": "example.org/v1",
										"kind": "ClusterThing",
										"resourceRef": {"name": "my-cluster-thing"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Denied(fmt.Sprintf(errFmtNamespacedUsageOfClusterN, "protect-cluster", "default", "example.org/v1", "ClusterThing", "my-cluster-thing")),
			},
		},
		"DenyNamespacedUsageOfClusterScopedResourceWithSelector": {
			reason: "We should deny a namespaced Usage that references a cluster-scoped resource when only a resourceSelector is specified.",
			params: params{
				mapper: &mockRESTMapper{
					restMappingFn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Scope: meta.RESTScopeRoot,
						}, nil
					},
				},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "protect-cluster",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"apiVersion": "example.org/v1",
										"kind": "ClusterThing",
										"resourceSelector": {"matchLabels": {"env": "prod"}}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Denied(fmt.Sprintf(errFmtNamespacedUsageOfCluster, "protect-cluster", "default", "example.org/v1", "ClusterThing")),
			},
		},
		"DenyNamespacedUsageOfClusterScopedResourceOnUpdate": {
			reason: "We should deny an update that changes spec.of to reference a cluster-scoped resource.",
			params: params{
				mapper: &mockRESTMapper{
					restMappingFn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
						return &meta.RESTMapping{
							Scope: meta.RESTScopeRoot,
						}, nil
					},
				},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "protect-cluster",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"apiVersion": "example.org/v1",
										"kind": "ClusterThing",
										"resourceRef": {"name": "my-cluster-thing"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Denied(fmt.Sprintf(errFmtNamespacedUsageOfClusterN, "protect-cluster", "default", "example.org/v1", "ClusterThing", "my-cluster-thing")),
			},
		},
		"AllowMissingAPIVersionOrKind": {
			reason: "We should allow a Usage when apiVersion or kind is missing in spec.of, letting the API server or reconciler handle the validation.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "protection.crossplane.io/v1beta1",
								"kind": "Usage",
								"metadata": {
									"name": "test",
									"namespace": "default"
								},
								"spec": {
									"of": {
										"resourceRef": {"name": "my-resource"}
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewValidatingHandler(tc.params.mapper, tc.params.opts...)

			got := h.Handle(context.Background(), tc.args.request)
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("%s\nHandle(...): -want response, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
