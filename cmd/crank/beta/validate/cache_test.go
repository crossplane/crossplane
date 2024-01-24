package validate

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	fs   = afero.NewMemMapFs()
	osFs = afero.NewOsFs()
)

func TestLocalCacheExists(t *testing.T) {
	type args struct {
		image string
	}
	type want struct {
		path string
		err  error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Exists": {
			reason: "Exists should return an empty path with no error if it exists",
			args: args{
				image: "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.0",
			},
			want: want{
				path: "",
				err:  nil,
			},
		},
		"DoesNotExist": {
			reason: "Exists should return the path with no error",
			args: args{
				image: "xpkg.upbound.io/crossplane-contrib/provider-nop:v0.2.1",
			},
			want: want{
				path: "testdata/cache/xpkg.upbound.io/crossplane-contrib/provider-nop@v0.2.1",
				err:  nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs:       fs,
				cacheDir: "testdata/cache",
			}
			got, err := c.Exists(tc.args.image)
			if diff := cmp.Diff(tc.want.path, got); diff != "" {
				t.Errorf("%s\nExists(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nExists(...): -want error, +got error:\n%s", tc.reason, diff)
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
				fs:       tc.args.fs,
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
		schemas [][]byte
		path    string
		fs      afero.Fs
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Store": {
			reason: "Store should store the schemas in the cache",
			args: args{
				schemas: [][]byte{
					[]byte("apiVersion: apiextensions.k8s.io/v1beta1\nkind: CustomResourceDefinition\nmetadata:\n  name: test\n"),
				},
				path: "testdata/cache/xpkg.upbound.io/crossplane-contrib/dummy@v0.2.0",
				fs:   fs,
			},
			want: want{
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &LocalCache{
				fs: tc.args.fs,
			}

			err := c.Store(tc.args.schemas, tc.args.path)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nStore(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if tc.want.err == nil {
				fPath := filepath.Join(tc.args.path, packageFileName)
				info, err := fs.Stat(fPath)
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
