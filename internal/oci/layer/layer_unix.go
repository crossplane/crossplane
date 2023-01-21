//go:build unix

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

	"golang.org/x/sys/unix"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// Error strings.
const (
	errCreateFIFO = "cannot create FIFO"
)

// ExtractFIFO is a HeaderHandler that creates a FIFO at the supplied path per
// the supplied tar header.
func ExtractFIFO(h *tar.Header, _ io.Reader, path string) error {
	// We won't have CAP_MKNOD in a user namespace created by a user who doesn't
	// have CAP_MKNOD in the initial/root user namespace, but we don't need it
	// to use mknod to create a FIFO.
	// https://man7.org/linux/man-pages/man2/mknod.2.html
	mode := uint32(h.Mode&0777) | unix.S_IFIFO
	dev := unix.Mkdev(uint32(h.Devmajor), uint32(h.Devminor))
	return errors.Wrap(unix.Mknod(path, mode, int(dev)), errCreateFIFO)
}
