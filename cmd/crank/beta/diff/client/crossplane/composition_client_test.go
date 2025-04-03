package crossplane

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

const CrossplaneAPIExtGroup = "apiextensions.crossplane.io"
const CrossplaneAPIExtGroupV1 = "apiextensions.crossplane.io/v1"

func TestDefaultCompositionClient_FindMatchingComposition(t *testing.T) {

	type fields struct {
		compositions map[string]*apiextensionsv1.Composition
	}

	type args struct {
		ctx context.Context
		res *un.Unstructured
	}

	type want struct {
		composition *apiextensionsv1.Composition
		err         error
	}

	// Create test compositions
	matchingComp := tu.NewComposition("matching-comp").
		WithCompositeTypeRef("example.org/v1", "XR1").
		Build()

	nonMatchingComp := tu.NewComposition("non-matching-comp").
		WithCompositeTypeRef("example.org/v1", "OtherXR").
		Build()

	referencedComp := tu.NewComposition("referenced-comp").
		WithCompositeTypeRef("example.org/v1", "XR1").
		Build()

	incompatibleComp := tu.NewComposition("incompatible-comp").
		WithCompositeTypeRef("example.org/v1", "OtherXR").
		Build()

	labeledComp := func() *apiextensionsv1.Composition {
		comp := tu.NewComposition("labeled-comp").
			WithCompositeTypeRef("example.org/v1", "XR1").
			Build()
		comp.SetLabels(map[string]string{
			"environment": "production",
			"tier":        "standard",
		})
		return comp
	}()

	aComp := func() *apiextensionsv1.Composition {
		comp := tu.NewComposition("a-comp").
			WithCompositeTypeRef("example.org/v1", "XR1").
			Build()
		comp.SetLabels(map[string]string{
			"environment": "production",
		})
		return comp
	}()

	bComp := func() *apiextensionsv1.Composition {
		comp := tu.NewComposition("b-comp").
			WithCompositeTypeRef("example.org/v1", "XR1").
			Build()
		comp.SetLabels(map[string]string{
			"environment": "production",
		})
		return comp
	}()

	versionMismatchComp := tu.NewComposition("version-mismatch-comp").
		WithCompositeTypeRef("example.org/v2", "XR1").
		Build()

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		mockDef      tu.MockDefinitionClient
		fields       fields
		args         args
		want         want
	}{
		"NoMatchingComposition": {
			reason: "Should return error when no matching composition exists",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"non-matching-comp": nonMatchingComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"MatchingComposition": {
			reason: "Should return the matching composition",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"matching-comp":     matchingComp,
					"non-matching-comp": nonMatchingComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				composition: matchingComp,
			},
		},
		"DirectCompositionReference": {
			reason: "Should return the composition referenced by spec.compositionRef.name",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"default-comp":    matchingComp,
					"referenced-comp": referencedComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedField(xr.Object, "referenced-comp", "spec", "compositionRef", "name")
					return xr
				}(),
			},
			want: want{
				composition: referencedComp,
			},
		},
		"DirectCompositionReferenceIncompatible": {
			reason: "Should return error when directly referenced composition is incompatible",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"matching-comp":     matchingComp,
					"incompatible-comp": incompatibleComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedField(xr.Object, "incompatible-comp", "spec", "compositionRef", "name")
					return xr
				}(),
			},
			want: want{
				err: errors.Errorf("composition incompatible-comp is not compatible with example.org/v1, Kind=XR1"),
			},
		},
		"ReferencedCompositionNotFound": {
			reason: "Should return error when referenced composition doesn't exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"existing-comp": matchingComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedField(xr.Object, "non-existent-comp", "spec", "compositionRef", "name")
					return xr
				}(),
			},
			want: want{
				err: errors.Errorf("composition non-existent-comp referenced in example.org/v1, Kind=XR1/my-xr not found"),
			},
		},
		"CompositionSelectorMatch": {
			reason: "Should return composition matching the selector labels",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"labeled-comp":      labeledComp,
					"non-matching-comp": nonMatchingComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedStringMap(xr.Object, map[string]string{
						"environment": "production",
					}, "spec", "compositionSelector", "matchLabels")
					return xr
				}(),
			},
			want: want{
				composition: labeledComp,
			},
		},
		"CompositionSelectorNoMatch": {
			reason: "Should return error when no composition matches the selector",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"labeled-comp": func() *apiextensionsv1.Composition {
						comp := tu.NewComposition("labeled-comp").
							WithCompositeTypeRef("example.org/v1", "XR1").
							Build()
						comp.SetLabels(map[string]string{
							"environment": "staging",
						})
						return comp
					}(),
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedStringMap(xr.Object, map[string]string{
						"environment": "production",
					}, "spec", "compositionSelector", "matchLabels")
					return xr
				}(),
			},
			want: want{
				err: errors.Errorf("no compatible composition found matching labels map[environment:production] for example.org/v1, Kind=XR1/my-xr"),
			},
		},
		"MultipleCompositionMatches": {
			reason: "Should return an error when multiple compositions match the selector",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"a-comp": aComp,
					"b-comp": bComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedStringMap(xr.Object, map[string]string{
						"environment": "production",
					}, "spec", "compositionSelector", "matchLabels")
					return xr
				}(),
			},
			want: want{
				err: errors.New("ambiguous composition selection: multiple compositions match"),
			},
		},
		"EmptyCompositionCache_DefaultLookup": {
			reason: "Should return error when composition cache is empty (default lookup)",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"EmptyCompositionCache_DirectReference": {
			reason: "Should return error when composition cache is empty (direct reference)",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedField(xr.Object, "referenced-comp", "spec", "compositionRef", "name")
					return xr
				}(),
			},
			want: want{
				err: errors.Errorf("composition referenced-comp referenced in example.org/v1, Kind=XR1/my-xr not found"),
			},
		},
		"EmptyCompositionCache_Selector": {
			reason: "Should return error when composition cache is empty (selector)",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithEmptyXRDsFetch().
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{},
			},
			args: args{
				ctx: context.Background(),
				res: func() *un.Unstructured {
					xr := tu.NewResource("example.org/v1", "XR1", "my-xr").Build()
					_ = un.SetNestedStringMap(xr.Object, map[string]string{
						"environment": "production",
					}, "spec", "compositionSelector", "matchLabels")
					return xr
				}(),
			},
			want: want{
				err: errors.Errorf("no compatible composition found matching labels map[environment:production] for example.org/v1, Kind=XR1/my-xr"),
			},
		},
		"AmbiguousDefaultSelection": {
			reason: "Should return error when multiple compositions match by type but no selection criteria provided",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
					return []*un.Unstructured{}, nil
				}).
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"comp1": matchingComp,
					"comp2": referencedComp, // Both match same XR type
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.New("ambiguous composition selection: multiple compositions exist for example.org/v1, Kind=XR1"),
			},
		},
		"DifferentVersions": {
			reason: "Should not match compositions with different versions",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
					return []*un.Unstructured{}, nil
				}).
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"version-mismatch-comp": versionMismatchComp,
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "XR1", "my-xr").Build(),
			},
			want: want{
				err: errors.Errorf("no composition found for %s", "example.org/v1, Kind=XR1"),
			},
		},
		"ClaimResource": {
			reason: "Should find composition for a claim type by determining XR type from XRD",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					// Set up to return XRDs when requested
					if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "CompositeResourceDefinition" {
						return []*un.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": CrossplaneAPIExtGroupV1,
									"kind":       "CompositeResourceDefinition",
									"metadata": map[string]interface{}{
										"name": "xexampleresources.example.org",
									},
									"spec": map[string]interface{}{
										"group": "example.org",
										"names": map[string]interface{}{
											"kind": "XExampleResource",
										},
										"claimNames": map[string]interface{}{
											"kind": "ExampleResourceClaim",
										},
										"versions": []interface{}{
											map[string]interface{}{
												"name":          "v1",
												"served":        true,
												"referenceable": false,
											},
											map[string]interface{}{
												"name":          "v2",
												"served":        true,
												"referenceable": true, // This is the version compositions should reference
											},
											map[string]interface{}{
												"name":          "v3alpha1",
												"served":        true,
												"referenceable": false,
											},
										},
									},
								},
							},
						}, nil
					}
					return []*un.Unstructured{}, nil
				}).
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
					return []*un.Unstructured{
						{
							Object: map[string]interface{}{
								"apiVersion": CrossplaneAPIExtGroupV1,
								"kind":       "CompositeResourceDefinition",
								"metadata": map[string]interface{}{
									"name": "xexampleresources.example.org",
								},
								"spec": map[string]interface{}{
									"group": "example.org",
									"names": map[string]interface{}{
										"kind": "XExampleResource",
									},
									"claimNames": map[string]interface{}{
										"kind": "ExampleResourceClaim",
									},
									"versions": []interface{}{
										map[string]interface{}{
											"name":          "v1",
											"served":        true,
											"referenceable": false,
										},
										map[string]interface{}{
											"name":          "v2",
											"served":        true,
											"referenceable": true, // This is the version compositions should reference
										},
										map[string]interface{}{
											"name":          "v3alpha1",
											"served":        true,
											"referenceable": false,
										},
									},
								},
							},
						},
					}, nil
				}).
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"matching-comp": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "matching-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v2", // Match the referenceable version v2
								Kind:       "XExampleResource",
							},
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "ExampleResourceClaim", "test-claim").
					WithSpecField("compositionRef", map[string]interface{}{
						"name": "matching-comp",
					}).
					Build(),
			},
			want: want{
				composition: &apiextensionsv1.Composition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "matching-comp",
					},
					Spec: apiextensionsv1.CompositionSpec{
						CompositeTypeRef: apiextensionsv1.TypeReference{
							APIVersion: "example.org/v2",
							Kind:       "XExampleResource",
						},
					},
				},
				err: nil,
			},
		},
		"ClaimResourceWithNoReferenceableVersion": {
			reason: "Should return error when XRD has no referenceable version",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					// Return XRDs when requested - but this one has NO referenceable version
					if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "CompositeResourceDefinition" {
						return []*un.Unstructured{
							{
								Object: map[string]interface{}{
									"apiVersion": CrossplaneAPIExtGroupV1,
									"kind":       "CompositeResourceDefinition",
									"metadata": map[string]interface{}{
										"name": "xexampleresources.example.org",
									},
									"spec": map[string]interface{}{
										"group": "example.org",
										"names": map[string]interface{}{
											"kind": "XExampleResource",
										},
										"claimNames": map[string]interface{}{
											"kind": "ExampleResourceClaim",
										},
										"versions": []interface{}{
											map[string]interface{}{
												"name":          "v1",
												"served":        true,
												"referenceable": false, // No referenceable version
											},
											map[string]interface{}{
												"name":          "v2",
												"served":        true,
												"referenceable": false, // No referenceable version
											},
										},
									},
								},
							},
						}, nil
					}
					return []*un.Unstructured{}, nil
				}).
				Build(),
			mockDef: *tu.NewMockDefinitionClient().
				WithSuccessfulInitialize().
				WithGetXRDs(func(context.Context) ([]*un.Unstructured, error) {
					return []*un.Unstructured{
						{
							Object: map[string]interface{}{
								"apiVersion": CrossplaneAPIExtGroupV1,
								"kind":       "CompositeResourceDefinition",
								"metadata": map[string]interface{}{
									"name": "xexampleresources.example.org",
								},
								"spec": map[string]interface{}{
									"group": "example.org",
									"names": map[string]interface{}{
										"kind": "XExampleResource",
									},
									"claimNames": map[string]interface{}{
										"kind": "ExampleResourceClaim",
									},
									"versions": []interface{}{
										map[string]interface{}{
											"name":          "v1",
											"served":        true,
											"referenceable": false, // No referenceable version
										},
										map[string]interface{}{
											"name":          "v2",
											"served":        true,
											"referenceable": false, // No referenceable version
										},
									},
								},
							},
						},
					}, nil
				}).
				Build(),
			fields: fields{
				compositions: map[string]*apiextensionsv1.Composition{
					"matching-comp": {
						ObjectMeta: metav1.ObjectMeta{
							Name: "matching-comp",
						},
						Spec: apiextensionsv1.CompositionSpec{
							CompositeTypeRef: apiextensionsv1.TypeReference{
								APIVersion: "example.org/v1",
								Kind:       "XExampleResource",
							},
						},
					},
				},
			},
			args: args{
				ctx: context.Background(),
				res: tu.NewResource("example.org/v1", "ExampleResourceClaim", "test-claim").
					WithSpecField("compositionRef", map[string]interface{}{
						"name": "matching-comp",
					}).
					Build(),
			},
			want: want{
				composition: nil,
				err:         errors.New("no referenceable version found in XRD"), // Should fail with this error
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			// Create the CompositionClient
			c := &DefaultCompositionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				compositions:   tt.fields.compositions,
			}

			// Test the FindMatchingComposition method
			got, err := c.FindMatchingComposition(tt.args.ctx, tt.args.res)

			if tt.want.err != nil {
				if err == nil {
					t.Errorf("\n%s\nFindMatchingComposition(...): expected error but got none", tt.reason)
					return
				}

				if !strings.Contains(err.Error(), tt.want.err.Error()) {
					t.Errorf("\n%s\nFindMatchingComposition(...): expected error containing %q, got %q",
						tt.reason, tt.want.err.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nFindMatchingComposition(...): unexpected error: %v", tt.reason, err)
				return
			}

			if tt.want.composition != nil {
				if diff := cmp.Diff(tt.want.composition.Name, got.Name); diff != "" {
					t.Errorf("\n%s\nFindMatchingComposition(...): -want composition name, +got composition name:\n%s", tt.reason, diff)
				}

				if diff := cmp.Diff(tt.want.composition.Spec.CompositeTypeRef, got.Spec.CompositeTypeRef); diff != "" {
					t.Errorf("\n%s\nFindMatchingComposition(...): -want composition type ref, +got composition type ref:\n%s", tt.reason, diff)
				}
			}
		})
	}
}

