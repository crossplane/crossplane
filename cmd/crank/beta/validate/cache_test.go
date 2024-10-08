package validate

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

var (
	fs   = afero.NewMemMapFs()
	osFs = afero.NewOsFs()
)

func TestLocalCacheGet(t *testing.T) {
	type args struct {
		image    string
		cacheDir string
		fs       afero.Fs
	}
	type want struct {
		schemas []*unstructured.Unstructured
		meta    *unstructured.Unstructured
		err     error
	}
	validSchema := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: testresources.test.crossplane.io
  annotations:
    controller-gen.kubebuilder.io/version: v0.11.1
spec:
  group: test.crossplane.io
  names:
    kind: TestResource
    plural: testresources
    singular: testresource
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                desiredState:
                  type: string
                  description: "Desired state of the TestResource."
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
---
`
	validMeta := `apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-test
  annotations:
    company: "Crossplane"
    license: "Apache-2.0"
    maintainer: "Crossplane Maintainers <info@crossplane.io>"
    source: "github.com/crossplane-contrib/provider-test"
spec:
  controller:
    image: "crossplane/provider-test:latest"
---
`
	uValidSchema := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(validSchema), uValidSchema)
	if err != nil {
		t.Fatalf("Error unmarshalling YAML: %v", err)
	}
	uValidaMeta := &unstructured.Unstructured{}
	err = yaml.Unmarshal([]byte(validMeta), uValidaMeta)
	if err != nil {
		t.Fatalf("Error unmarshalling YAML: %v", err)
	}

	// Define common variables to be reused
	validImage := "xpkg.upbound.io/crossplane-contrib/provider-test:v0.2.0"
	imgMetaless := "xpkg.upbound.io/crossplane-contrib/provider-test:v0.2.0-metaless"

	// Test cases
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessWithMetaAndSchemas": {
			reason: "Get should return both the meta and schemas from the cache",
			args: args{
				image:    validImage,
				cacheDir: "testdata/cache",
				fs:       fs,
			},
			want: want{
				schemas: []*unstructured.Unstructured{uValidSchema},
				meta:    uValidaMeta,
				err:     nil,
			},
		},
		"ErrorMissingMeta": {
			reason: "Get should return an error if the meta package is missing",
			args: args{
				image:    imgMetaless,
				cacheDir: "testdata/cache",
				fs:       fs,
			},
			want: want{
				schemas: []*unstructured.Unstructured{uValidSchema},
				meta:    nil,
				err:     errors.New("cannot find meta package"),
			},
		},
		"ErrorCacheDoesNotExist": {
			reason: "Get should return an error if the cache directory does not exist",
			args: args{
				image:    validImage,
				cacheDir: "testdata/non-existent-cache",
				fs:       fs,
			},
			want: want{
				schemas: nil,
				meta:    nil,
				err:     errors.Errorf(notFoundErrorFmt, "testdata/non-existent-cache/xpkg.upbound.io/crossplane-contrib/provider-test@v0.2.0"),
			},
		},
	}

	// Loop through test cases
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs:       fs,
				cacheDir: tc.args.cacheDir,
			}

			gotSchemas, gotMeta, err := c.Get(tc.args.image)

			if diff := cmp.Diff(tc.want.schemas, gotSchemas); diff != "" {
				t.Errorf("%s\nGet(...): -want schemas, +got schemas:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.meta, gotMeta); diff != "" {
				t.Errorf("%s\nGet(...): -want meta, +got meta:\n%s", tc.reason, diff)
			}
			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("%s\nGet(...): -want meta, +got meta:\n%s", tc.reason, diff)
				}
			} else if errors.Is(err, tc.want.err) == false { // handle cases where one is nil and the other is not
				t.Errorf("%s\nGet(...): -want error, +got error:\n-want: %v\n+got: %v", tc.reason, tc.want.err, err)
			}
		})
	}
}

func TestLocalCacheFlush(t *testing.T) {
	cases := map[string]struct {
		reason  string
		wantErr error
	}{
		"Flush": {
			reason:  "Flush should flush the cache",
			wantErr: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs:       fs,
				cacheDir: "testdata/cache",
			}

			err := c.Flush()
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nFlush(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLocalCacheInit(t *testing.T) {
	type args struct {
		cacheDir string
		fs       afero.Fs
	}
	cases := map[string]struct {
		reason  string
		args    args
		wantErr error
	}{
		"Success": {
			reason: "Init should initialize the cache",
			args: args{
				cacheDir: "testdata/cache",
				fs:       fs,
			},
			wantErr: nil,
		},
		"Error": {
			reason: "Init should return an error if it cannot create the cache directory",
			args: args{
				cacheDir: "/",
				fs:       osFs,
			},
			wantErr: nil,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs:       fs,
				cacheDir: tc.args.cacheDir,
			}

			err := c.Init()
			if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("Init() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.wantErr == nil {
				info, err := fs.Stat(c.cacheDir)
				if diff := cmp.Diff(tc.wantErr, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("Init() could not stat cache directory: %v", err)
				}

				if !info.IsDir() {
					t.Errorf("Init() cache directory is not a directory")
				}
			}
		})
	}
}

func TestLocalCacheLoad(t *testing.T) {
	type args struct {
		cacheDir string
	}
	type want struct {
		schemas []*unstructured.Unstructured
		err     error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Load": {
			reason: "Load should load the schemas from the cache",
			args:   args{cacheDir: "./testdata/crds"},
			want: want{
				schemas: []*unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"apiVersion": "apiextensions.k8s.io/v1beta1",
							"kind":       "CustomResourceDefinition",
							"metadata": map[string]interface{}{
								"name": "test",
							},
						},
					},
				},
				err: nil,
			},
		},
		"LoadNonExisting": {
			reason: "Load should return an error if the package does not exist",
			args:   args{cacheDir: "./testdata/non-existing"},
			want: want{
				schemas: nil,
				err:     cmpopts.AnyError,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs:       fs,
				cacheDir: tc.args.cacheDir,
			}

			got, err := c.Load()
			if diff := cmp.Diff(tc.want.schemas, got); diff != "" {
				t.Errorf("%s\nLoad(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nLoad(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLocalCacheStore(t *testing.T) {
	type args struct {
		image    string
		schemas  [][]byte
		cacheDir string
		fs       afero.Fs
	}
	type want struct {
		err error
	}

	// Test cases
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Store": {
			reason: "Store should store the schemas in the cache directory for the provided image",
			args: args{
				image: "xpkg.upbound.io/crossplane-contrib/dummy:v0.2.0",
				schemas: [][]byte{
					[]byte("apiVersion: apiextensions.k8s.io/v1beta1\nkind: CustomResourceDefinition\nmetadata:\n  name: test\n"),
				},
				cacheDir: "testdata/cache",
				fs:       fs,
			},
			want: want{
				err: nil,
			},
		},
	}

	// For each test case, run the test logic
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create the LocalCache instance
			c := &LocalCache{
				fs:       fs,
				cacheDir: tc.args.cacheDir,
			}

			// Call Store method
			err := c.Store(tc.args.image, tc.args.schemas)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			// Verify if file is correctly stored
			if tc.want.err == nil {
				cachePath := c.getCachePathOf(tc.args.image)
				fPath := filepath.Join(cachePath, packageFileName)
				info, err := tc.args.fs.Stat(fPath)
				if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
					t.Errorf("%s\nStore(...): -want error, +got error:\n%s", tc.reason, diff)
				}

				if info.IsDir() {
					t.Errorf("%s\nStore(...): -want file, +got directory", tc.reason)
				}
			}
		})
	}
}
