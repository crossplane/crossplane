package composition

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"

	"github.com/crossplane/crossplane/pkg/validation/internal/schema"
)

func getDefaultMetadataSchema() *apiextensions.JSONSchemaProps {
	return defaultMetadataOnly(&apiextensions.JSONSchemaProps{})
}

func getDefaultSchema() *apiextensions.JSONSchemaProps {
	return defaultMetadataSchema(&apiextensions.JSONSchemaProps{})
}

func TestDefaultMetadataSchema(t *testing.T) {
	type args struct {
		in *apiextensions.JSONSchemaProps
	}
	type want struct {
		out *apiextensions.JSONSchemaProps
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Nil": {
			reason: "Nil should output the default metadata schema",
			args:   args{in: nil},
			want: want{
				out: getDefaultSchema(),
			},
		},
		"Empty": {
			reason: "Empty should output the default metadata schema",
			args:   args{in: &apiextensions.JSONSchemaProps{}},
			want: want{
				out: getDefaultSchema(),
			},
		},
		"Metadata": {
			reason: "Metadata should output the default metadata schema",
			args: args{in: &apiextensions.JSONSchemaProps{
				Type: string(schema.KnownJSONTypeObject),
				Properties: map[string]apiextensions.JSONSchemaProps{
					"metadata": *getDefaultMetadataSchema(),
				},
			}},
			want: want{
				out: &apiextensions.JSONSchemaProps{
					Type: string(schema.KnownJSONTypeObject),
					Properties: map[string]apiextensions.JSONSchemaProps{
						"metadata": *getDefaultMetadataSchema(),
					},
				},
			},
		},
		"SpecPreserved": {
			reason: "Other properties should be preserved",
			args: args{
				in: &apiextensions.JSONSchemaProps{
					Type: string(schema.KnownJSONTypeObject),
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Type: string(schema.KnownJSONTypeObject),
							AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
								Allows: true,
							},
						},
					},
				},
			},
			want: want{
				out: &apiextensions.JSONSchemaProps{
					Type: string(schema.KnownJSONTypeObject),
					Properties: map[string]apiextensions.JSONSchemaProps{
						"metadata": *getDefaultMetadataSchema(),
						"spec": {
							Type: string(schema.KnownJSONTypeObject),
							AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
								Allows: true,
							},
						},
					},
				},
			},
		},
		"MetadataNotOverwrite": {
			reason: "Other properties should not be overwritten in metadata if specified in the default",
			args: args{in: &apiextensions.JSONSchemaProps{
				Type: string(schema.KnownJSONTypeObject),
				Properties: map[string]apiextensions.JSONSchemaProps{
					"metadata": {
						Type: string(schema.KnownJSONTypeObject),
						Properties: map[string]apiextensions.JSONSchemaProps{
							"name": {
								Type: string(schema.KnownJSONTypeBoolean),
							},
						},
					},
				},
			}},
			want: want{
				out: func() *apiextensions.JSONSchemaProps {
					s := getDefaultSchema()
					metadata := s.Properties["metadata"]
					metadata.Properties["name"] = apiextensions.JSONSchemaProps{
						Type: string(schema.KnownJSONTypeBoolean),
					}
					s.Properties["metadata"] = metadata
					return s
				}(),
			},
		},
		"MetadataPreserved": {
			reason: "Other properties should be preserved in if not specified in the default",
			args: args{
				in: &apiextensions.JSONSchemaProps{
					Type: string(schema.KnownJSONTypeObject),
					Properties: map[string]apiextensions.JSONSchemaProps{
						"metadata": {
							Type: string(schema.KnownJSONTypeObject),
							Properties: map[string]apiextensions.JSONSchemaProps{
								"annotations": {
									Type: string(schema.KnownJSONTypeObject),
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: string(schema.KnownJSONTypeString)},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				out: func() *apiextensions.JSONSchemaProps {
					s := getDefaultSchema()
					metadata := s.Properties["metadata"]
					annotations := metadata.Properties["annotations"]
					if annotations.Properties == nil {
						annotations.Properties = map[string]apiextensions.JSONSchemaProps{}
					}
					annotations.Properties["foo"] = apiextensions.JSONSchemaProps{
						Type: string(schema.KnownJSONTypeString),
					}
					metadata.Properties["annotations"] = annotations
					s.Properties["metadata"] = metadata
					return s
				}(),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out := defaultMetadataSchema(tc.args.in)
			if diff := cmp.Diff(tc.want.out, out); diff != "" {
				t.Errorf("\n%s\ndefaultMetadataSchema(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
