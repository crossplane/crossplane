/*
Copyright 2026 The Crossplane Authors.

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
	"encoding/json"
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

	"github.com/crossplane/crossplane/apis/v2/protection/v1beta1"
)

var _ admission.Handler = &ScopeHandler{}

type mockRESTMapper struct {
	meta.RESTMapper

	fn func(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error)
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return m.fn(gk, versions...)
}

func TestScopeHandle(t *testing.T) {
	type params struct {
		mapper meta.RESTMapper
		opts   []ScopeHandlerOption
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
		"UnexpectedDelete": {
			reason: "We should return an error if the request is a delete (not a create or update).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedScopeOp, admissionv1.Delete)),
			},
		},
		"UnexpectedConnect": {
			reason: "We should return an error if the request is a connect (not a create or update).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Connect,
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedScopeOp, admissionv1.Connect)),
			},
		},
		"UnexpectedOperation": {
			reason: "We should return an error if the request is unknown (not a create or update).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Operation("unknown"),
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedScopeOp, admissionv1.Operation("unknown"))),
			},
		},
		"UnmarshalError": {
			reason: "We should return an error if we cannot unmarshal the Usage.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object:    runtime.RawExtension{Raw: []byte("not-json")},
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, json.Unmarshal([]byte("not-json"), &v1beta1.Usage{})),
			},
		},
		"AllowedUnparseableAPIVersion": {
			reason: "We should allow a Usage with an unparseable APIVersion. The Usage controller returns an error for it.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "/invalid/", "kind": "Thing", "resourceRef": {"name": "thing"}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"AllowedUnknownKind": {
			reason: "We should allow a Usage of a kind whose scope we can't determine, e.g. because its CRD isn't installed yet.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return nil, errBoom
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "Unknown", "resourceRef": {"name": "thing"}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"AllowedNamespacedResource": {
			reason: "We should allow a namespaced Usage of a namespaced resource.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return &meta.RESTMapping{Scope: meta.RESTScopeNamespace}, nil
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "Thing", "resourceRef": {"name": "thing"}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"DeniedClusterScopedResource": {
			reason: "We should deny a namespaced Usage of a cluster-scoped resource.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return &meta.RESTMapping{Scope: meta.RESTScopeRoot}, nil
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Denied(fmt.Sprintf(errFmtClusterScopedOf, schema.FromAPIVersionAndKind("example.org/v1", "ClusterThing"))),
			},
		},
		"DeniedUpdatedToClusterScopedResourceBySelector": {
			reason: "We should deny an update that changes a namespaced Usage to use a cluster-scoped resource, even if it uses a resource selector rather than a resource ref.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return &meta.RESTMapping{Scope: meta.RESTScopeRoot}, nil
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceSelector": {"matchLabels": {"env": "prod"}}}}
						}`)},
						OldObject: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "Thing", "resourceSelector": {"matchLabels": {"env": "prod"}}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Denied(fmt.Sprintf(errFmtClusterScopedOf, schema.FromAPIVersionAndKind("example.org/v1", "ClusterThing"))),
			},
		},
		"UnmarshalOldObjectError": {
			reason: "We should return an error if we cannot unmarshal the old Usage on an update.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
						OldObject: runtime.RawExtension{Raw: []byte("not-json")},
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, json.Unmarshal([]byte("not-json"), &v1beta1.Usage{})),
			},
		},
		"AllowedUpdateOfDeletedUsage": {
			reason: "We should allow updates to a Usage that's being deleted, so that its finalizers can be removed. Denying them would leave the Usage stuck terminating - the Usage controller removes its finalizer with an update.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return &meta.RESTMapping{Scope: meta.RESTScopeRoot}, nil
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default", "deletionTimestamp": "2026-07-24T00:00:00Z"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
						OldObject: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default", "deletionTimestamp": "2026-07-24T00:00:00Z", "finalizers": ["usage.apiextensions.crossplane.io"]},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
					},
				},
			},
			want: want{
				resp: admission.Allowed(""),
			},
		},
		"AllowedUpdateWithUnchangedOf": {
			reason: "We should allow an update that doesn't change what resource a Usage uses, even if that resource is cluster-scoped. A Usage that uses a cluster-scoped resource might predate this webhook - it must remain mutable and deletable. The Usage controller surfaces the error.",
			params: params{
				mapper: &mockRESTMapper{fn: func(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
					return &meta.RESTMapping{Scope: meta.RESTScopeRoot}, nil
				}},
			},
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default", "annotations": {"example.org/note": "updated"}},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
						OldObject: runtime.RawExtension{Raw: []byte(`{
							"apiVersion": "protection.crossplane.io/v1beta1",
							"kind": "Usage",
							"metadata": {"name": "protect", "namespace": "default"},
							"spec": {"of": {"apiVersion": "example.org/v1", "kind": "ClusterThing", "resourceRef": {"name": "cluster-resource"}}}
						}`)},
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
			h := NewScopeHandler(tc.params.mapper, tc.params.opts...)

			got := h.Handle(context.Background(), tc.args.request)
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("%s\nh.Handle(...): -want response, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
