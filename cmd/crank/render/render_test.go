/*
Copyright 2023 The Crossplane Authors.

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

package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	ucomposite "github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
)

func TestGetSecret(t *testing.T) {
	secrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "namespace1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "namespace2",
			},
		},
	}

	tests := map[string]struct {
		name      string
		namespace string
		secrets   []corev1.Secret
		wantErr   bool
	}{
		"SecretFound": {
			name:      "secret1",
			namespace: "namespace1",
			secrets:   secrets,
			wantErr:   false,
		},
		"SecretNotFound": {
			name:      "secret3",
			namespace: "namespace3",
			secrets:   secrets,
			wantErr:   true,
		},
		"SecretWrongNamespace": {
			name:      "secret1",
			namespace: "namespace2",
			secrets:   secrets,
			wantErr:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := GetSecret(tc.name, tc.namespace, tc.secrets)
			if (err != nil) != tc.wantErr {
				t.Errorf("GetSecret() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSetComposedResourceMetadata(t *testing.T) {
	type args struct {
		cd   *composed.Unstructured
		xr   *ucomposite.Unstructured
		name string
	}
	type want struct {
		generateName   string
		compositeLabel string
		claimName      string
		claimNamespace string
	}

	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"RootXRUsesOwnName": {
			reason: "A root XR without composite label should use its own name",
			args: args{
				cd: composed.New(),
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetName("root-xr")
					return xr
				}(),
				name: "resource-a",
			},
			want: want{
				generateName:   "root-xr-",
				compositeLabel: "root-xr",
			},
		},
		"NestedXRPropagatesRootLabel": {
			reason: "A nested XR with composite label should propagate the root's name",
			args: args{
				cd: composed.New(),
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetName("root-xr-child")
					xr.SetLabels(map[string]string{
						AnnotationKeyCompositeName: "root-xr",
					})
					return xr
				}(),
				name: "resource-a",
			},
			want: want{
				generateName:   "root-xr-",
				compositeLabel: "root-xr",
			},
		},
		"NestedXRPropagatesClaimLabels": {
			reason: "A nested XR with claim labels should propagate them to composed resources",
			args: args{
				cd: composed.New(),
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetName("root-xr-child")
					xr.SetLabels(map[string]string{
						AnnotationKeyCompositeName:  "root-xr",
						AnnotationKeyClaimName:      "my-claim",
						AnnotationKeyClaimNamespace: "claim-ns",
					})
					return xr
				}(),
				name: "resource-a",
			},
			want: want{
				generateName:   "root-xr-",
				compositeLabel: "root-xr",
				claimName:      "my-claim",
				claimNamespace: "claim-ns",
			},
		},
		"RootXRWithClaimReference": {
			reason: "A root XR with ClaimReference but no claim labels should use ClaimReference for claim labels",
			args: args{
				cd: composed.New(),
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetName("root-xr")
					xr.SetClaimReference(&reference.Claim{
						Name:      "my-claim",
						Namespace: "claim-ns",
					})
					return xr
				}(),
				name: "resource-a",
			},
			want: want{
				generateName:   "root-xr-",
				compositeLabel: "root-xr",
				claimName:      "my-claim",
				claimNamespace: "claim-ns",
			},
		},
		"XRWithClaimLabelsButNoCompositeLabel": {
			reason: "An XR with claim labels but no composite label should fall back to XR name and still propagate claim labels",
			args: args{
				cd: composed.New(),
				xr: func() *ucomposite.Unstructured {
					xr := ucomposite.New(ucomposite.WithSchema(ucomposite.SchemaLegacy))
					xr.SetName("root-xr")
					xr.SetLabels(map[string]string{
						AnnotationKeyClaimName:      "my-claim",
						AnnotationKeyClaimNamespace: "claim-ns",
					})
					return xr
				}(),
				name: "resource-a",
			},
			want: want{
				generateName:   "root-xr-",
				compositeLabel: "root-xr",
				claimName:      "my-claim",
				claimNamespace: "claim-ns",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := SetComposedResourceMetadata(tc.args.cd, tc.args.xr, tc.args.name)
			if err != nil {
				t.Fatalf("SetComposedResourceMetadata() error = %v", err)
			}

			if diff := cmp.Diff(tc.want.generateName, tc.args.cd.GetGenerateName()); diff != "" {
				t.Errorf("%s\nSetComposedResourceMetadata() generateName: -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.compositeLabel, tc.args.cd.GetLabels()[AnnotationKeyCompositeName]); diff != "" {
				t.Errorf("%s\nSetComposedResourceMetadata() compositeLabel: -want, +got:\n%s", tc.reason, diff)
			}
			if tc.want.claimName != "" {
				if diff := cmp.Diff(tc.want.claimName, tc.args.cd.GetLabels()[AnnotationKeyClaimName]); diff != "" {
					t.Errorf("%s\nSetComposedResourceMetadata() claimName: -want, +got:\n%s", tc.reason, diff)
				}
			}
			if tc.want.claimNamespace != "" {
				if diff := cmp.Diff(tc.want.claimNamespace, tc.args.cd.GetLabels()[AnnotationKeyClaimNamespace]); diff != "" {
					t.Errorf("%s\nSetComposedResourceMetadata() claimNamespace: -want, +got:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func MustStructJSON(j string) *structpb.Struct {
	s := &structpb.Struct{}
	if err := protojson.Unmarshal([]byte(j), s); err != nil {
		panic(err)
	}

	return s
}
