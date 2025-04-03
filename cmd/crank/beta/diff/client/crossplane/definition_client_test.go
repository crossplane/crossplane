package crossplane

import (
	"context"
	"strings"
	"testing"

	"strings"

	"github.com/google/go-cmp/cmp"
	un "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	tu "github.com/crossplane/crossplane/cmd/crank/beta/diff/testutils"
)

var _ DefinitionClient = (*tu.MockDefinitionClient)(nil)

func TestDefaultDefinitionClient_GetXRDs(t *testing.T) {
	ctx := context.Background()

	// Create test XRDs
	xrd1 := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd1").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XR1",
			"plural":   "xr1s",
			"singular": "xr1",
		}).
		Build()

	xrd2 := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd2").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XR2",
			"plural":   "xr2s",
			"singular": "xr2",
		}).
		Build()

	type fields struct {
		xrds       []*un.Unstructured
		xrdsLoaded bool
	}

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		fields       fields
		want         []*un.Unstructured
		wantErr      bool
		errSubstring string
	}{
		"NoXRDsFound": {
			reason: "Should return empty slice when no XRDs exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "CompositeResourceDefinition" {
						return []*un.Unstructured{}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			fields: fields{
				xrds:       nil,
				xrdsLoaded: false,
			},
			want:    []*un.Unstructured{},
			wantErr: false,
		},
		"XRDsExist": {
			reason: "Should return all XRDs when they exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == CrossplaneAPIExtGroup && gvk.Kind == "CompositeResourceDefinition" {
						return []*un.Unstructured{xrd1, xrd2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			fields: fields{
				xrds:       nil,
				xrdsLoaded: false,
			},
			want:    []*un.Unstructured{xrd1, xrd2},
			wantErr: false,
		},
		"ListError": {
			reason: "Should propagate errors from the Kubernetes API",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			fields: fields{
				xrds:       nil,
				xrdsLoaded: false,
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot list XRDs",
		},
		"UsesCache": {
			reason: "Should use cached XRDs when already loaded",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				// This should never be called since cache is used
				WithListResources(func(context.Context, schema.GroupVersionKind, string) ([]*un.Unstructured, error) {
					t.Errorf("ListResources should not be called when cache is available")
					return nil, errors.New("should not be called")
				}).
				Build(),
			fields: fields{
				xrds:       []*un.Unstructured{xrd1, xrd2},
				xrdsLoaded: true,
			},
			want:    []*un.Unstructured{xrd1, xrd2},
			wantErr: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultDefinitionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				xrds:           tt.fields.xrds,
				xrdsLoaded:     tt.fields.xrdsLoaded,
			}

			got, err := c.GetXRDs(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nGetXRDs(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nGetXRDs(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetXRDs(): unexpected error: %v", tt.reason, err)
				return
			}

			// Verify cache state
			if !c.xrdsLoaded {
				t.Errorf("\n%s\nGetXRDs(): cache not marked as loaded after call", tt.reason)
			}

			// Compare XRD count
			if diff := cmp.Diff(len(tt.want), len(got)); diff != "" {
				t.Errorf("\n%s\nGetXRDs(): -want count, +got count:\n%s", tt.reason, diff)
			}

			// Check if all expected XRDs are present by name
			wantNames := make(map[string]bool)
			gotNames := make(map[string]bool)

			for _, xrd := range tt.want {
				wantNames[xrd.GetName()] = true
			}

			for _, xrd := range got {
				gotNames[xrd.GetName()] = true
			}

			// Check missing XRDs
			for name := range wantNames {
				if !gotNames[name] {
					t.Errorf("\n%s\nGetXRDs(): missing expected XRD with name %s", tt.reason, name)
				}
			}

			// Check unexpected XRDs
			for name := range gotNames {
				if !wantNames[name] {
					t.Errorf("\n%s\nGetXRDs(): unexpected XRD with name %s", tt.reason, name)
				}
			}
		})
	}
}

func TestDefaultDefinitionClient_GetXRDForClaim(t *testing.T) {
	ctx := context.Background()

	// Create test XRDs
	xrdWithClaimKind := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd-with-claim").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XExampleResource",
			"plural":   "xexampleresources",
			"singular": "xexampleresource",
		}).
		WithSpecField("claimNames", map[string]interface{}{
			"kind":     "ExampleClaim",
			"plural":   "exampleclaims",
			"singular": "exampleclaim",
		}).
		Build()

	xrdWithoutClaimKind := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd-without-claim").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XOtherResource",
			"plural":   "xotherresources",
			"singular": "xotherresource",
		}).
		// No claimNames field
		Build()

	type args struct {
		gvk schema.GroupVersionKind
	}

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		cachedXRDs   []*un.Unstructured
		args         args
		want         *un.Unstructured
		wantErr      bool
		errSubstring string
	}{
		"MatchingClaimFound": {
			reason: "Should return the XRD that defines the claim kind",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdWithClaimKind, xrdWithoutClaimKind},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleClaim",
				},
			},
			want:    xrdWithClaimKind,
			wantErr: false,
		},
		"NoMatchingClaim": {
			reason: "Should return error when no XRD defines the claim kind",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdWithoutClaimKind},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleClaim",
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines claim type",
		},
		"GetXRDsError": {
			reason: "Should propagate error from GetXRDs",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			cachedXRDs: nil, // Force GetXRDs to be called
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "ExampleClaim",
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot get XRDs",
		},
		"DifferentGroup": {
			reason: "Should not match XRD with different group",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdWithClaimKind},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "different.org", // Different group
					Version: "v1",
					Kind:    "ExampleClaim", // Same kind
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines claim type",
		},
		"DifferentKind": {
			reason: "Should not match XRD with different claim kind",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdWithClaimKind},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org", // Same group
					Version: "v1",
					Kind:    "DifferentClaim", // Different kind
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines claim type",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultDefinitionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				xrds:           tt.cachedXRDs,
				xrdsLoaded:     tt.cachedXRDs != nil, // Only mark as loaded if we have cached XRDs
			}

			got, err := c.GetXRDForClaim(ctx, tt.args.gvk)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nGetXRDForClaim(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nGetXRDForClaim(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetXRDForClaim(): unexpected error: %v", tt.reason, err)
				return
			}

			if diff := cmp.Diff(tt.want.GetName(), got.GetName()); diff != "" {
				t.Errorf("\n%s\nGetXRDForClaim(): -want name, +got name:\n%s", tt.reason, diff)
			}

			// Verify it's the right XRD by checking the claim kind
			claimNames, found, _ := un.NestedMap(got.Object, "spec", "claimNames")
			if !found {
				t.Errorf("\n%s\nGetXRDForClaim(): returned XRD missing spec.claimNames", tt.reason)
				return
			}

			claimKind, found, _ := un.NestedString(claimNames, "kind")
			if !found || claimKind != tt.args.gvk.Kind {
				t.Errorf("\n%s\nGetXRDForClaim(): returned XRD has wrong claim kind, want %s, got %s",
					tt.reason, tt.args.gvk.Kind, claimKind)
			}
		})
	}
}

