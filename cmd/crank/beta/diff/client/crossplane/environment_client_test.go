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

var _ EnvironmentClient = (*tu.MockEnvironmentClient)(nil)

func TestDefaultEnvironmentClient_GetEnvironmentConfigs(t *testing.T) {
	ctx := context.Background()

	// Create test environment configurations
	envConfig1 := tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "env-config-1").
		WithSpecField("data", map[string]interface{}{
			"key1": "value1",
		}).
		Build()

	envConfig2 := tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "env-config-2").
		WithSpecField("data", map[string]interface{}{
			"key2": "value2",
		}).
		Build()

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		want         []*un.Unstructured
		wantErr      bool
		errSubstring string
	}{
		"NoConfigs": {
			reason: "Should return empty slice when no environment configs exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == "apiextensions.crossplane.io" &&
						gvk.Version == "v1alpha1" &&
						gvk.Kind == "EnvironmentConfig" {
						return []*un.Unstructured{}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			want:    []*un.Unstructured{},
			wantErr: false,
		},
		"ConfigsExist": {
			reason: "Should return all environment configs when they exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == "apiextensions.crossplane.io" &&
						gvk.Version == "v1alpha1" &&
						gvk.Kind == "EnvironmentConfig" {
						return []*un.Unstructured{envConfig1, envConfig2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			want:    []*un.Unstructured{envConfig1, envConfig2},
			wantErr: false,
		},
		"ListError": {
			reason: "Should propagate errors from the Kubernetes API",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot list environment configs",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultEnvironmentClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				envConfigs:     make(map[string]*un.Unstructured),
			}

			got, err := c.GetEnvironmentConfigs(ctx)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nGetEnvironmentConfigs(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nGetEnvironmentConfigs(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetEnvironmentConfigs(): unexpected error: %v", tt.reason, err)
				return
			}

			// Compare the counts
			if diff := cmp.Diff(len(tt.want), len(got)); diff != "" {
				t.Errorf("\n%s\nGetEnvironmentConfigs(): -want count, +got count:\n%s", tt.reason, diff)
			}

			// Create maps of configs by name for comparison
			wantConfigs := make(map[string]*un.Unstructured)
			gotConfigs := make(map[string]*un.Unstructured)

			for _, cfg := range tt.want {
				wantConfigs[cfg.GetName()] = cfg
			}

			for _, cfg := range got {
				gotConfigs[cfg.GetName()] = cfg
			}

			// Check for missing configs
			for name, wantCfg := range wantConfigs {
				if _, ok := gotConfigs[name]; !ok {
					t.Errorf("\n%s\nGetEnvironmentConfigs(): missing config with name %s", tt.reason, name)
				} else {
					// Check data field for configs that exist in both lists
					wantData, _, _ := un.NestedMap(wantCfg.Object, "spec", "data")
					gotData, _, _ := un.NestedMap(gotConfigs[name].Object, "spec", "data")
					if diff := cmp.Diff(wantData, gotData); diff != "" {
						t.Errorf("\n%s\nGetEnvironmentConfigs(): config %s data mismatch -want, +got:\n%s", tt.reason, name, diff)
					}
				}
			}

			// Check for unexpected configs
			for name := range gotConfigs {
				if _, ok := wantConfigs[name]; !ok {
					t.Errorf("\n%s\nGetEnvironmentConfigs(): unexpected config with name %s", tt.reason, name)
				}
			}
		})
	}
}

func TestDefaultEnvironmentClient_GetEnvironmentConfig(t *testing.T) {
	ctx := context.Background()

	// Create test environment configuration
	envConfig := tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "test-env-config").
		WithSpecField("data", map[string]interface{}{
			"key": "value",
		}).
		Build()

	type args struct {
		name string
	}

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		cachedConfig map[string]*un.Unstructured
		args         args
		want         *un.Unstructured
		wantErr      bool
		errSubstring string
	}{
		"CachedConfig": {
			reason: "Should return environment config from cache when available",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				// GetResource should not be called when cache is used
				WithGetResource(func(_ context.Context, _ schema.GroupVersionKind, _, _ string) (*un.Unstructured, error) {
					t.Error("GetResource should not be called when config is in cache")
					return nil, nil
				}).
				Build(),
			cachedConfig: map[string]*un.Unstructured{
				"cached-config": envConfig,
			},
			args: args{
				name: "cached-config",
			},
			want:    envConfig,
			wantErr: false,
		},
		"FetchFromCluster": {
			reason: "Should fetch environment config from cluster when not in cache",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithGetResource(func(_ context.Context, gvk schema.GroupVersionKind, _, name string) (*un.Unstructured, error) {
					if gvk.Group == "apiextensions.crossplane.io" &&
						gvk.Version == "v1alpha1" &&
						gvk.Kind == "EnvironmentConfig" &&
						name == "test-env-config" {
						return envConfig, nil
					}
					return nil, errors.New("unexpected resource request")
				}).
				Build(),
			cachedConfig: map[string]*un.Unstructured{},
			args: args{
				name: "test-env-config",
			},
			want:    envConfig,
			wantErr: false,
		},
		"NotFound": {
			reason: "Should return error when environment config doesn't exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithResourceNotFound().
				Build(),
			cachedConfig: map[string]*un.Unstructured{},
			args: args{
				name: "nonexistent-config",
			},
			want:         nil,
			wantErr:      true,
			errSubstring: "cannot get environment config",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultEnvironmentClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				envConfigs:     tt.cachedConfig,
			}

			got, err := c.GetEnvironmentConfig(ctx, tt.args.name)

			if tt.wantErr {
				if err == nil {
					t.Errorf("\n%s\nGetEnvironmentConfig(): expected error but got none", tt.reason)
					return
				}
				if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("\n%s\nGetEnvironmentConfig(): expected error containing %q, got %q", tt.reason, tt.errSubstring, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("\n%s\nGetEnvironmentConfig(): unexpected error: %v", tt.reason, err)
				return
			}

			// Check that the returned config has the right name
			if got.GetName() != tt.want.GetName() {
				t.Errorf("\n%s\nGetEnvironmentConfig(): want name %s, got name %s", tt.reason, tt.want.GetName(), got.GetName())
			}

			// Check that the data is correct
			wantData, _, _ := un.NestedMap(tt.want.Object, "spec", "data")
			gotData, _, _ := un.NestedMap(got.Object, "spec", "data")
			if diff := cmp.Diff(wantData, gotData); diff != "" {
				t.Errorf("\n%s\nGetEnvironmentConfig(): config data mismatch -want, +got:\n%s", tt.reason, diff)
			}

			// Verify the config was added to the cache
			if _, ok := c.envConfigs[tt.args.name]; !ok {
				t.Errorf("\n%s\nGetEnvironmentConfig(): config not added to cache after fetch", tt.reason)
			}
		})
	}
}