func TestDefaultCompositionClient_GetComposition(t *testing.T) {
	ctx := context.Background()

	// Create a test composition
	testComp := tu.NewComposition("test-comp").
		WithCompositeTypeRef("example.org/v1", "XR1").
		Build()

	// Mock resource client
	mockResource := tu.NewMockResourceClient().
		WithSuccessfulInitialize().
		WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
			if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "Composition" && name == "test-comp" {
				u := &un.Unstructured{}
				u.SetGroupVersionKind(gvk)
				u.SetName(name)
				obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testComp)
				if err != nil {
					return nil, err
				}
				u.SetUnstructuredContent(obj)
				return u, nil
			}
			return nil, errors.New("composition not found")
		}).
		Build()

	tests := map[string]struct {
		reason      string
		name        string
		cache       map[string]*apiextensionsv1.Composition
		expectComp  *apiextensionsv1.Composition
		expectError bool
	}{
		"CachedComposition": {
			reason: "Should return composition from cache when available",
			name:   "cached-comp",
			cache: map[string]*apiextensionsv1.Composition{
				"cached-comp": testComp,
			},
			expectComp:  testComp,
			expectError: false,
		},
		"FetchFromCluster": {
			reason:      "Should fetch composition from cluster when not in cache",
			name:        "test-comp",
			cache:       map[string]*apiextensionsv1.Composition{},
			expectComp:  testComp,
			expectError: false,
		},
		"NotFound": {
			reason:      "Should return error when composition doesn't exist",
			name:        "nonexistent-comp",
			cache:       map[string]*apiextensionsv1.Composition{},
			expectComp:  nil,
			expectError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultCompositionClient{
				resourceClient: mockResource,
				logger:         tu.TestLogger(t, false),
				compositions:   tt.cache,
			}

			comp, err := c.GetComposition(ctx, tt.name)

			if tt.expectError {
				if err == nil {
					t.Errorf("\n%s\nGetComposition(...): expected error but got none", tt.reason)
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetComposition(...): unexpected error: %v", tt.reason, err)
				return
			}

			if diff := cmp.Diff(tt.expectComp.GetName(), comp.GetName()); diff != "" {
				t.Errorf("\n%s\nGetComposition(...): -want name, +got name:\n%s", tt.reason, diff)
			}

			if diff := cmp.Diff(tt.expectComp.Spec.CompositeTypeRef, comp.Spec.CompositeTypeRef); diff != "" {
				t.Errorf("\n%s\nGetComposition(...): -want type ref, +got type ref:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestDefaultCompositionClient_ListCompositions(t *testing.T) {
	ctx := context.Background()

	// Create test compositions
	comp1 := tu.NewComposition("comp1").
		WithCompositeTypeRef("example.org/v1", "XR1").
		Build()
	comp2 := tu.NewComposition("comp2").
		WithCompositeTypeRef("example.org/v1", "XR2").
		Build()

	// Convert compositions to unstructured
	u1 := &un.Unstructured{}
	obj1, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(comp1)
	u1.SetUnstructuredContent(obj1)
	u1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   CrossplaneAPIExtGroup,
		Version: "v1",
		Kind:    "Composition",
	})

	u2 := &un.Unstructured{}
	obj2, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(comp2)
	u2.SetUnstructuredContent(obj2)
	u2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   CrossplaneAPIExtGroup,
		Version: "v1",
		Kind:    "Composition",
	})

	tests := map[string]struct {
		reason        string
		mockResource  *tu.MockResourceClient
		expectComps   []*apiextensionsv1.Composition
		expectError   bool
		errorContains string
	}{
		"SuccessfulList": {
			reason: "Should return compositions when list succeeds",
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "Composition" {
						return []*un.Unstructured{u1, u2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			expectComps: []*apiextensionsv1.Composition{comp1, comp2},
			expectError: false,
		},
		"ListError": {
			reason: "Should return error when list fails",
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			expectComps:   nil,
			expectError:   true,
			errorContains: "cannot list compositions",
		},
		"ConversionError": {
			reason: "Should return error when conversion fails",
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					// Create an invalid unstructured that will definitely fail conversion
					invalid := &un.Unstructured{}
					invalid.SetGroupVersionKind(gvk)
					invalid.SetName("invalid")

					// Include invalid data that won't convert to a Composition
					invalid.Object["spec"] = "not-a-map-but-a-string" // This will cause conversion to fail

					return []*un.Unstructured{invalid}, nil
				}).
				Build(),
			expectComps:   nil,
			expectError:   true,
			errorContains: "cannot convert unstructured to Composition",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultCompositionClient{
				resourceClient: tt.mockResource,
				logger:         tu.TestLogger(t, false),
				compositions:   make(map[string]*apiextensionsv1.Composition),
			}

			comps, err := c.ListCompositions(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("\n%s\nListCompositions(...): expected error but got none", tt.reason)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("\n%s\nListCompositions(...): expected error containing %q, got %q",
						tt.reason, tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nListCompositions(...): unexpected error: %v", tt.reason, err)
				return
			}

			if len(comps) != len(tt.expectComps) {
				t.Errorf("\n%s\nListCompositions(...): expected %d compositions, got %d",
					tt.reason, len(tt.expectComps), len(comps))
				return
			}

			// Check that we got the expected compositions
			for i, expected := range tt.expectComps {
				found := false
				for _, actual := range comps {
					if actual.GetName() == expected.GetName() {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("\n%s\nListCompositions(...): composition %s not found in result",
						tt.reason, tt.expectComps[i].GetName())
				}
			}
		})
	}
}

func TestDefaultCompositionClient_Initialize(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		reason       string
		mockResource *tu.MockResourceClient
		expectError  bool
	}{
		"SuccessfulInitialization": {
			reason: "Should successfully initialize the client",
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			expectError: false,
		},
		"ResourceClientInitFailed": {
			reason: "Should return error when resource client initialization fails",
			mockResource: tu.NewMockResourceClient().
				WithInitialize(func(_ context.Context) error {
					return errors.New("init error")
				}).
				Build(),
			expectError: true,
		},
		"ListCompositionsFailed": {
			reason: "Should return error when listing compositions fails",
			mockResource: tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			expectError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultCompositionClient{
				resourceClient: tt.mockResource,
				logger:         tu.TestLogger(t, false),
				compositions:   make(map[string]*apiextensionsv1.Composition),
			}

			err := c.Initialize(ctx)

			if tt.expectError && err == nil {
				t.Errorf("\n%s\nInitialize(): expected error but got none", tt.reason)
			} else if !tt.expectError && err != nil {
				t.Errorf("\n%s\nInitialize(): unexpected error: %v", tt.reason, err)
			}
		})
	}
}
