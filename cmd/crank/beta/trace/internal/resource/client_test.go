// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
)

type xrcOpt func(c *claim.Unstructured)

func withXRCRef(ref *v1.ObjectReference) xrcOpt {
	return func(c *claim.Unstructured) {
		c.SetResourceReference(ref)
	}
}

func withXRCSecretRef(ref *xpv1.LocalSecretReference) xrcOpt {
	return func(c *claim.Unstructured) {
		c.SetWriteConnectionSecretToReference(ref)
	}
}

func buildXRC(namespace string, name string, opts ...xrcOpt) *unstructured.Unstructured {
	c := claim.New()
	c.SetName(name)
	c.SetNamespace(namespace)
	for _, f := range opts {
		f(c)
	}
	return &c.Unstructured
}

type xrOpt func(c *composite.Unstructured)

func withXRRefs(refs ...v1.ObjectReference) xrOpt {
	return func(c *composite.Unstructured) {
		c.SetResourceReferences(refs)
	}
}

func withXRSecretRef(ref *xpv1.SecretReference) xrOpt {
	return func(c *composite.Unstructured) {
		c.SetWriteConnectionSecretToReference(ref)
	}
}

func buildXR(name string, opts ...xrOpt) *unstructured.Unstructured {
	c := composite.New()
	c.SetName(name)
	for _, f := range opts {
		f(c)
	}
	return &c.Unstructured
}

func TestGetResourceChildrenRefs(t *testing.T) {
	type args struct {
		resource   *Resource
		witSecrets bool
	}
	type want struct {
		refs []v1.ObjectReference
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"XRCWithChildrenXR": {
			reason: "Should return the XR child for an XRC.",
			args: args{
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRWithChildren": {
			reason: "Should return the list of children refs for an XR.",
			args: args{
				resource: &Resource{
					Unstructured: *buildXR("root-xr", withXRRefs(v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					}, v1.ObjectReference{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
						Namespace:  "ns-1",
					},
					)),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
						Namespace:  "ns-1",
					},
				},
			},
		},
		"XRCWithChildrenXRandConnectionSecretEnabled": {
			reason: "Should return the XR child, but no writeConnectionSecret ref for an XRC.",
			args: args{
				witSecrets: true,
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCSecretRef(&xpv1.LocalSecretReference{
						Name: "secret-1",
					}), withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "ns-1",
						Name:       "secret-1",
					},
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRCWithChildrenXRandConnectionSecretDisabled": {
			reason: "Should return the XR child, but no writeConnectionSecret, ref for an XRC.",
			args: args{
				witSecrets: false,
				resource: &Resource{
					Unstructured: *buildXRC("ns-1", "xrc", withXRCSecretRef(&xpv1.LocalSecretReference{
						Name: "secret-1",
					}), withXRCRef(&v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					})),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "example.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
				},
			},
		},
		"XRWithChildrenAndSecret": {
			reason: "Should return a list of children refs for an XR.",
			args: args{
				witSecrets: true,
				resource: &Resource{
					Unstructured: *buildXR("root-xr", withXRSecretRef(&xpv1.SecretReference{
						Name:      "secret-1",
						Namespace: "ns-1",
					}), withXRRefs(v1.ObjectReference{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "MR",
							Name:       "mr-2",
						},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "XR",
							Name:       "xr-1",
						},
						v1.ObjectReference{
							APIVersion: "example2.com/v1",
							Kind:       "XRC",
							Name:       "xrc-1",
						},
					)),
				},
			},
			want: want{
				refs: []v1.ObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "ns-1",
						Name:       "secret-1",
					},
					{
						APIVersion: "example.com/v1",
						Kind:       "MR",
						Name:       "mr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "MR",
						Name:       "mr-2",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XR",
						Name:       "xr-1",
					},
					{
						APIVersion: "example2.com/v1",
						Kind:       "XRC",
						Name:       "xrc-1",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getResourceChildrenRefs(tc.args.resource, tc.args.witSecrets)
			if diff := cmp.Diff(tc.want.refs, got, cmpopts.SortSlices(func(r1, r2 v1.ObjectReference) bool {
				return strings.Compare(r1.String(), r2.String()) < 0
			})); diff != "" {
				t.Errorf("\n%s\ngetResourceChildrenRefs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