func TestDefaultEnvironmentClient_Initialize(t *testing.T) {
	ctx := context.Background()

	// Create test environment configurations
	envConfig1 := tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "env-config-1").Build()
	envConfig2 := tu.NewResource("apiextensions.crossplane.io/v1alpha1", "EnvironmentConfig", "env-config-2").Build()

	tests := map[string]struct {
		reason       string
		mockResource tu.MockResourceClient
		wantErr      bool
		wantCached   map[string]bool
	}{
		"SuccessfulInitialization": {
			reason: "Should successfully initialize and cache environment configs",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResources(func(_ context.Context, gvk schema.GroupVersionKind, _ string) ([]*un.Unstructured, error) {
					if gvk.Group == "apiextensions.crossplane.io" &&
						gvk.Version == "v1alpha1" &&
						gvk.Kind == "EnvironmentConfig" {
						return []*un.Unstructured{envConfig1, envConfig2}, nil
					}
					return nil, errors.New("unexpected GVK")
				}).
				Build(),
			wantErr: false,
			wantCached: map[string]bool{
				"env-config-1": true,
				"env-config-2": true,
			},
		},
		"NoConfigs": {
			reason: "Should successfully initialize with empty cache when no configs exist",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithEmptyListResources().
				Build(),
			wantErr:    false,
			wantCached: map[string]bool{},
		},
		"ListError": {
			reason: "Should return error when listing environment configs fails",
			mockResource: *tu.NewMockResourceClient().
				WithSuccessfulInitialize().
				WithListResourcesFailure("list error").
				Build(),
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &DefaultEnvironmentClient{
				resourceClient: &tt.mockResource,
				logger:         tu.TestLogger(t, false),
				envConfigs:     make(map[string]*un.Unstructured),
			}

			err := c.Initialize(ctx)

			if tt.wantErr && err == nil {
				t.Errorf("\n%s\nInitialize(): expected error but got none", tt.reason)
				return
			} else if !tt.wantErr && err != nil {
				t.Errorf("\n%s\nInitialize(): unexpected error: %v", tt.reason, err)
				return
			}

			// If no error expected, check the cache state
			if !tt.wantErr {
				for name := range tt.wantCached {
					if _, ok := c.envConfigs[name]; !ok {
						t.Errorf("\n%s\nInitialize(): expected config %s to be cached, but it's not", tt.reason, name)
					}
				}

				// Check we don't have extra configs
				if len(c.envConfigs) != len(tt.wantCached) {
					t.Errorf("\n%s\nInitialize(): expected %d cached configs, got %d", tt.reason, len(tt.wantCached), len(c.envConfigs))
				}
			}
		})
	}
}
