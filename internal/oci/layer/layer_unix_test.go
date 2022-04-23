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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestExtractFIFO(t *testing.T) {
	tmp, _ := os.MkdirTemp(os.TempDir(), t.Name())
	defer os.RemoveAll(tmp)

	inNonExistentDir := filepath.Join(tmp, "non-exist", "src")
	newFIFO := filepath.Join(tmp, "fifo")

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
		"FIFOError": {
			reason: "We should return an error if we can't create a FIFO",
			h:      HeaderHandlerFn(ExtractFIFO),
			args: args{
				h:    &tar.Header{Mode: 0700},
				path: inNonExistentDir,
			},
			want: errors.Wrap(errors.New("no such file or directory"), errCreateFIFO),
		},
		"Successful": {
			reason: "We should not return an error if we can create a symlink",
			h:      HeaderHandlerFn(ExtractFIFO),
			args: args{
				h:    &tar.Header{Mode: 0700},
				path: newFIFO,
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
