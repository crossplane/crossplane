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
	"os"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/spf13/afero"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestGet(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.xpkg")
	_ = tarball.Write(name.Tag{}, empty.Image, cf)

	type args struct {
		cache Cache
		tag   string
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
				cache: NewImageCache("/cache", fs),
				tag:   "",
				id:    "exists",
			},
		},
		"ErrNotExist": {
			reason: "Should return error if package does not exist at path.",
			args: args{
				cache: NewImageCache("/cache", fs),
				tag:   "",
				id:    "not-exist",
			},
			want: &os.PathError{Op: "open", Path: "/cache/not-exist.xpkg", Err: afero.ErrFileNotFound},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.args.cache.Get(tc.args.tag, tc.args.id)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGet(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestStore(t *testing.T) {
	fs := afero.NewMemMapFs()

	type args struct {
		cache Cache
		tag   string
		id    string
		img   v1.Image
	}
	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Success": {
			reason: "Should not return an error if package is created at path.",
			args: args{
				cache: NewImageCache("/cache", fs),
				tag:   "crossplane/exist-xpkg:latest",
				id:    "exists-1234567",
				img:   empty.Image,
			},
		},
		"ErrFailedCreate": {
			reason: "Should return an error if file creation fails.",
			args: args{
				cache: NewImageCache("/cache", afero.NewReadOnlyFs(fs)),
				tag:   "crossplane/exist-xpkg:latest",
				id:    "exists-1234567",
				img:   empty.Image,
			},
			want: syscall.EPERM,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.cache.Store(tc.args.tag, tc.args.id, tc.args.img)

			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nStore(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	fs := afero.NewMemMapFs()
	cf, _ := fs.Create("/cache/exists.xpkg")
	_ = tarball.Write(name.Tag{}, empty.Image, cf)

	type args struct {
		cache Cache
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
				cache: NewImageCache("/cache", fs),
				id:    "exists",
			},
		},
		"SuccessNotExist": {
			reason: "Should not return an error if package does not exist.",
			args: args{
				cache: NewImageCache("/cache", fs),
				id:    "not-exist",
			},
		},
		"ErrFailedDelete": {
			reason: "Should return an error if file deletion fails.",
			args: args{
				cache: NewImageCache("/cache", afero.NewReadOnlyFs(fs)),
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
