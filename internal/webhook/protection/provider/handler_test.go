/*
Copyright 2024 The Crossplane Authors.

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

package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionv1 "k8s.io/api/admission/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1alpha1 "github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
)

var _ admission.Handler = &Handler{}

var errBoom = errors.New("boom")

func TestHandle(t *testing.T) {
	type args struct {
		client  client.Client
		request admission.Request
	}

	type want struct {
		resp admission.Response
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"UnexpectedCreate": {
			reason: "We should return an error if the request is a create (not a delete).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Create,
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, admissionv1.Create)),
			},
		},
		"UnexpectedUpdate": {
			reason: "We should return an error if the request is an update (not a delete).",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Errorf(errFmtUnexpectedOp, admissionv1.Update)),
			},
		},
		"DeleteWithoutOldObj": {
			reason: "We should return an error if delete request does not have the old object.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusBadRequest, errors.Wrap(errors.New("unexpected end of JSON input"), errUnmarshalProvider)),
			},
		},
		"DeleteAllowedNoRevision": {
			reason: "We should allow deletion if provider has no current revision.",
			args: args{
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": ""
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
		"DeleteAllowedRevisionNotFound": {
			reason: "We should allow deletion if the provider revision is not found.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						if _, ok := obj.(*v1.ProviderRevision); ok {
							return kerrors.NewNotFound(schema.GroupResource{Group: "pkg.crossplane.io", Resource: "providerrevisions"}, key.Name)
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": "test-provider-abc123"
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
		"DeleteAllowedNoCRDs": {
			reason: "We should allow deletion if the provider revision has no CRDs.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if pr, ok := obj.(*v1.ProviderRevision); ok {
							*pr = v1.ProviderRevision{
								Status: v1.ProviderRevisionStatus{
									PackageRevisionStatus: v1.PackageRevisionStatus{
										ObjectRefs: []xpv1.TypedReference{
											{
												APIVersion: "v1",
												Kind:       "ServiceAccount",
												Name:       "test-sa",
											},
										},
									},
								},
							}
							return nil
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": "test-provider-abc123"
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
		"DeleteAllowedNoCRsExist": {
			reason: "We should allow deletion if no custom resources exist for the provider's MRDs.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.ProviderRevision:
							*o = v1.ProviderRevision{
								Status: v1.ProviderRevisionStatus{
									PackageRevisionStatus: v1.PackageRevisionStatus{
										ObjectRefs: []xpv1.TypedReference{
											{
												APIVersion: "apiextensions.crossplane.io/v1alpha1",
												Kind:       v1alpha1.ManagedResourceDefinitionKind,
												Name:       "tests.example.com",
											},
										},
									},
								},
							}
							return nil
						case *v1alpha1.ManagedResourceDefinition:
							*o = v1alpha1.ManagedResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: key.Name,
								},
								Spec: v1alpha1.ManagedResourceDefinitionSpec{
									State: v1alpha1.ManagedResourceDefinitionActive,
								},
							}
							return nil
						case *extv1.CustomResourceDefinition:
							*o = extv1.CustomResourceDefinition{
								Spec: extv1.CustomResourceDefinitionSpec{
									Group: "example.com",
									Names: extv1.CustomResourceDefinitionNames{
										Kind:     "Test",
										ListKind: "TestList",
									},
									Scope: extv1.ClusterScoped,
									Versions: []extv1.CustomResourceDefinitionVersion{
										{
											Name:    "v1alpha1",
											Storage: true,
										},
									},
								},
							}
							return nil
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							ul.Items = []unstructured.Unstructured{} // No instances
							return nil
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": "test-provider-abc123"
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
		"DeleteBlockedCRsExist": {
			reason: "We should block deletion if custom resources exist for the provider's MRDs.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.ProviderRevision:
							*o = v1.ProviderRevision{
								Status: v1.ProviderRevisionStatus{
									PackageRevisionStatus: v1.PackageRevisionStatus{
										ObjectRefs: []xpv1.TypedReference{
											{
												APIVersion: "apiextensions.crossplane.io/v1alpha1",
												Kind:       v1alpha1.ManagedResourceDefinitionKind,
												Name:       "buckets.s3.aws.upbound.io",
											},
										},
									},
								},
							}
							return nil
						case *v1alpha1.ManagedResourceDefinition:
							*o = v1alpha1.ManagedResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: key.Name,
								},
								Spec: v1alpha1.ManagedResourceDefinitionSpec{
									State: v1alpha1.ManagedResourceDefinitionActive,
								},
							}
							return nil
						case *extv1.CustomResourceDefinition:
							*o = extv1.CustomResourceDefinition{
								Spec: extv1.CustomResourceDefinitionSpec{
									Group: "s3.aws.upbound.io",
									Names: extv1.CustomResourceDefinitionNames{
										Kind:     "Bucket",
										ListKind: "BucketList",
									},
									Scope: extv1.ClusterScoped,
									Versions: []extv1.CustomResourceDefinitionVersion{
										{
											Name:    "v1beta1",
											Storage: true,
										},
									},
								},
							}
							return nil
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							// Return 1 instance (only need 1 to block)
							ul.Items = []unstructured.Unstructured{
								{},
							}
							return nil
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "provider-aws-s3"
								},
								"status": {
									"currentRevision": "provider-aws-s3-abc123"
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:   int32(http.StatusConflict),
							Reason: metav1.StatusReason(errCRsExist),
						},
					},
				},
			},
		},
		"DeleteBlockedMultipleCRDsWithCRs": {
			reason: "We should block deletion if multiple MRDs have custom resources (returns early on first match).",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.ProviderRevision:
							*o = v1.ProviderRevision{
								Status: v1.ProviderRevisionStatus{
									PackageRevisionStatus: v1.PackageRevisionStatus{
										ObjectRefs: []xpv1.TypedReference{
											{
												APIVersion: "apiextensions.crossplane.io/v1alpha1",
												Kind:       v1alpha1.ManagedResourceDefinitionKind,
												Name:       "buckets.s3.aws.upbound.io",
											},
											{
												APIVersion: "apiextensions.crossplane.io/v1alpha1",
												Kind:       v1alpha1.ManagedResourceDefinitionKind,
												Name:       "roles.iam.aws.upbound.io",
											},
										},
									},
								},
							}
							return nil
						case *v1alpha1.ManagedResourceDefinition:
							*o = v1alpha1.ManagedResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: key.Name,
								},
								Spec: v1alpha1.ManagedResourceDefinitionSpec{
									State: v1alpha1.ManagedResourceDefinitionActive,
								},
							}
							return nil
						case *extv1.CustomResourceDefinition:
							if key.Name == "buckets.s3.aws.upbound.io" {
								*o = extv1.CustomResourceDefinition{
									Spec: extv1.CustomResourceDefinitionSpec{
										Group: "s3.aws.upbound.io",
										Names: extv1.CustomResourceDefinitionNames{
											Kind:     "Bucket",
											ListKind: "BucketList",
										},
										Scope: extv1.ClusterScoped,
										Versions: []extv1.CustomResourceDefinitionVersion{
											{
												Name:    "v1beta1",
												Storage: true,
											},
										},
									},
								}
							} else {
								*o = extv1.CustomResourceDefinition{
									Spec: extv1.CustomResourceDefinitionSpec{
										Group: "iam.aws.upbound.io",
										Names: extv1.CustomResourceDefinitionNames{
											Kind:     "Role",
											ListKind: "RoleList",
										},
										Scope: extv1.ClusterScoped,
										Versions: []extv1.CustomResourceDefinitionVersion{
											{
												Name:    "v1beta1",
												Storage: true,
											},
										},
									},
								}
							}
							return nil
						}
						return nil
					},
					MockList: func(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
						if ul, ok := list.(*unstructured.UnstructuredList); ok {
							// First CRD has instances, so it will return early
							ul.Items = []unstructured.Unstructured{
								{},
							}
							return nil
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "provider-aws"
								},
								"status": {
									"currentRevision": "provider-aws-abc123"
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Response{
					AdmissionResponse: admissionv1.AdmissionResponse{
						Allowed: false,
						Result: &metav1.Status{
							Code:   int32(http.StatusConflict),
							Reason: metav1.StatusReason(errCRsExist),
						},
					},
				},
			},
		},
		"DeleteRejectedCannotGetRevision": {
			reason: "We should reject a delete request if we cannot get the provider revision.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						if _, ok := obj.(*v1.ProviderRevision); ok {
							return errBoom
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": "test-provider-abc123"
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusInternalServerError, errors.Wrapf(errBoom, errFmtGetProviderRevision, "test-provider-abc123")),
			},
		},
		"DeleteRejectedCannotGetCRD": {
			reason: "We should reject a delete request if we cannot get a CRD for an MRD.",
			args: args{
				client: &test.MockClient{
					MockGet: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
						switch o := obj.(type) {
						case *v1.ProviderRevision:
							*o = v1.ProviderRevision{
								Status: v1.ProviderRevisionStatus{
									PackageRevisionStatus: v1.PackageRevisionStatus{
										ObjectRefs: []xpv1.TypedReference{
											{
												APIVersion: "apiextensions.crossplane.io/v1alpha1",
												Kind:       v1alpha1.ManagedResourceDefinitionKind,
												Name:       "tests.example.com",
											},
										},
									},
								},
							}
							return nil
						case *v1alpha1.ManagedResourceDefinition:
							*o = v1alpha1.ManagedResourceDefinition{
								ObjectMeta: metav1.ObjectMeta{
									Name: key.Name,
								},
								Spec: v1alpha1.ManagedResourceDefinitionSpec{
									State: v1alpha1.ManagedResourceDefinitionActive,
								},
							}
							return nil
						case *extv1.CustomResourceDefinition:
							return errBoom
						}
						return nil
					},
				},
				request: admission.Request{
					AdmissionRequest: admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						OldObject: runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "pkg.crossplane.io/v1",
								"kind": "Provider",
								"metadata": {
									"name": "test-provider"
								},
								"status": {
									"currentRevision": "test-provider-abc123"
								}
							}`),
						},
					},
				},
			},
			want: want{
				resp: admission.Errored(http.StatusInternalServerError, errors.Wrapf(errors.Wrapf(errBoom, errFmtGetCRD, "tests.example.com"), errFmtListCRs, "tests.example.com")),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewHandler(tc.args.client)

			got := h.Handle(context.Background(), tc.args.request)
			if diff := cmp.Diff(tc.want.resp, got); diff != "" {
				t.Errorf("%s\nHandle(...): -want response, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
