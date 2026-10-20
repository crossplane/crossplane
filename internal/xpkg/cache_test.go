/*
Copyright 2020 The Crossplane Authors.

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

package xpkg

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var _ PackageCache = &FsPackageCache{}

func TestHas(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.gz")
	_ = fs.Mkdir("/cache/some-dir.gz", os.ModeDir)

	defer cf.Close()

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
			want: true,
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
			want: false,
		},
		"ErrIsDir": {
			reason: "Should return error if path is a directory.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "some-dir.gz",
			},
			want: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := tc.args.cache.Has(tc.args.id)

			if diff := cmp.Diff(tc.want, h); diff != "" {
				t.Errorf("\n%s\nHas(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestGet(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.gz")
	// NOTE(hasheddan): valid gzip header.
	cf.Write([]byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 0})
	cf, _ = fs.Create("/cache/not-gzip.gz")

	cf.WriteString("some content")
	defer cf.Close()

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package exists at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
		},
		"ErrNotGzip": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-gzip",
			},
			want: gzip.ErrHeader,
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
			want: &os.PathError{Op: "open", Path: "/cache/not-exist.gz", Err: afero.ErrFileNotFound},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.args.cache.Get(tc.args.id)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStore(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		cache   func(fs afero.Fs) PackageCache
		id      string
		content io.Reader
	}
	type want struct {
		err   error
		files []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Success": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				cache:   func(fs afero.Fs) PackageCache { return NewFsPackageCache("/cache", fs) },
				id:      "exists-1234567",
				content: new(bytes.Buffer),
			},
			want: want{files: []string{"exists-1234567.gz"}},
		},
		"ErrFailedCreate": {
			reason: "Should return an error if file creation fails.",
			args: args{
				cache:   func(fs afero.Fs) PackageCache { return NewFsPackageCache("/cache", afero.NewReadOnlyFs(fs)) },
				id:      "exists-1234567",
				content: new(bytes.Buffer),
			},
			want: want{err: syscall.EPERM},
		},
		"ErrFailedReadLeavesNoFile": {
			reason: "Should return an error and leave nothing in the cache if reading the content fails partway.",
			args: args{
				cache:   func(fs afero.Fs) PackageCache { return NewFsPackageCache("/cache", fs) },
				id:      "partial-1234567",
				content: io.MultiReader(strings.NewReader("partial content"), iotest.ErrReader(errBoom)),
			},
			want: want{err: errBoom},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			_ = fs.Mkdir("/cache", 0o755)

			err := tc.args.cache(fs).Store(tc.args.id, io.NopCloser(tc.args.content))
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			infos, _ := afero.ReadDir(fs, "/cache")
			files := make([]string, 0, len(infos))
			for _, fi := range infos {
				files = append(files, fi.Name())
			}
			if diff := cmp.Diff(tc.want.files, files, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want files in cache, +got files in cache:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	fs := afero.NewMemMapFs()
	_, _ = fs.Create("/cache/exists.xpkg")

	type args struct {
		cache PackageCache
		id    string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package is deleted at path.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "exists",
			},
		},
		"SuccessNotExist": {
			reason: "Should not return an error if package does not exist.",
			args: args{
				cache: NewFsPackageCache("/cache", fs),
				id:    "not-exist",
			},
		},
		"ErrFailedDelete": {
			reason: "Should return an error if file deletion fails.",
			args: args{
				cache: NewFsPackageCache("/cache", afero.NewReadOnlyFs(fs)),
				id:    "exists-1234567",
			},
			want: syscall.EPERM,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Delete(tc.args.id)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
