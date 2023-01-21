//go:build linux

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

package overlay

import (
	"fmt"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

// NOTE(negz): Technically _all_ of the overlay implementation is only useful on
// Linux, but we want to support building what we can on other operating systems
// (e.g. Darwin) to make it possible for folks running them to ensure that code
// compiles and passes tests during development. Avoid adding code to this file
// unless it actually needs Linux to run.

// Mount the tmpfs mount.
func (m TmpFSMount) Mount() error {
	var flags uintptr
	return errors.Wrapf(unix.Mount("tmpfs", m.Mountpoint, "tmpfs", flags, ""), "cannot mount tmpfs at %q", m.Mountpoint)
}

// Unmount the tmpfs mount.
func (m TmpFSMount) Unmount() error {
	var flags int
	return errors.Wrapf(unix.Unmount(m.Mountpoint, flags), "cannot unmount tmpfs at %q", m.Mountpoint)
}

// Mount the overlay mount.
func (m OverlayMount) Mount() error {
	var flags uintptr
	data := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", strings.Join(m.Lower, ":"), m.Upper, m.Work)
	return errors.Wrapf(unix.Mount("overlay", m.Mountpoint, "overlay", flags, data), "cannot mount overlayfs at %q", m.Mountpoint)
}

// Unmount the overlay mount.
func (m OverlayMount) Unmount() error {
	var flags int
	return errors.Wrapf(unix.Unmount(m.Mountpoint, flags), "cannot unmount overlayfs at %q", m.Mountpoint)
}
