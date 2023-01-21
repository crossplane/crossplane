/*
Copyright 2022 The Crossplane Authors.

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

package layer

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

type MockHandler struct{ err error }

func (h *MockHandler) Handle(_ *tar.Header, _ io.Reader, _ string) error {
	return h.err
}

func TestStackingExtractor(t *testing.T) {
	errBoom := errors.New("boom")
	coolFile := "/cool/file"
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	type args struct {
		ctx  context.Context
		tb   io.Reader
		root string
	}
	cases := map[string]struct {
		reason string
		e      *StackingExtractor
		args   args
		want   error
	}{
		"ContextDone": {
			reason: "If the supplied context is done we should return its error.",
			e:      NewStackingExtractor(&MockHandler{}),
			args: args{
				ctx: cancelled,
			},
			want: cancelled.Err(),
		},
		"NotATarball": {
			reason: "If the supplied io.Reader is not a tarball we should return an error.",
			e:      NewStackingExtractor(&MockHandler{}),
			args: args{
				ctx: context.Background(),
				tb: func() io.Reader {
					b := &bytes.Buffer{}
					_, _ = b.WriteString("hi!")
					return b
				}(),
			},
			want: errors.Wrap(errors.New("unexpected EOF"), errAdvanceTarball),
		},
		"ErrorHandlingHeader": {
			reason: "If our HeaderHandler returns an error we should surface it.",
			e:      NewStackingExtractor(&MockHandler{err: errBoom}),
			args: args{
				ctx: context.Background(),
				tb: func() io.Reader {
					b := &bytes.Buffer{}
					tb := tar.NewWriter(b)
					tb.WriteHeader(&tar.Header{
						Typeflag: tar.TypeReg,
						Name:     coolFile,
					})
					_, _ = io.WriteString(tb, "hi!")
					tb.Close()
					return b
				}(),
			},
			want: errors.Wrapf(errBoom, errFmtHandleTarHeader, coolFile),
		},
		"Success": {
			reason: "If we successfully extract our tarball we should return a nil error.",
			e:      NewStackingExtractor(&MockHandler{}),
			args: args{
				ctx: context.Background(),
				tb: func() io.Reader {
					b := &bytes.Buffer{}
					tb := tar.NewWriter(b)
					tb.WriteHeader(&tar.Header{
						Typeflag: tar.TypeReg,
						Name:     coolFile,
					})
					_, _ = io.WriteString(tb, "hi!")
					tb.Close()
					return b
				}(),
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.e.Apply(tc.args.ctx, tc.args.tb, tc.args.root)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Apply(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestWhiteoutHandler(t *testing.T) {
	errBoom := errors.New("boom")

	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	coolDir := filepath.Join(tmp, "cool")
	coolFile := filepath.Join(coolDir, "file")
	coolWhiteout := filepath.Join(coolDir, ociWhiteoutPrefix+"file")
	_ = os.MkdirAll(coolDir, 0700)

	opaqueDir := filepath.Join(tmp, "opaque")
	opaqueDirWhiteout := filepath.Join(opaqueDir, ociWhiteoutOpaqueDir)
	_ = os.MkdirAll(opaqueDir, 0700)
	f, _ := os.Create(filepath.Join(opaqueDir, "some-file"))
	f.Close()

	nonExistentDirWhiteout := filepath.Join(tmp, "non-exist", ociWhiteoutOpaqueDir)

	type args struct {
		h    *tar.Header
		tr   io.Reader
		path string
	}
	cases := map[string]struct {
		reason string
		h      HeaderHandler
		args   args
		want   error
	}{
		"NotAWhiteout": {
			reason: "Files that aren't whiteouts should be passed to the underlying handler.",
			h:      NewWhiteoutHandler(&MockHandler{err: errBoom}),
			args: args{
				path: coolFile,
			},
			want: errBoom,
		},
		"HeaderAlreadyHandled": {
			reason: "We shouldn't whiteout a file that was already handled.",
			h: func() HeaderHandler {
				w := NewWhiteoutHandler(&MockHandler{})
				_ = w.Handle(nil, nil, coolFile) // Handle the file we'll try to whiteout.
				return w
			}(),
			args: args{
				path: coolWhiteout,
			},
			want: nil,
		},
		"WhiteoutFile": {
			reason: "We should delete a whited-out file.",
			h:      NewWhiteoutHandler(&MockHandler{}),
			args: args{
				path: filepath.Join(tmp, coolWhiteout),
			},
			// os.RemoveAll won't return an error even if this doesn't exist.
			want: nil,
		},
		"OpaqueDirDoesNotExist": {
			reason: "We should return early if asked to whiteout a directory that doesn't exist.",
			h:      NewWhiteoutHandler(&MockHandler{}),
			args: args{
				path: nonExistentDirWhiteout,
			},
			want: nil,
		},
		"WhiteoutOpaqueDir": {
			reason: "We should whiteout all files in an opaque directory.",
			h:      NewWhiteoutHandler(&MockHandler{}),
			args: args{
				path: opaqueDirWhiteout,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.h.Handle(tc.args.h, tc.args.tr, tc.args.path)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Handle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractHandler(t *testing.T) {
	errBoom := errors.New("boom")

	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	coolDir := filepath.Join(tmp, "cool")
	coolFile := filepath.Join(coolDir, "file")

	type args struct {
		h    *tar.Header
		tr   io.Reader
		path string
	}
	cases := map[string]struct {
		reason string
		h      HeaderHandler
		args   args
		want   error
	}{
		"UnsupportedMode": {
			reason: "Handling an unsupported file type should return an error.",
			h:      &ExtractHandler{handler: map[byte]HeaderHandler{}},
			args: args{
				h: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     coolFile,
				},
			},
			want: errors.Errorf(errFmtUnsupportedType, coolFile, tar.TypeReg),
		},
		"HandlerError": {
			reason: "Errors from an underlying handler should be returned.",
			h: &ExtractHandler{handler: map[byte]HeaderHandler{
				tar.TypeReg: &MockHandler{err: errBoom},
			}},
			args: args{
				h: &tar.Header{
					Typeflag: tar.TypeReg,
					Name:     coolFile,
				},
			},
			want: errors.Wrap(errBoom, errExtractTarHeader),
		},
		"Success": {
			reason: "If the underlying handler works we should return a nil error.",
			h: &ExtractHandler{handler: map[byte]HeaderHandler{
				tar.TypeReg: &MockHandler{},
			}},
			args: args{
				h: &tar.Header{
					Typeflag: tar.TypeReg,

					// We don't currently check the return value of Lchown, but
					// this will increase the chances it works by ensuring we
					// try to chown to our own UID/GID.
					Uid: os.Getuid(),
					Gid: os.Getgid(),
				},
				path: coolFile,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.h.Handle(tc.args.h, tc.args.tr, tc.args.path)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Handle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractDir(t *testing.T) {
	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	newDir := filepath.Join(tmp, "new")
	existingDir := filepath.Join(tmp, "existing-dir")
	existingFile := filepath.Join(tmp, "existing-file")
	_ = os.MkdirAll(existingDir, 0700)
	f, _ := os.Create(existingFile)
	f.Close()

	type args struct {
		h    *tar.Header
		tr   io.Reader
		path string
	}
	cases := map[string]struct {
		reason string
		h      HeaderHandler
		args   args
		want   error
	}{
		"ExistingPathIsNotADir": {
			reason: "We should return an error if trying to extract a dir to a path that exists but is not a dir.",
			h:      HeaderHandlerFn(ExtractDir),
			args: args{
				h:    &tar.Header{Mode: 0700},
				path: existingFile,
			},
			want: errors.Errorf(errFmtNotDir, existingFile),
		},
		"SuccessfulCreate": {
			reason: "We should not return an error if we can create the dir.",
			h:      HeaderHandlerFn(ExtractDir),
			args: args{
				h:    &tar.Header{Mode: 0700},
				path: newDir,
			},
			want: nil,
		},
		"SuccessfulChmod": {
			reason: "We should not return an error if we can chmod the existing dir",
			h:      HeaderHandlerFn(ExtractDir),
			args: args{
				h:    &tar.Header{Mode: 0700},
				path: existingDir,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.h.Handle(tc.args.h, tc.args.tr, tc.args.path)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Handle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractSymlink(t *testing.T) {
	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	linkSrc := filepath.Join(tmp, "src")
	linkDst := filepath.Join(tmp, "dst")
	inNonExistentDir := filepath.Join(tmp, "non-exist", "src")

	type args struct {
		h    *tar.Header
		tr   io.Reader
		path string
	}
	cases := map[string]struct {
		reason string
		h      HeaderHandler
		args   args
		want   error
	}{
		"SymlinkError": {
			reason: "We should return an error if we can't create a symlink",
			h:      HeaderHandlerFn(ExtractSymlink),
			args: args{
				h:    &tar.Header{Linkname: linkDst},
				path: inNonExistentDir,
			},
			want: errors.Wrap(errors.Errorf("symlink %s %s: no such file or directory", linkDst, inNonExistentDir), errSymlink),
		},
		"Successful": {
			reason: "We should not return an error if we can create a symlink",
			h:      HeaderHandlerFn(ExtractSymlink),
			args: args{
				h:    &tar.Header{Linkname: linkDst},
				path: linkSrc,
			},
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.h.Handle(tc.args.h, tc.args.tr, tc.args.path)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Handle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestExtractFile(t *testing.T) {
	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	inNonExistentDir := filepath.Join(tmp, "non-exist", "file")
	newFile := filepath.Join(tmp, "coolFile")

	type args struct {
		h    *tar.Header
		tr   io.Reader
		path string
	}
	cases := map[string]struct {
		reason string
		h      HeaderHandler
		args   args
		want   error
	}{
		"OpenFileError": {
			reason: "We should return an error if we can't create a file",
			h:      HeaderHandlerFn(ExtractFile),
			args: args{
				h:    &tar.Header{},
				path: inNonExistentDir,
			},
			want: errors.Wrap(errors.Errorf("open %s: no such file or directory", inNonExistentDir), errOpenFile),
		},
		"SuccessfulWRite": {
			reason: "We should return a nil error if we successfully wrote the file.",
			h:      HeaderHandlerFn(ExtractFile),
			args: func() args {
				b := &bytes.Buffer{}
				tw := tar.NewWriter(b)

				content := []byte("hi!")
				h := &tar.Header{
					Typeflag: tar.TypeReg,
					Mode:     0600,
					Size:     int64(len(content)),
				}

				_ = tw.WriteHeader(h)
				_, _ = tw.Write(content)
				_ = tw.Close()

				tr := tar.NewReader(b)
				tr.Next()

				return args{
					h:    h,
					tr:   tr,
					path: newFile,
				}
			}(),
			want: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.h.Handle(tc.args.h, tc.args.tr, tc.args.path)
			if diff := cmp.Diff(tc.want, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nh.Handle(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}
