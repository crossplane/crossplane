package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestDefaultValues(t *testing.T) {
	type args struct {
		xr         map[string]any
		crd        extv1.CustomResourceDefinition
		apiVersion string
	}

	cases := map[string]struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		"SetDefaultValues": {
			name: "Should set default values according to schema",
			args: args{
				apiVersion: "example.com/v1",
				xr: map[string]any{
					"spec": map[string]any{},
				},
				crd: extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"cooldown": {
														Type:    "integer",
														Default: &extv1.JSON{Raw: []byte(`5`)},
													},
													"enabled": {
														Type:    "boolean",
														Default: &extv1.JSON{Raw: []byte(`true`)},
													},
													"status": {
														Type: "string",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"spec": map[string]any{
					"cooldown": int64(5),
					"enabled":  true,
				},
			},
			wantErr: false,
		},
		"DontOverwriteExistingValues": {
			name: "Should not overwrite existing values with defaults",
			args: args{
				apiVersion: "example.com/v1",
				xr: map[string]any{
					"spec": map[string]any{
						"cooldown": int64(10),
						"enabled":  false,
					},
				},
				crd: extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"cooldown": {
														Type:    "integer",
														Default: &extv1.JSON{Raw: []byte(`5`)},
													},
													"enabled": {
														Type:    "boolean",
														Default: &extv1.JSON{Raw: []byte(`true`)},
													},
													"status": {
														Type: "string",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"spec": map[string]any{
					"cooldown": int64(10),
					"enabled":  false,
				},
			},
			wantErr: false,
		},
		"MultipleVersions": {
			name: "Should only apply defaults from requested version",
			args: args{
				apiVersion: "example.com/v2",
				xr: map[string]any{
					"spec": map[string]any{},
				},
				crd: extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"cooldown": {
														Type:    "integer",
														Default: &extv1.JSON{Raw: []byte(`5`)},
													},
													"enabled": {
														Type:    "boolean",
														Default: &extv1.JSON{Raw: []byte(`true`)},
													},
												},
											},
										},
									},
								},
							},
							{
								Name: "v2",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"cooldown": {
														Type:    "integer",
														Default: &extv1.JSON{Raw: []byte(`15`)},
													},
													"enabled": {
														Type:    "boolean",
														Default: &extv1.JSON{Raw: []byte(`false`)},
													},
													"newField": {
														Type:    "string",
														Default: &extv1.JSON{Raw: []byte(`"default"`)},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"spec": map[string]any{
					"cooldown": int64(15),
					"enabled":  false,
					"newField": "default",
				},
			},
			wantErr: false,
		},
		"IncorrectAPIVersion": {
			name: "Should return error for incorrect API version",
			args: args{
				apiVersion: "wrong-group/v1",
				xr: map[string]any{
					"spec": map[string]any{},
				},
				crd: extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"cooldown": {
														Type:    "integer",
														Default: &extv1.JSON{Raw: []byte(`5`)},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]any{
				"spec": map[string]any{},
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := DefaultValues(tc.args.xr, tc.args.apiVersion, tc.args.crd)
			if (err != nil) != tc.wantErr {
				t.Errorf("DefaultValues() error = %v, wantErr %v", err, tc.wantErr)
			}

			if diff := cmp.Diff(tc.want, tc.args.xr); diff != "" {
				t.Errorf("DefaultValues() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