func TestDefaultDefinitionClient_GetXRDForXR(t *testing.T) {
	ctx := context.Background()

	// Create test XRDs
	xrdForXR1 := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd-for-xr1").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XR1",
			"plural":   "xr1s",
			"singular": "xr1",
		}).
		WithSpecField("versions", []interface{}{
			map[string]interface{}{
				"name":    "v1",
				"served":  true,
				"storage": true,
			},
			map[string]interface{}{
				"name":    "v2",
				"served":  true,
				"storage": false,
			},
		}).
		Build()

	xrdForXR2 := tu.NewResource("apiextensions.crossplane.io/v1", "CompositeResourceDefinition", "xrd-for-xr2").
		WithSpecField("group", "example.org").
		WithSpecField("names", map[string]interface{}{
			"kind":     "XR2",
			"plural":   "xr2s",
			"singular": "xr2",
		}).
		WithSpecField("versions", []interface{}{
			map[string]interface{}{
				"name":    "v1alpha1",
				"served":  true,
				"storage": true,
			},
		}).
		Build()

	type args struct {
		gvk schema.GroupVersionKind
	}

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		cachedXRDs   []*un.Unstructured
		args         args
		want         *un.Unstructured
		wantErr      bool
		errSubstring string
	}{
		"MatchingXRFound": {
			reason: "Should return the XRD that defines the XR kind",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdForXR1, xrdForXR2},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "XR1",
				},
			},
			want:    xrdForXR1,
			wantErr: false,
		},
		"MatchingXRWithDifferentVersion": {
			reason: "Should return the XRD that defines the XR kind with matching version",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdForXR1, xrdForXR2},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v2", // Using v2 version
					Kind:    "XR1",
				},
			},
			want:    xrdForXR1,
			wantErr: false,
		},
		"NoMatchingXR": {
			reason: "Should return error when no XRD defines the XR kind",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdForXR1, xrdForXR2},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "NonExistentXR", // No XRD defines this
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines XR type",
		},
		"GetXRDsError": {
			reason: "Should propagate error from GetXRDs",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			cachedXRDs: nil, // Force GetXRDs to be called
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v1",
					Kind:    "XR1",
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot get XRDs",
		},
		"DifferentGroup": {
			reason: "Should not match XRD with different group",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdForXR1},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "different.org", // Different group
					Version: "v1",
					Kind:    "XR1", // Same kind
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines XR type",
		},
		"VersionNotFound": {
			reason: "Should not match XRD if version doesn't exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				Build(),
			cachedXRDs: []*un.Unstructured{xrdForXR1},
			args: args{
				gvk: schema.GroupVersionKind{
					Group:   "example.org",
					Version: "v3", // Version doesn't exist in XRD
					Kind:    "XR1",
				},
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "no XRD found that defines XR type",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultDefinitionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				xrds:           tt.cachedXRDs,
				xrdsLoaded:     tt.cachedXRDs != nil, // Only mark as loaded if we have cached XRDs
			}

			got, err := c.GetXRDForXR(ctx, tt.args.gvk)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nGetXRDForXR(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nGetXRDForXR(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetXRDForXR(): unexpected error: %v", tt.reason, err)
				return
			}

			if diff := cmp.Diff(tt.want.GetName(), got.GetName()); diff != "" {
				t.Errorf("\n%s\nGetXRDForXR(): -want name, +got name:\n%s", tt.reason, diff)
			}

			// Verify it's the right XRD by checking the XR kind
			xrKind, found, _ := un.NestedString(got.Object, "spec", "names", "kind")
			if !found || xrKind != tt.args.gvk.Kind {
				t.Errorf("\n%s\nGetXRDForXR(): returned XRD has wrong XR kind, want %s, got %s",
					tt.reason, tt.args.gvk.Kind, xrKind)
			}
		})
	}
}

func TestDefaultDefinitionClient_Initialize(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		wantErr      bool
	}{
		"SuccessfulInitialization": {
			reason: "Should successfully initialize the client",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			wantErr: false,
		},
		"GetXRDsError": {
			reason: "Should return error when getting XRDs fails",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultDefinitionClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
			}

			err := c.Initialize(ctx)

			if tt.wantErr && err == nil {
				t.Errorf("\n%s\nInitialize(): expected error but got none", tt.reason)
			} else if !tt.wantErr && err != nil {
				t.Errorf("\n%s\nInitialize(): unexpected error: %v", tt.reason, err)
			}

			// If we succeeded, verify that XRDs are now loaded
			if !tt.wantErr {
				if !c.xrdsLoaded {
					t.Errorf("\n%s\nInitialize(): XRDs not marked as loaded after successful initialization", tt.reason)
				}
			}
		})
	}
}
